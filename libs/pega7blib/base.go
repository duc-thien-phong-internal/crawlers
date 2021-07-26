package pega7lib

import (
	"fmt"
	"github.com/antchfx/htmlquery"
	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	"github.com/duc-thien-phong/techsharedservices/models/softwareclient"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"github.com/tebeka/selenium"
	"golang.org/x/net/html"
	"net/http/cookiejar"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Base struct {
	basecrawler.Worker

	harnessID     string
	transactionID string
	loginURLID    string
}

func (p Base) StillEnoughPermission(errMessage string) bool {
	return true
}

func (p *Base) goToLoginPage(loginPage string) error {
	logger.Root.Infof("%#v", p.Browser.DriverInfo)
	if p.Browser == nil || p.Browser.DriverInfo.Driver == nil {
		p.NeedToClose = true
		p.SetNeedToBeReplaced(true)
		return ErrInvalidSession
	}
	d := *(p.Browser.DriverInfo.Driver)
	err := utils.RetryOperation(func() error {
		logger.Root.Infof("Go to login page")
		d.DeleteAllCookies()
		err := d.Get(loginPage)
		if err != nil {
			return err
		}
		time.Sleep(1000 * time.Millisecond)

		err = d.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
			elm, err := d.FindElement(selenium.ByID, "txtUserID")
			return elm != nil, err
		}, 15*time.Second, 1*time.Second)
		return err
	}, 3, 10*time.Second)

	return err
}

func (p *Base) enterSubmitLogin(pDriver *selenium.WebDriver, accountInfo *basecrawler.UsefulAccountInfo) string {
	logger.Root.Infof("Filing login information")

	d := *pDriver
	var errorMessage string
	errElement, err := d.FindElement(selenium.ByXPATH, "//div[@id='error']|//div[@class='message']")
	if err == nil {
		errorMessage, _ = errElement.Text()
		display, _ := errElement.IsDisplayed()
		if display {
			errorMessage = strings.ToLower(errorMessage)
			// if strings.Contains(errorMessage, "maximum number of active sessions for the current operator has been reached") {
			// 	return errorMessage
			// }
			logger.Root.Errorf("!!!!! ErrorMessage before login: %s\n", errorMessage)
			text, _ := d.PageSource()
			utils.WriteContentToPage([]byte(text), "login-error-", "no-information")
			d.DeleteAllCookies()
			if strings.Contains(errorMessage, "blocked") {
				return "Found error message before login"
			}
		}
	}

	txtUserID, err := d.FindElement(selenium.ByID, "txtUserID")

	if err != nil {
		logger.Root.Infof("Error when searching element user ID: %s\n", err)

		return "The system doesn't allow to login now"
	} else {
		displayed, _ := txtUserID.IsDisplayed()
		enabled, _ := txtUserID.IsEnabled()
		if !displayed || !enabled {
			return "The system doesn't allow to login now"
		}
		txtUserID.SendKeys(accountInfo.Account.Username)
	}

	txtPassword, err := d.FindElement(selenium.ByID, "txtPassword")
	if err != nil {
		logger.Root.Infof("Error when searching element txtPassword: %s\n", err)
	} else {
		txtPassword.SendKeys(accountInfo.Account.Password)
	}

	btnSubmit, err := d.FindElement(selenium.ByCSSSelector, "button#sub")
	if err != nil {
		logger.Root.Infof("Error when searching element btnSubmit: %s\n", err)
	} else {
		btnSubmit.Click()
	}

	time.Sleep(5 * time.Second)
	err = d.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		errElement, err := d.FindElement(selenium.ByXPATH, "//div[@id='error']|//div[@class='message']")
		if err != nil {
			errElement, err = d.FindElement(selenium.ByCSSSelector, "div.current-application")
			if err != nil || errElement == nil {
				return false, err
			}
		}

		displayed, _ := errElement.IsDisplayed()
		errorMessage, _ = errElement.Text()
		return displayed, nil
	}, 30*time.Second, 1*time.Second)

	logger.Root.Infof("ErrorMessage: %s\n", errorMessage)

	if errorMessage == "" {
		elm, err := d.FindElement(selenium.ByXPATH, "//tbody/tr/td[2]/table/tbody/tr[2]/td[2]")
		if elm != nil && err == nil {
			errorMessage, _ = elm.Text()
		}
	}

	return errorMessage
}

func (p *Base) LoginWithAccount(acc *basecrawler.UsefulAccountInfo) string {
	if err := p.goToLoginPage(LoginURL); err != nil {
		logger.Root.Infof("Error when going to login page: %v", err)
		return err.Error()
	}
	d := p.Browser.DriverInfo.Driver
	return p.enterSubmitLogin(d, acc)
}

func (p *Base) CheckAuthenticate() (bool, error) {
	accountInfo := p.CurrentAccount
	if accountInfo != nil {
		lastAccount, ok := p.LastAccountByUsername[accountInfo.Account.Username]
		if accountInfo.LastTimeLogin.Add(time.Duration(10)*time.Minute).Unix() > time.Now().Unix() && ok && lastAccount.Username == accountInfo.Account.Username {
			logger.Root.Info("Too early to check again")
			return true, nil
		}
	} else {
		logger.Root.Fatal("There is no account")
		return false, basecrawler.ErrNoUsableAccount
	}

	p.LastAccountByUsername[accountInfo.Account.Username] = *accountInfo.Account
	if accountInfo.CookieJar == nil {
		accountInfo.CookieJar, _ = cookiejar.New(nil)
	}

	// p.Browser.SendAjax()
	return true, nil
}

func (p *Base) Close() error {
	if p.Browser != nil {
		p.Browser.Close()
	}

	return nil
}

func (p *Base) getHarnessID(defaultValue string) {
	d := *(p.Browser.DriverInfo.Driver)

	harnessElm, err := d.FindElement(selenium.ByXPATH, "//*[@id='pzHarnessID']")
	if err != nil {
		logger.Root.Errorf("There is no harnessID")
	} else {
		id, _ := harnessElm.GetAttribute("value")
		if id != "" {
			p.harnessID = id
			logger.Root.Infof("The harnessID: %s", p.harnessID)
			return
		}
	}

	currentURL, _ := (*p.Browser.DriverInfo.Driver).CurrentURL()
	reg := regexp.MustCompile(".*pzHarnessID=(?P<id>[^&#]+).*")
	match := utils.MatchRegexInString(currentURL, reg)

	if id, ok := match["id"]; ok && id != "" {
		p.harnessID = id
		logger.Root.Infof("The harnessID: %s", p.harnessID)
		return
	}

	if p.harnessID == "" {
		p.harnessID = defaultValue
	}
}

func (p *Base) FetchInformationOfProfile(dataID string, fetchFull bool) (*customer.Application, error) {
	logger.Root.Infof("`%s` Fetching profile %s", p.GetID(), dataID)
	resultChan := make(chan string, 1)
	startingTime := time.Now().Unix()
	defer func() {
		(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = '"+p.GetID()+"'", []interface{}{})
		logger.Root.Infof("`%s`Finishes fetching profile %s in %d seconds", p.GetID(), dataID, time.Now().Unix()-startingTime)
	}()

	p.getHarnessID(p.harnessID)

	url := BaseURL + `!TABTHREAD1?pyActivity=Work-.Open&InsHandle=FECREDIT-BASE-LOAN-WORK%20@workID@&Action=Review&AllowInput=false&HarnessPurpose=Review&pzHarnessID=@harnessID@`
	url = strings.ReplaceAll(url, "@workID@", dataID)
	url = strings.ReplaceAll(url, "@id@", p.loginURLID)
	url = strings.ReplaceAll(url, "@harnessID@", p.harnessID)
	d := *(p.Browser.DriverInfo.Driver)

	if err := d.Get(url); err != nil {
		logger.Root.Errorf("Could not navigate to url: `%s`", url)
		return nil, err
	}
	time.Sleep(1000 * time.Millisecond)
	go func() {

		basecrawler.WaitUntilElementDisplay(d, "//*[@id='TABANCHOR' and @tabtitle='Information']", 15, 1, 2, 3)

		if elm, err := d.FindElement(selenium.ByXPATH, "//*[@id='TABANCHOR' and @tabtitle='Information']"); err == nil {
			elm.MoveTo(0, 0)
			elm.Click()
			time.Sleep(300 * time.Millisecond)
			basecrawler.WaitUntilElementDisplay(d, "//*[@id='TABANCHOR' and @tabtitle='Loan']", 10, 1, 1, 3)

			if elm, err := d.FindElement(selenium.ByXPATH, "//*[@id='TABANCHOR' and @tabtitle='Loan']"); err == nil {
				elm.MoveTo(0, 0)
				elm.Click()
				time.Sleep(300 * time.Millisecond)
				basecrawler.WaitUntilElementDisplay(d, "//*[@node_name='LoanDetails']", 10, 1, 1, 3)
			}

			if fetchFull {
				if elm, err := d.FindElement(selenium.ByXPATH, "//*[@id='TABANCHOR' and @tabtitle='Customer']"); err == nil {
					elm.MoveTo(0, 0)
					elm.Click()
					time.Sleep(300 * time.Millisecond)
					basecrawler.WaitUntilElementDisplay(d, "//*[@node_name='Identification']", 5, 1, 1, 3)
					time.Sleep(300 * time.Millisecond)
				}
				if elm, err := d.FindElement(selenium.ByXPATH, "//*[@id='TABANCHOR' and @tabtitle='Family']"); err == nil {
					elm.MoveTo(0, 0)
					elm.Click()
					time.Sleep(300 * time.Millisecond)
					basecrawler.WaitUntilElementDisplay(d, "//*[@node_name='Family']", 7, 1, 1, 3)
					time.Sleep(300 * time.Millisecond)
				}
				if elm, err := d.FindElement(selenium.ByXPATH, "//*[@id='TABANCHOR' and @tabtitle='Contacts']"); err == nil {
					elm.MoveTo(0, 0)
					elm.Click()
					time.Sleep(300 * time.Millisecond)
					basecrawler.WaitUntilElementDisplay(d, "//*[@node_name='ContactInformation']", 5, 1, 1, 3)
					time.Sleep(300 * time.Millisecond)
				}
				if elm, err := d.FindElement(selenium.ByXPATH, "//*[@id='TABANCHOR' and @tabtitle='Work']"); err == nil {
					elm.MoveTo(0, 0)
					elm.Click()
					time.Sleep(300 * time.Millisecond)
					basecrawler.WaitUntilElementDisplay(d, "//*[@node_name='EmployerAndWorkExperience']", 5, 1, 1, 3)
					time.Sleep(300 * time.Millisecond)
				}
				if elm, err := d.FindElement(selenium.ByXPATH, "//*[@id='TABANCHOR' and @tabtitle='Income']"); err == nil {
					elm.MoveTo(0, 0)
					elm.Click()
					time.Sleep(300 * time.Millisecond)
					basecrawler.WaitUntilElementDisplay(d, "//*[@node_name='Income']", 5, 1, 1, 3)
					time.Sleep(300 * time.Millisecond)
				}
				if elm, err := d.FindElement(selenium.ByXPATH, "//*[@id='TABANCHOR' and @tabtitle='Referenced Persons']"); err == nil {
					elm.MoveTo(0, 0)
					elm.Click()
					time.Sleep(300 * time.Millisecond)
					basecrawler.WaitUntilElementDisplay(d, "//*[@node_name='OtherReferensedPersons']", 7, 1, 1, 3)
					time.Sleep(300 * time.Millisecond)
				}

				time.Sleep(300 * time.Millisecond)
			}
		}

		content, err := d.PageSource()

		if err != nil {
			logger.Root.Errorf("Error when fetching information of profile %s: %v", dataID, err)
		} else {
			// logger.Root.Infof("Profile %s:\n`%s`", workID, result)
			// utils.WriteContentToPage([]byte(result), "profile-detail-"+workID+"--", "mockkkkkk")
		}
		resultChan <- content
		close(resultChan)
	}()

	var content string
	select {
	case content = <-resultChan:
		break
	case <-time.After(60 * time.Second):
		logger.Root.Infof("`%s` Timeout when fetching profile `%s`", p.GetID(), dataID)
		return nil, ErrTimeout
	}

	if content == "" {
		logger.Root.Errorf("Error when parsing customer application. Empty content here")
		return nil, ErrEmptyContent
	} else if strings.Contains(content, "The Operation completed successfully, but returned no content") {
		return nil, ErrInvalidSession
	} else {
		logger.Root.Infof("get content profile Ok")
	}

	if strings.Contains(content, "TSA Mobile") {
		logger.Root.Infof("there is information about TSA Phone before additional page")
	}
	if strings.Contains(content, "DSA Mobile") {
		logger.Root.Infof("there is information about TSA Phone before additional page")
	}

	content = "<html>" + content + "</html>"

	content = strings.ReplaceAll(content, "<!DOCTYPE html> ", "")
	doc, err := htmlquery.Parse(strings.NewReader(content))
	if err != nil {
		logger.Root.Errorf("Error when loading html content of application. %v", err)
		utils.WriteContentToPage([]byte(content), "profile-detail-"+dataID, "fail-pages")
		return nil, err
	}
	app, err := constructCustomerApplicationFromContent(doc, dataID)

	if app == nil || err != nil {
		if strings.Contains(content, "txtPassword") && strings.Contains(content, "txtUserID") {
			logger.Root.Infof("Need to close the worker `%s` because the detail profile is the login page", p.GetID())
			p.SetNeedToBeReplaced(true)
			p.SetNeedToClose(true)
		}
		logger.Root.Infof("Could not construct app from content. Err: %v", err)
		utils.WriteContentToPage([]byte(content), "profile-detail-"+dataID, "fail-pages")
		return nil, ErrEmptyContent
	}

	// p.sendGetTracker(workID)
	address := p.getAddress(doc, dataID)
	if address != nil {
		app.PersonalInformation.Address = *address
	} else {
		utils.WriteContentToPage([]byte(content), "address-", "no-information")
	}

	// p.sendGetTracker(workID)
	references := p.getReferencedPeople(doc, dataID)
	if len(references) > 0 {
		app.PersonalInformation.Reference1 = references[0]
	} else {
		utils.WriteContentToPage([]byte(content), "references-", "no-information")
	}
	if len(references) > 1 {
		app.PersonalInformation.Reference2 = references[1]
	}
	if len(references) > 2 {
		app.PersonalInformation.Reference3 = references[2]
	}
	if len(references) > 3 {
		app.PersonalInformation.Reference4 = references[3]
	}
	if len(references) > 4 {
		app.PersonalInformation.Reference5 = references[4]
	}

	return app, err
}

func constructCustomerApplicationFromContent(doc *html.Node, workID string) (*customer.Application, error) {

	// app := customer.Applications{}
	app := customer.NewApplication(true, customer.SourcePega, softwareclient.ApplicationPega7)

	app.WorkID = workID

	// doc := htmlquery.FindOne(doc, "//*[contains(@node_name, 'CaseInformation')]")
	// if doc == nil {
	// 	return nil, ErrEmptyContent
	// }

	app.FinancialInformation.LoanProduct = &customer.LoanProductScheme{}

	if v := getElementValue(doc, &app, "pxCurrentStageLabel", TypeString, nil, "CurrentStage"); v != nil {
		app.ApplicationStage = v.(string)
	} else {
		return nil, ErrEmptyContent
	}
	if nodes := htmlquery.Find(doc, "//*[@node_name='pyCaseHeader']//*[contains(@class, 'content-inner')]//*[contains(@class,'dataValueRead')]"); len(nodes) >= 3 {
		app.ApplicationSubStage = strings.Trim(htmlquery.InnerText(nodes[2]), " \n\t")
	}

	if v := getElementValueWithInnerText(doc, &app, "FullName", "Customer Name", TypeString, nil, "pyCaseSummary"); v != nil {
		app.PersonalInformation.FullName = v.(string)
	} else {
		logger.Root.Errorf("Error empty field `FullName`")
		return nil, ErrEmptyContent
	}

	if v := getElementValueWithInnerText(doc, &app, "", "Requested Amount", TypeFloat64, formatPrice, "LoanDetails||LoanAndProductDetails||pyCaseSummary"); v != nil {
		app.FinancialInformation.RequestedAmount = v.(float64)
		if app.FinancialInformation.RequestedAmount == 0 {
			return nil, ErrEmptyContent
		}
	} else {
		logger.Root.Errorf("Error empty field `RequestedAmount`.")
		return nil, ErrEmptyContent
	}

	if v := getElementValueWithInnerText(doc, &app, "RequestedTenure", "Requested Tenure", TypeInt32, nil, "LoanDetails||LoanAndProductDetails||pyCaseSummary"); v != nil {
		app.FinancialInformation.RequestedTerm = v.(int32)
	} else {
		logger.Root.Errorf("Error empty field `RequestedTenure`")
		return nil, ErrEmptyContent
	}
	if v := getElementValueWithInnerText(doc, &app, "POSName", "POS Name", TypeString, nil, "LoanDetails||LoanAndProductDetails||pyCaseSummary"); v != nil {
		app.FinancialInformation.POSName = v.(string)
	}

	if v := getElementValueWithInnerText(doc, &app, "POSCode", "POS Code", TypeString, nil, "LoanDetails||LoanAndProductDetails||pyCaseSummary"); v != nil {
		app.FinancialInformation.POSCode = v.(string)
	}

	if v := getElementValueWithInnerText(doc, &app, "pxCreateDateTime", "Created", TypeDateWithTime, nil, "pyCaseSummary"); v != nil {
		// 10/01/2021 21:19:41
		app.CreatedAt = v.(time.Time)
		app.ApplicationDate = v.(time.Time)
	}

	if v := getElementValueWithInnerText(doc, &app, "Gender", "Gender", TypeString, nil, "Identification"); v != nil {
		app.PersonalInformation.Gender = getGenderFromString(v.(string))
	} else {
		logger.Root.Errorf("Error empty field `Gender`")
		return nil, ErrEmptyContent
	}

	if v := getElementValueWithInnerText(doc, &app, "Hometown", "Hometown", TypeString, nil, "Identification"); v != nil {
		app.PersonalInformation.Address.City = v.(string)
		app.PersonalInformation.Address.CurrentAddress = v.(string)
	} else {
		logger.Root.Errorf("Error empty field `Hometown`")
		return nil, ErrEmptyContent
	}

	if v := getElementValueWithInnerText(doc, &app, "DateOfBirth", "Date of birth", TypeShortDate, nil, "Identification"); v != nil {
		// 10/01/2021
		app.PersonalInformation.Birthday = v.(time.Time)
	} else {
		if v := getElementValueWithInnerText(doc, &app, "DateOfBirth", "Date of birth", TypeShortDate, nil, "pyCaseSummary"); v != nil {
			app.PersonalInformation.Birthday = v.(time.Time)
		} else {
			logger.Root.Errorf("Error empty field `DateOfBirth`")
			return nil, ErrEmptyContent
		}
	}

	if v := getElementValueWithInnerText(doc, &app, "CCName", "CC Name", TypeString, nil, ""); v != nil {
		app.UserInput = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "CCCode", "CC Code", TypeString, nil, ""); v != nil {
		app.UserInputCode = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "ReferenceID", "Reference ID", TypeString, nil, ""); v != nil {
		app.ReferenceID = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "ProductType", "Product type", TypeString, nil, "ProductDetails"); v != nil {
		app.FinancialInformation.ProductType = v.(string)
		app.FinancialInformation.LoanProduct.ProductType = v.(string)
	} else {
		logger.Root.Errorf("Error empty field `ProductType`")
		return nil, ErrEmptyContent
	}

	if v := getElementValueWithInnerText(doc, &app, "SchemeDescription", "Scheme", TypeString, nil, "ProductDetails"); v != nil {
		app.FinancialInformation.ProductName = v.(string)
		app.FinancialInformation.LoanProduct.ProductName = v.(string)
	}

	if v := getElementValueWithInnerText(doc, &app, "ApplicationID", "Application ID", TypeUint64, nil, "pyCaseSummary"); v != nil {
		app.AppNo = v.(uint64)
	} else {
		logger.Root.Errorf("Error empty field `ApplicationID`")
		return nil, ErrEmptyContent
	}
	if v := getElementValueWithInnerText(doc, &app, "PhoneNumber", "Phone Number", TypeString, nil, "pyCaseSummary"); v != nil {
		app.PersonalInformation.Mobile = v.(string)
	} else {
		logger.Root.Errorf("Error empty field `PhoneNumber`")
		return nil, ErrEmptyContent
	}

	if v := getElementValueWithInnerText(doc, &app, "NationalId", "National ID", TypeString, nil, "pyCaseSummary"); v != nil {
		app.PersonalInformation.IDCardNumber = v.(string)
	} else {
		logger.Root.Errorf("Error empty field `NationalId`")
		return nil, ErrEmptyContent
	}

	if v := getElementValueWithInnerText(doc, &app, "NationalIdDateIssue", "Issue Date Of National ID", TypeShortDate, nil, "pyCaseSummary"); v != nil {
		// 10/01/2021
		t := v.(time.Time)
		app.PersonalInformation.IDCardIssueDate = &t
	} else {
		logger.Root.Errorf("Error empty field `NationalIdDateIssue`")
		return nil, ErrEmptyContent
	}

	if v := getElementValueWithInnerText(doc, &app, "DealerName", "Dealer Name", TypeString, nil, "pyCaseSummary"); v != nil {
		app.Good.DealerName = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "DealerCode", "Dealer Code", TypeString, nil, "pyCaseSummary"); v != nil {
		app.Good.DealerCode = v.(string)
	}

	if v := getElementValueWithInnerText(doc, &app, "TotalAssetCost", "Total Asset Cost", TypeFloat64, formatPrice, "AssetDetails"); v != nil {
		app.Good.TotalAssetCost = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "FinancedGoodType", "Goods Type", TypeString, nil, "AssetDetails"); v != nil {
		app.Good.GoodType = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "FinancedGoodBrand", "Goods Brand", TypeString, nil, "AssetDetails"); v != nil {
		app.Good.GoodBrand = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "Type", "Scheme Type", TypeString, nil, "AssetDetails"); v != nil {
		app.Good.SchemeType = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "AssetType", "Asset Type", TypeString, nil, "AssetDetails"); v != nil {
		app.Good.AssetType = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "AssetCategory", "Asset Category", TypeString, nil, "AssetDetails"); v != nil {
		app.Good.AssetCategory = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "AssetCost", "Asset Cost", TypeFloat64, formatPrice, "AssetDetails"); v != nil {
		app.Good.AssetCost = v.(float64)
	}

	if v := getElementValueWithInnerText(doc, &app, "EMEINumber", "EMEI Number", TypeString, nil, "AssetDetails"); v != nil {
		app.Good.EMEINumber = v.(string)
	}

	if v := getElementValueWithInnerText(doc, &app, "CollateralDescription", "Collateral Description", TypeString, nil, "AssetDetails"); v != nil {
		app.Good.Collateral = v.(string)
	}

	// INFORMATION - LOAN - SCHEME

	if v := getElementValueWithInnerText(doc, &app, "MinimumTenure", "Min Tenure Months", TypeInt32, nil, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.MinTentureMonths = v.(int32)
	}
	if v := getElementValueWithInnerText(doc, &app, "MaximumTenure", "Max Tenure Months", TypeInt32, nil, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.MaxTentureMonths = v.(int32)
	}
	if v := getElementValueWithInnerText(doc, &app, "MinimumTicketVND", "Min Amount", TypeFloat64, formatPrice, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.MinAmount = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "MaximumTicketVND", "Max Amount", TypeFloat64, formatPrice, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.MaxAmount = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "MinAge", "Min Age", TypeUint8, formatPrice, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.MinAge = v.(uint8)
	}
	if v := getElementValueWithInnerText(doc, &app, "MaxAge", "Max Age", TypeUint8, formatPrice, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.MaxAge = v.(uint8)
	}

	if v := getElementValueWithInnerText(doc, &app, "MinimumDownPayment", "Min Down Payment Percent", TypeFloat64, func(s string) string {
		return strings.ReplaceAll(strings.Trim(s, " \n\t"), "%", "")
	}, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.MinDownPaymentPercent = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "MaximumDownPayment", "Max Down Payment Percent", TypeFloat64, func(s string) string {
		return strings.ReplaceAll(strings.Trim(s, " \n\t"), "%", "")
	}, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.MaxDownPaymentPercent = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "EffectiveAnnualInterestRate", "Interest rate", TypeFloat64, func(s string) string {
		return strings.ReplaceAll(strings.Trim(s, " \n\t"), "%", "")
	}, "ProductDetails"); v != nil {
		app.FinancialInformation.EffectiveRate = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "MinimumDownpaymentRequired", "Min Down Payment  Amount", TypeFloat64, formatPrice, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.MinDownPaymentAmount = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "MaximumDownpaymentRequired", "Max Down Payment  Amount", TypeFloat64, formatPrice, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.MaxDownPaymentAmount = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "SchemeEndDate", "Expire Date", TypeShortDate, nil, "ProductDetails"); v != nil {
		app.FinancialInformation.LoanProduct.ExpireDate = v.(time.Time)
	}
	// can test with LV-2140183
	if v := getElementValueWithInnerText(doc, &app, "DisbursementChannel", "Disb. Channel", TypeString, nil, ""); v != nil {
		app.FinancialInformation.DisbursementChannel = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "BankAccountNumber", "Bank Account", TypeString, nil, "BankTransferSection"); v != nil {
		app.FinancialInformation.DisbursementBankAccountNumber = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "BankNameDesc", "Bank Name", TypeString, nil, "BankTransferSection"); v != nil {
		app.FinancialInformation.DisbursementBankName = v.(string)
	}

	// INFORMATION - LOAN - AMOUNTS
	if v := getElementValueWithInnerText(doc, &app, "DownPayment", "Down Payment", TypeFloat64, formatPrice, "LoanDetails"); v != nil {
		app.FinancialInformation.RequestedDownPayment = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "LoanPurposeIdDesc", "Loan Purpose ID", TypeString, nil, "LoanDetails"); v != nil {
		app.FinancialInformation.LoanPurpose = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "EMI", "EMI", TypeFloat64, formatPrice, "LoanDetails"); v != nil {
		app.FinancialInformation.EMI = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "InsuranceAmount", "Insurance Amount", TypeFloat64, formatPrice, "LoanDetails"); v != nil {
		app.FinancialInformation.InsuranceAmount = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "TotalCreditAmount", "Total Amount", TypeFloat64, formatPrice, "LoanDetails"); v != nil {
		app.FinancialInformation.RequestedAmountAndInsurance = v.(float64)
		if app.FinancialInformation.RequestedAmountAndInsurance == 0 {
			app.FinancialInformation.RequestedAmountAndInsurance = app.FinancialInformation.RequestedAmount
			//return nil, ErrEmptyContent
		}
	} else {
		if v := getElementValueWithInnerText(doc, &app, "", "Topup Amount", TypeFloat64, formatPrice, "LoanDetails"); v != nil {
			app.FinancialInformation.RequestedAmountAndInsurance = v.(float64)
		} else {
			if v := getElementValueWithInnerText(doc, &app, "", "Final Eligible Amount", TypeFloat64, formatPrice, "LoanDetails"); v != nil {
				app.FinancialInformation.RequestedAmountAndInsurance = v.(float64)
			} else {
				if v := getElementValueWithInnerText(doc, &app, "", "Cash Offered To Customer", TypeFloat64, formatPrice, "LoanDetails"); v != nil {
					app.FinancialInformation.RequestedAmountAndInsurance = v.(float64)
				} else {
					logger.Root.Errorf("Error empty field `RequestedAmountAndInsurance` with xpath %s.", makeXPathPega75("TotalCreditAmount", "LoanDetails", "Total Amount"))
					app.FinancialInformation.RequestedAmountAndInsurance = app.FinancialInformation.RequestedAmount
				}
				//return nil, ErrEmptyContent
			}
		}
	}

	if v := getElementValueWithInnerText(doc, &app, "OfferAmountLoan", "Offer Amount", TypeFloat64, formatPrice, "ApprovalAmounts"); v != nil {
		app.FinancialInformation.FinancedAmount = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "OfferTenureLoan", "Offer Tenure", TypeInt32, nil, "ApprovalAmounts"); v != nil {
		app.FinancialInformation.FinancedTerm = v.(int32)
	}
	if v := getElementValueWithInnerText(doc, &app, "OfferDownPayment", "Down Payment", TypeFloat64, formatPrice, "ApprovalAmounts"); v != nil {
		app.FinancialInformation.FinancedDownPayment = v.(float64)
	}
	if v := getElementValue(doc, &app, "TSAID", TypeString, nil, "TSADetail"); v != nil {
		app.TSACode = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "", "TSA Mobile Phone", TypeString, nil, "TSADetail"); v != nil {
		app.TSAPhone = v.(string)
	} else {
		logger.Root.Infof("There is no TSA phone")
	}

	if v := getElementValueWithInnerText(doc, &app, "", "TSA Name", TypeString, nil, "TSADetail"); v != nil {
		app.TSAName = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "SalesNotes", "Sales Notes", TypeString, nil, "TSADetail||CaseInformation"); v != nil && v.(string) != "" {
		app.Notes = append(app.Notes, customer.Note{Owner: "", Content: "tsa: " + v.(string), AtTime: utils.GetCurrentTime()})
	}

	if v := getElementValueWithInnerText(doc, &app, "DSAId", "DSA Code", TypeString, nil, "DSADetail"); v != nil {
		app.DSACode = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "", "DSA Mobile Phone", TypeString, nil, "DSADetail"); v != nil {
		app.DSAPhone = v.(string)
	} else {
		logger.Root.Infof("There is no DSA phone")
	}
	if v := getElementValueWithInnerText(doc, &app, "ServicePhone", "Service Phone", TypeString, nil, "DSADetailsExtension"); v != nil {
		app.DSAServicePhone = v.(string)
	}

	if v := getElementValueWithInnerText(doc, &app, "pyLastName", "DSA Name", TypeString, nil, "DSADetail"); v != nil {
		app.DSAName = v.(string)
	}

	if v := getElementValueWithInnerText(doc, &app, "ServicePhone", "Service Phone", TypeString, nil, "TSADetailsExtension"); v != nil {
		app.TSAServicePhone = v.(string)
	}

	// cc
	if v := getElementValueWithInnerText(doc, &app, "ServicePhone", "Service Phone", TypeString, nil, "CCDetailsExtension"); v != nil {
		app.ServicePhone = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "PhoneNumber", "Phone Number", TypeString, nil, "CCDetailsExtension"); v != nil {
		app.CCPhoneNumber = v.(string)
	}

	if v := getElementValueWithInnerText(doc, &app, "SalesNotes", "Sale Notes", TypeString, nil, "CCDetailsExtension"); v != nil && v.(string) != "" {
		app.Notes = append(app.Notes, customer.Note{Owner: "", Content: "cc: " + v.(string), AtTime: utils.GetCurrentTime()})
	}

	// INFORMATION - CUSTOMER
	if v := getElementValueWithInnerText(doc, &app, "ContactId",
		"Cif No.", TypeString, nil, "Identification"); v != nil {
		app.CIFNo = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "Age", "Age", TypeUint, nil, "Identification"); v != nil {
		app.PersonalInformation.Age = v.(uint)
	}
	lastName := ""
	if v := getElementValueWithInnerText(doc, &app, "LastName", "Last Name", TypeString, nil, "Identification"); v != nil {
		lastName = v.(string)
	}
	middleName := ""
	if v := getElementValueWithInnerText(doc, &app, "MiddleName", "Middle Name", TypeString, nil, "Identification"); v != nil {
		middleName = v.(string)
	}
	firstName := ""
	if v := getElementValueWithInnerText(doc, &app, "FirstName", "First name", TypeString, nil, "Identification"); v != nil {
		firstName = v.(string)
	}
	app.PersonalInformation.FullName = strings.Join([]string{lastName, middleName, firstName}, " ")

	if v := getElementValueWithInnerText(doc, &app, "NationalId", "National ID", TypeString, nil, "Identification"); v != nil {
		if app.PersonalInformation.IDCardNumber == "" {
			app.PersonalInformation.IDCardNumber = v.(string)
		}
	} else {
		logger.Root.Errorf("Error empty field `NationalId`")
		return nil, ErrEmptyContent
	}
	if v := getElementValueWithInnerText(doc, &app, "NationalIdDateIssue", "Date of Issue", TypeShortDate, nil, "Identification"); v != nil {
		if app.PersonalInformation.IDCardIssueDate == nil {
			t := v.(time.Time)
			app.PersonalInformation.IDCardIssueDate = &t
		}
	} else {
		logger.Root.Errorf("Error empty field `NationalIdDateIssue`")
		return nil, ErrEmptyContent
	}
	if v := getElementValueWithInnerText(doc, &app, "FBNumber", "FB Number", TypeString, nil, "Identification"); v != nil {
		app.PersonalInformation.FBNumber = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "PlaceOfIssueDesc", "Place of Issue", TypeString, nil, "Identification"); v != nil {
		app.PersonalInformation.IDCardIssuePlace = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "NationalID02", "National ID 02", TypeStringPointer, nil, "Identification"); v != nil {
		app.PersonalInformation.IDCardNumber2 = v.(*string)
	}
	if v := getElementValueWithInnerText(doc, &app, "DrivingLicenseNo", "Driving License No.", TypeStringPointer, nil, "Identification"); v != nil {
		app.PersonalInformation.DrivingLicenseNo = v.(*string)
	}
	if v := getElementValueWithInnerText(doc, &app, "DrivingLicenseSeriesNo", "Driving License Series No.", TypeStringPointer, nil, "Identification"); v != nil {
		app.PersonalInformation.DrivingLicenseSeriesNo = v.(*string)
	}
	if v := getElementValueWithInnerText(doc, &app, "IsFBOwner", "Family Book Owner", TypeBoolean, nil, "Identification"); v != nil {
		app.PersonalInformation.FamilyBookOwner = v.(bool)
	}
	if v := getElementValueWithInnerText(doc, &app, "MartialStatusId", "Marital status", TypeMarriage, nil, "Identification"); v != nil {
		app.PersonalInformation.MaritalStatus = v.(customer.MaritalStatus)
	}
	if v := getElementValueWithInnerText(doc, &app, "SocialStatus", "Social Status", TypeStringPointer, nil, "Identification"); v != nil {
		app.PersonalInformation.SocialStatus = v.(*string)
	}
	if v := getElementValueWithInnerText(doc, &app, "EducationId", "Education", TypeString, nil, "Identification"); v != nil {
		app.PersonalInformation.Education = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "SchoolName", "School Name", TypeStringPointer, nil, "Identification"); v != nil {
		app.PersonalInformation.School = v.(*string)
	}
	if v := getElementValueWithInnerText(doc, &app, "YearOfGraduation", "Year of Graduation", TypeInt32, nil, "Identification"); v != nil {
		app.PersonalInformation.YearOfGraduation = v.(int32)
	}

	spouse := &app.PersonalInformation.Reference1
	var sLastName, sMiddleName, sFirstName string
	// INFORMATION - FAMILY
	if v := getElementValueWithInnerText(doc, &app, "LastName", "Last Name", TypeString, nil, "Spouse"); v != nil {
		sLastName = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "MiddleName", "Middle Name", TypeString, nil, "Spouse"); v != nil {
		sMiddleName = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "FirstName", "First name", TypeString, nil, "Spouse"); v != nil {
		sFirstName = v.(string)
	}
	spouse.Name = strings.Join([]string{sLastName, sMiddleName, sFirstName}, " ")
	if v := getElementValueWithInnerText(doc, &app, "NationalId", "ID Card Number", TypeString, nil, "Spouse"); v != nil {
		spouse.IDCardNumber = v.(string)
	}
	spouse.Name = strings.Trim(spouse.Name, " ")
	if spouse.Name != "" {
		spouse.Relationship = "Vợ/chồng"
	}

	if v := getElementValueWithInnerText(doc, &app, "DateOfBirth", "Date of birth", TypeShortDate, nil, "Spouse"); v != nil {
		spouse.Birthday = v.(time.Time)
	}
	if v := getElementValueWithInnerText(doc, &app, "SaleCompName", "Sale Company Name", TypeString, nil, "CompanyDetail"); v != nil {
		app.PersonalInformation.Company = v.(string)
	}
	if v := getElementValue(doc, &app, "Position", TypeString, nil, "CompanyDetail"); v != nil {
		app.PersonalInformation.Position = v.(string)
	}
	if v := getElementValueWithInnerText(doc, &app, "YearsInPresentJob", "Current Work experience(Yrs)", TypeInt, nil, "CompanyDetail"); v != nil {
		app.PersonalInformation.NumWorkingYears = v.(int)
	}

	// INFORMATION  - INCOME
	if v := getElementValueWithInnerText(doc, &app, "GrossMonthlyIncome", "Gross Inc./mon.", TypeFloat64, formatPrice, "ObligationsSummary"); v != nil {
		app.PersonalInformation.Income = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "NetMonthlyAmount", "Net /mon.", TypeFloat64, formatPrice, "ObligationsSummary"); v != nil {
		app.PersonalInformation.IncomeNet = v.(float64)
	}
	if v := getElementValueWithInnerText(doc, &app, "ExpensesOfCustomerPerMonth", "Cust. Exp./month", TypeFloat64, formatPrice, "Expenses||ObligationsSummary"); v != nil {
		app.PersonalInformation.Expense = v.(float64)
	}

	return &app, nil
}

func makeXPathPega70(label, regionStr, innerText string) string {
	xpath := "//*[contains(@class, 'dataLabelForRead')"

	if label != "" {
		xpath += " and @for='" + label + "'"
	}

	if innerText != "" {
		xpath += " and contains(., '" + innerText + "')"
	}
	xpath += "]/following-sibling::div[1]/span"
	if regionStr != "" {
		regions := strings.Split(regionStr, "||")
		regionXpath := "//*["
		for i, r := range regions {
			if i > 0 {
				regionXpath += " or "
			}
			regionXpath += "@node_name='" + r + "'"
		}
		regionXpath += "]"
		xpath = regionXpath + xpath

	}
	return xpath
}

func makeXPathPega75(label, regionStr, innerText string) string {
	xpath := "//*[contains(@class, 'dataLabelForRead')"

	// if label != "" {
	// 	xpath += " and normalize-space()='" + innerText + "'"
	// }

	if innerText != "" {
		xpath += " and contains(translate(., 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), '" + strings.ToLower(innerText) + "')"
	}
	xpath += "]/following-sibling::div[1]/span"
	if regionStr != "" {
		regions := strings.Split(regionStr, "||")
		regionXpath := "//*["
		for i, r := range regions {
			if i > 0 {
				regionXpath += " or "
			}
			regionXpath += "@node_name='" + r + "'"
		}
		regionXpath += "]"
		xpath = regionXpath + xpath

	}
	return xpath
}

func getElementOfLabel(doc *html.Node, label string, region string, innerText string) (string, error) {

	// xpath := makeXPathPega70(label, region, innerText) + "|" + makeXPathPega75(label, region, innerText)
	xpath := makeXPathPega75(label, region, innerText)
	// fmt.Printf("xpath: `%s`\n", xpath)
	elm, err := htmlquery.Query(doc, xpath)
	if elm == nil || err != nil {
		// logger.Root.Errorf("Could not find %s with xpath `%s` err: %v", label, xpath, err)
		return "", ErrMissingField
	}
	txt := strings.Trim(htmlquery.InnerText(elm), " \n\t")
	if txt == "" {
		// logger.Root.Errorf("Empty value of %s with xpath `%s`. Raw value: `%v`", label, xpath, htmlquery.InnerText(elm))
	}

	return txt, nil

}

func getElementValue(doc *html.Node, app *customer.Application, field string, t DataType, replaceRawValue func(string) string, region string) interface{} {
	return getElementValueWithInnerText(doc, app, field, "", t, replaceRawValue, region)
}

func getElementValueWithInnerText(doc *html.Node, app *customer.Application, field string, innerText string, t DataType, replaceRawValue func(string) string, region string) interface{} {
	v, err := getElementOfLabel(doc, field, region, innerText)
	if err != nil {
		// logger.Root.Errorf("Error when getting value of %s in region %s: %v", field, region, err)
		// return nil
	}
	// fmt.Printf("value %v\n", v)

	if replaceRawValue != nil {
		v = replaceRawValue(v)
	}
	v = strings.ReplaceAll(v, "\u00A0", " ")
	v = strings.Trim(v, " \n\t")

	switch t {
	case TypeString:
		return v
	case TypeStringPointer:
		return &v
	case TypeBoolean:
		v = strings.ToLower(v)
		v = strings.Trim(v, " \n\t")
		if v == "true" {
			return true
		}
		return false
	case TypeMarriage:
		v = strings.ToLower(v)
		v = strings.Trim(v, " \n\t")
		if v == "single" {
			return customer.MaritalStatusSingle
		} else if v == "married" {
			return customer.MaritalStatusMarried
		} else if v == "divorced" {
			return customer.MaritalStatusDivorce
		} else {
			return customer.MaritalStatusUnknown
		}
	case TypeFloat64:
		vInt, err := strconv.ParseFloat(v, 64)
		if err != nil {
			// logger.Root.Errorf("Could not convert value `%s` to float64 in field %s", v, field)
			return nil
		}
		return vInt

	case TypeInt32:
		vInt, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			// logger.Root.Errorf("Could not convert value `%s` to int32 in field %s", v, field)
			return nil

		}
		return int32(vInt)
	case TypeInt:
		vInt, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			// logger.Root.Errorf("Could not convert value `%s` to int in field %s", v, field)
			return nil

		}
		return int(vInt)

	case TypeUint64:
		vInt, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			// logger.Root.Errorf("Could not convert value `%s` to uint64 in field %s", v, field)
			return nil

		}
		return vInt

	case TypeUint8:
		vInt, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			// logger.Root.Errorf("Could not convert value `%s` to uint64 in field %s", v, field)
			return nil
		}
		return uint8(vInt)
	case TypeUint:
		vInt, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			// logger.Root.Errorf("Could not convert value `%s` to uint64 in field %s", v, field)
			return nil
		}
		return uint(vInt)

	case TypeShortDate:
		if v == "" {
			return nil
		}
		if t, err := time.Parse("02/01/2006", v); err != nil {
			// logger.Root.Errorf("Could not parse the short date `%v` %v", v, err)
			return nil
		} else {
			return t
		}

	case TypeDateWithTime:
		if v == "" {
			return nil
		}
		if t, err := time.Parse("02/01/2006 15:04:05 -07:00", v+" +07:00"); err != nil {
			// logger.Root.Errorf("Could not parse the date with time `%v`. %v", v, err)
			return nil
		} else {
			return t
		}
	}

	return nil
}

func getGenderFromString(str string) customer.Gender {
	switch str {
	case "Female":
		return customer.GenderFemale
	case "Male":
		return customer.GenderMale
	default:
		return customer.GenderUnknown
	}
}

func (p *Base) getAddress(doc *html.Node, workID string) *customer.Address {
	addressEls := htmlquery.Find(doc, "//*[contains(@id, '$PpyWorkPage$pLoanApp$pPersonalInfo$pPerson$pAddresses$l')]")
	if len(addressEls) == 0 {
		logger.Root.Infof("There is no address")
		return nil
	} else {
		logger.Root.Infof("%d address found", len(addressEls))
	}

	// // send initial request to getTrackerChange
	// url := p.formatCommonURL(BaseURL + `!TABTHREAD1?pyActivity=pzGetTrackerChanges&pzTransactionId=@transactionID@&pzFromFrame=pyWorkPage&pzPrimaryPageName=pyWorkPage&inStandardsMode=true&AJAXTrackID=4&pzHarnessID=@harnessID@&HeaderButtonSectionName=SubSectionCaseContainerTopBBB`, workID)

	// logger.Root.Infof("Send request to getTrackerChange before fetching address url: %s", url)
	// payload := `&$OCAEventMgmt=&$OCompositeAPIInclude=&$ODesktopWrapperInclude=&$OEvalDOMScripts_Include=&$OHarnessInlineScripts=&$OLaunchFlowScriptInclude=&$OListViewIncludes=&$OMenuBar=&$OSpecCheckerScript=&$OStreamIncluded_SmartInfo_Script=&$OWorkformStyles=&$Ocpmpayload=&$OdocumentInfo_bundle=&$Ogridincludes=&$OmenubarInclude=&$OpyWorkFormStandard=&$OpzControlMenuScripts=&$OpzHarnessStaticScripts=&$OpzTextIncludes=&$OxmlDocumentInclude=`

	// referer := p.formatCommonURL(BaseURL + `!TABTHREAD1?pyActivity=Work-.Open&InsHandle=FECREDIT-BASE-LOAN-WORK%20@workID@&Action=Review&AllowInput=false&HarnessPurpose=Review&pzHarnessID=@harnessID@`, workID)

	// header := map[string]interface{}{
	// 	"credentials": "include",
	// 	"headers": map[string]string{
	// 		"User-Agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:84.0) Gecko/20100101 Firefox/84.0",
	// 		"Accept":           "*/*",
	// 		"Accept-Language":  "en-US,en;q=0.5",
	// 		"X-Requested-With": "XMLHttpRequest",
	// 		"Content-Type":     "application/x-www-form-urlencoded",
	// 	},
	// 	"referrer": referer,
	// 	"method":   "POST",
	// 	"mode":     "cors",
	// }
	// headerInByte, _ := json.Marshal(header)
	// p.Browser.SendAjax(url, "POST", payload, string(headerInByte), 240, func(result string, err error) {
	// 	if err != nil {
	// 		logger.Root.Errorf("Could not send the initial request to getTrackerChange %v", err)
	// 		return
	// 	}
	// 	logger.Root.Infof("result of initial rquest: %s", result)
	// })

	addressEls = htmlquery.Find(doc, "//*[contains(@id, '$PpyWorkPage$pLoanApp$pPersonalInfo$pPerson$pAddresses$')]//*[contains(@data-attribute-name, 'Address')]")

	if len(addressEls) == 0 {
		return nil
	}
	a := customer.Address{}
	for _, elm := range addressEls {
		// logger.Root.Infof("Get address %d", i)
		text := htmlquery.InnerText(elm)
		processAddressText(text, &a)
		// logger.Root.Infof("Address %d: %s", i, text)
	}
	if a.CurrentAddress != "" || a.PermenentAddress != "" || a.WorkAddress != "" {
		return &a
	}

	return nil
}

func processAddressText(text string, a *customer.Address) {
	text = strings.ReplaceAll(text, "[M],", "[M]")
	text = strings.ReplaceAll(text, " .,", "")
	// logger.Root.Infof("Process address: %s", text)
	parts := strings.Split(text, ",")

	if len(parts) >= 3 {
		if strings.Contains(text, "Permanent") {
			a.PermenentAddress = strings.ReplaceAll(text, "Permanent :", "")
		} else if strings.Contains(text, "Current") {
			city := strings.Trim(parts[len(parts)-1], " \t\u00A0")
			district := strings.Trim(parts[len(parts)-2], " \t\u00A0")
			ward := strings.Trim(parts[len(parts)-3], " \t\u00A0")
			ward = strings.ReplaceAll(ward, "Current :", "")
			ward = strings.Trim(strings.ReplaceAll(ward, "[M] ", ""), " ")

			a.CurrentAddress = strings.ReplaceAll(text, "Current :", "")
			a.City = city
			a.District = district
			a.Ward = ward
		} else if strings.Contains(text, "Office") {
			a.WorkAddress = strings.ReplaceAll(text, "Office :", "")
		} else {
			logger.Root.Infof("The address `%s` doesnt match any type", text)
		}
	}
}

func (p *Base) getReferencedPeople(doc *html.Node, workID string) []customer.Reference {
	addressEls := htmlquery.Find(doc, "//*[contains(@id, '$PpyWorkPage$pLoanApp$pPersonalInfo$pOtherReferensedPersons$l')]")
	if len(addressEls) == 0 {
		logger.Root.Infof("Found no referenced")
		return []customer.Reference{}
	}

	res := []customer.Reference{}

	for _, elm := range addressEls {
		// logger.Root.Infof("Fetch ref %d", i)
		tds := htmlquery.Find(elm, "//*[contains(@class, 'oflowDivM')]")
		if len(tds) >= 4 {
			r := customer.Reference{
				Name:         htmlquery.InnerText(tds[0]),
				Relationship: htmlquery.InnerText(tds[1]),
				IDCardNumber: htmlquery.InnerText(tds[2]),
				Phone:        htmlquery.InnerText(tds[3]),
			}

			res = append(res, r)
		}
	}
	// for i := range addressEls {
	// 	logger.Root.Infof("Fetch ref %d", i)
	// 	r := p.fetchReferenceAtIndex(i+1, workID)
	// 	if r != nil {
	// 		res = append(res, *r)
	// 	}
	// }
	return res
}

func formatPrice(value string) string {
	value = strings.ReplaceAll(value, ",", "")
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "VND", "")
	value = strings.ReplaceAll(value, "\u00A0", "")
	return value
}

func (p *Base) GoToReportPage() error {
	d := *(p.Browser.DriverInfo.Driver)
	currentURL, _ := d.CurrentURL()
	re := regexp.MustCompile(`https:\/\/.*/prweb(/PRWebLDAP2|)/(?P<id>[^/]+)/.*`)

	match := utils.MatchRegexInString(currentURL, re)
	var id string
	var ok bool
	if id, ok = match["id"]; !ok {
		if strings.Contains(currentURL, "/maintain") {
			return basecrawler.ErrMaintain
		}
		return fmt.Errorf("Invalid URL `%s`. Could not found the session id", currentURL)
	}
	logger.Root.Infof("`%v` Going to the report...", p.GetID())
	reportURL := strings.ReplaceAll(PageReportURL, "@id@", id)
	p.loginURLID = id
	logger.Root.Infof("report url: %s", reportURL)
	d.Get(reportURL)

	time.Sleep(5 * time.Second)
	err := d.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		errElement, err := d.FindElement(selenium.ByXPATH, "//*[@id='report_body']//th[contains(@class, 'cellCont')]")
		if err != nil {
			return false, err
		}

		displayed, _ := errElement.IsDisplayed()
		return displayed, nil
	}, 120*time.Second, 1*time.Second)

	if err != nil {
		logger.Root.Errorf("`%v` Could not go to the report. Err %v", p.GetID(), err)
		return err
	}

	logger.Root.Infof("ok")

	formElm, err := d.FindElement(selenium.ByXPATH, "//*[@id='RULE_KEY']//form")
	if err == nil && formElm != nil {
		actionURL, _ := formElm.GetAttribute("action")
		reg := regexp.MustCompile(".*TransactionId=(?P<id>[^&]+).*")
		match := utils.MatchRegexInString(actionURL, reg)
		if transactionID, ok := match["id"]; ok {
			p.transactionID = transactionID
			logger.Root.Infof("Transaction id: %s", transactionID)
		} else {
			p.transactionID = "e69fb54ed389ac388a10af25054a0d3a"
		}
	}
	time.Sleep(1 * time.Second)

	return nil
}

func (p *Base) formatCommonURL(url string, workID string) string {
	url = strings.ReplaceAll(url, "@workID@", workID)
	url = strings.ReplaceAll(url, "@transactionID@", p.transactionID)
	url = strings.ReplaceAll(url, "@id@", p.loginURLID)
	url = strings.ReplaceAll(url, "@harnessID@", p.harnessID)
	return url
}
