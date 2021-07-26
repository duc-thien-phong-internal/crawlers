package cicblib

import (
	"context"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	"github.com/duc-thien-phong/techsharedservices/models/softwareclient"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"github.com/tebeka/selenium"
)

type Base struct {
	numFails       int
	cancelFunction context.CancelFunc
	basecrawler.Worker
}

func (p Base) StillEnoughPermission(errMessage string) bool {
	return !strings.Contains(errMessage, "do not match")
}

// expiration dialog
// <div id="j_id61::_ttxt" class="x186">Expiration Warning</div>
// <button type="button" id="j_id61::ok" class="xf6 p_AFTextOnly" onclick="return false;" _afrpdo="ok">OK</button>

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
		time.Sleep(2000 * time.Millisecond)

		err = d.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
			elm, err := d.FindElement(selenium.ByID, "username")
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
	errorMessageXpath := "//*[@id='loginbox']//*[contains(@class, 'content')]//*[contains(@class, 'alert')]"
	//errElement, err := d.FindElement(selenium.ByXPATH, errorMessageXpath)
	//if err == nil {
	//	errorMessage, _ = errElement.Text()
	//	display, _ := errElement.IsDisplayed()
	//	if display {
	//		errorMessage = strings.ToLower(errorMessage)
	//		// if strings.Contains(errorMessage, "maximum number of active sessions for the current operator has been reached") {
	//		// 	return errorMessage
	//		// }
	//		logger.Root.Errorf("!!!!! ErrorMessage before login: %s\n", errorMessage)
	//		text, _ := d.PageSource()
	//		utils.WriteContentToPage([]byte(text), "login-error-", "no-information")
	//		d.DeleteAllCookies()
	//		if strings.Contains(errorMessage, "blocked") {
	//			return "Found error message before login"
	//		}
	//	}
	//}

	txtUserID, err := d.FindElement(selenium.ByID, "username")

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

	txtPassword, err := d.FindElement(selenium.ByID, "password")
	if err != nil {
		logger.Root.Infof("Error when searching element txtPassword: %s\n", err)
	} else {
		txtPassword.SendKeys(accountInfo.Account.Password)
	}

	btnSubmit, err := d.FindElement(selenium.ByCSSSelector, "#login")
	if err != nil {
		logger.Root.Infof("Error when searching element btnSubmit: %s\n", err)
	} else {
		btnSubmit.Click()
	}

	time.Sleep(5 * time.Second)
	err = d.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		errElement, err := d.FindElement(selenium.ByXPATH, errorMessageXpath)
		if err != nil {
			return false, err
		}

		displayed, _ := errElement.IsDisplayed()
		if displayed {
			errorMessage, _ = errElement.Text()
			logger.Root.Errorf("LOGIN FAIL!!!!! Sleep 120 seconds %s", errorMessage)

			time.Sleep(120 * time.Second)
			return displayed, nil
		}
		_, err = d.FindElement(selenium.ByXPATH, "//*[contains(@id, 'userProfileButton')]")
		if err == nil {
			return true, nil
		} else {
			return false, err
		}
	}, 30*time.Second, 1*time.Second)

	logger.Root.Infof("ErrorMessage: %s\n", errorMessage)
	if errorMessage == "" {
		err = d.Get(exploreProfile)
		time.Sleep(500 * time.Millisecond)
		if err != nil {
			return err.Error()
		}
		err = d.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
			errElement, err := d.FindElement(selenium.ByXPATH, "//*[contains(@name, 'region1:txtmacic')]")
			if err != nil {
				return false, err
			}

			displayed, _ := errElement.IsDisplayed()
			return displayed, nil
		}, 30*time.Second, 1*time.Second)
		if err != nil {
			return err.Error()
		}

		go p.runBackgroundProcessToClickButtonOK()
	}

	return errorMessage
}

func (p *Base) LoginWithAccount(acc *basecrawler.UsefulAccountInfo) string {
	if err := p.goToLoginPage(LoginURL); err != nil {
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

func (p *Base) runBackgroundProcessToClickButtonOK() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelFunction = cancel
	var d selenium.WebDriver

	logger.Root.Infof("^^^ Start background process to keep active session")

mainLoop:
	for {

		select {
		case <-ctx.Done():
			logger.Root.Infof("Stop the background process because the context is done")
			break mainLoop
			// The default case is executed if none of the other cases are ready for execution.
			// It prevents the select from blocking the main goroutine since the operations are blocking by default.
		default:
			break
		}

		if p.Browser != nil && p.Browser.DriverInfo.Driver != nil {

			d = *p.Browser.DriverInfo.Driver

			source, err := d.PageSource()
			if err == nil {
				if strings.Contains(source, "Expiration Warning") {
					btnOKs, err := d.FindElements(selenium.ByXPATH, "//*[contains(@id, '::ok') and contains(@id, 'j_id')]")

					if err == nil && len(btnOKs) > 0 {
						// btnOK := btnOKs[len(btnOKs)-1]
						for _, btnOK := range btnOKs {
							btnOK.MoveTo(0, 0)
							btnOK.Click()
							logger.Root.Infof("Click on button OK to avoid expiration.....")
							time.Sleep(100 * time.Millisecond)
						}
					} else {
						logger.Root.Errorf("Could not find the button to close the warning: %s", err)
					}
				}
				if strings.Contains(source, "Page Expired - S37- Cảnh báo") ||
					strings.Contains(source, "This page can't be displayed. Contact support for additional information") {
					logger.Root.Errorf("The page is expired. Need to be closed")
					p.NeedToClose = true
					break mainLoop
				}
			} else {
				logger.Root.Errorf("Error when getting page source: %s", err)
			}

			url, _ := (*p.Browser.DriverInfo.Driver).CurrentURL()
			if strings.Contains(url, "WCPageNotFound") {
				p.NeedToClose = true
				break mainLoop
			}

		} else {
			logger.Root.Infof("Checking expiration but browser is nil.....")
		}

		time.Sleep(10 * time.Second)
	}
}

func (p *Base) Close() error {
	if p.Browser != nil {
		p.Browser.Close()
	}

	if p.cancelFunction != nil {
		p.cancelFunction()
	}

	return nil
}

func parseAddress(address string) (city string, district string, ward string) {
	parts := strings.Split(address, ",")
	if len(parts) > 0 {
		city = strings.Trim(parts[len(parts)-1], " \n\t")
	}
	if len(parts) > 2 {
		district = strings.Trim(parts[len(parts)-2], " \n\t")
	}
	if len(parts) > 3 {
		ward = strings.Trim(parts[len(parts)-3], " \n\t")
	}
	return
}

func parsePageToGetInformation(htmlsource string, app *customer.Application) error {
	//*[contains(@id, 'region1:txtmacic::content')]
	doc, err := htmlquery.Parse(strings.NewReader(htmlsource))

	if err != nil {
		logger.Root.Errorf("\n\n\nError when parsing xml: %s\n\n\n", err)
		return err
	}

	customerNameElm := htmlquery.FindOne(doc, "//input[contains(@id, 'region1:txttenkh')]")
	if customerNameElm == nil {
		logger.Root.Errorf("Could not found element customerName")
		return ErrNoCustomer
	}
	customerName := strings.Trim(htmlquery.SelectAttr(customerNameElm, "value"), " \n")
	// if we have found the information of customer
	if customerName != "" {
		app.ApplicationDate = time.Now()
		app.PersonalInformation.FullName = customerName
		if addressElm := htmlquery.FindOne(doc, "//input[contains(@id, 'region1:txtdiachi')]"); addressElm != nil {
			address := htmlquery.SelectAttr(addressElm, "value")
			app.PersonalInformation.Address.CurrentAddress = address
			app.PersonalInformation.Address.PermenentAddress = address
			city, district, ward := parseAddress(address)
			app.PersonalInformation.Address.City = city
			app.PersonalInformation.Address.District = district
			app.PersonalInformation.Address.Ward = ward
		} else {
			logger.Root.Errorf("Could not found element addressElm")
		}
		if elm := htmlquery.FindOne(doc, "//input[contains(@id, 'region1:txtmacic')]"); elm != nil {
			value := htmlquery.SelectAttr(elm, "value")
			value = strings.Trim(value, " \t\n")
			app.CurrentCICCode = value
			app.WorkID = value
			app.AppNo = basecrawler.GenerateAppNoFromTimeStamp(uint64(time.Now().Nanosecond()/1000), softwareclient.ApplicationCICB)
		} else {
			logger.Root.Errorf("Could not found element txtMagic::content")
		}

		if elm := htmlquery.FindOne(doc, "//input[contains(@id, 'region1:txtcmnd')]"); elm != nil {
			value := htmlquery.SelectAttr(elm, "value")
			value = strings.Trim(value, " \t\n")
			app.PersonalInformation.IDCardNumber = value
		} else {
			logger.Root.Errorf("Could not found element txtcmnd::content")
		}

		if elm := htmlquery.FindOne(doc, "//input[contains(@id, 'region1:txtdkkd')]"); elm != nil {
			value := htmlquery.SelectAttr(elm, "value")
			value = strings.Trim(value, " \t\n")
			app.PersonalInformation.EnterpriseRegistrationNo = value
		} else {
			logger.Root.Errorf("Could not found element txtdkkd::content")
		}

		if elm := htmlquery.FindOne(doc, "//input[contains(@id, 'region1:txtdienthoai')]"); elm != nil {
			value := htmlquery.SelectAttr(elm, "value")
			value = strings.Trim(value, " \t\n")
			if len(value) >= 8 && len(value) <= 13 {
				app.PersonalInformation.Mobile = value
			}
		} else {
			logger.Root.Errorf("Could not found element txtdienthoai::content")
		}

		if elm := htmlquery.FindOne(doc, "//input[contains(@id, 'region1:txtmsthue')]"); elm != nil {
			value := htmlquery.SelectAttr(elm, "value")
			value = strings.Trim(value, " \t\n")
			app.PersonalInformation.TaxCode = value
		} else {
			logger.Root.Errorf("Could not found element txtmsthue::content")
		}

		elms := htmlquery.Find(doc, "//*[contains(@id, 'region1:panel_timkiemkh')]//*[contains(@id, 'T:oc_6025695556region1:j_id__')]")
		if len(elms) > 0 {
			elm := elms[len(elms)-1]
			app.CurrentCICInfo = htmlquery.InnerText(elm)
			result := utils.MatchRegexInString(app.CurrentCICInfo, regexp.MustCompile(`.*(?P<message>Khách hàng [^:]*)`))
			if message, exist := result["message"]; exist {
				app.CurrentCICInfo = message
			}
		}

	} else {
		// no customer with this CIC code
		//cicElm := htmlquery.FindOne(doc, "//*[contains(@id, 'region1:txtmacic::content')]")
		utils.WriteContentToPage([]byte(htmlsource), "cicb-empty-info-"+app.CurrentCICCode, "cicb-logs")
		logger.Root.Errorf("The value of txttenkh::content is empty")
		return ErrNoCustomer
	}

	return nil
}

// //*[contains(@class, 'parent-link')][last()]
// 8931915515
// 8931916245
// 8931916240
func (p *Base) getCustomerWarning(dataID string) (*customer.Application, error) {
	d := *p.Browser.DriverInfo.Driver
	if v, err := d.CurrentURL(); !strings.Contains(v, "https://cic.org.vn/webcenter/portal/CMSPortal/page1422/page1600?") {
		if err != nil {
			return nil, err
		}
		err = d.Get("https://cic.org.vn/webcenter/portal/CMSPortal/page1422/page1600?_afrLoop=514634572325924&_adf.ctrl-state=3cylzi5kv_22")
		time.Sleep(300 * time.Millisecond)
	}
	source, err := d.PageSource()
	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("could not get the source of the page")
		return nil, err
	}
	if strings.Contains(source, "Expiration Warning") {
		btnOK, err := d.FindElement(selenium.ByXPATH, "//*[contains(@id, 'j_id57::ok')]")
		if err != nil {
			logger.Root.Errorf("Could not click on button OK to avoid expiring")
			return nil, err
		}
		if btnOK != nil {
			btnOK.MoveTo(0, 0)
			btnOK.Click()
			p.SetNeedToBeReplaced(true)
			return nil, ErrInvalidSession
		}
	}
	// Expiration Warning
	// class=j_id57::_fcc
	err = basecrawler.WaitUntilElementDisplay(d, "//*[contains(@id, 'region1:btnhotrotimkiem')]", 45, 1, 3, 3)

	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Could not wait until button HoTroTimKiem appear %s. Refresh the page", err)
		d.Refresh()
	}

	var inputElm selenium.WebElement
	switch dataID[0] {
	case 'N':
		// Name
		inputElm, err = d.FindElement(selenium.ByXPATH, "//input[contains(@id, 'region1:txttenkh')]")
	case 'C':
		// CMND
		inputElm, err = d.FindElement(selenium.ByXPATH, "//input[contains(@id, 'region1:txtcmnd')]")
	case 'T':
		// Ma so thue
		inputElm, err = d.FindElement(selenium.ByXPATH, "//input[contains(@id, 'region1:txtmsthue')]")
	case 'B':
		// Business number
		inputElm, err = d.FindElement(selenium.ByXPATH, "//input[contains(@id, 'region1:txtdkkd')]")
	//case 'F':
	//	// CIF
	//	inputElm, err = d.FindElement(selenium.ByXPATH, "//*[contains(@id, 'region1:txtmacic')]")
	default:
		// CIC code
		inputElm, err = d.FindElement(selenium.ByXPATH, "//input[contains(@name, 'region1:txtmacic')]")
		dataID = " " + dataID
	}
	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Could not found element, err: %v", err)
		return nil, err
	}
	if dataID[1:] == "" {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Infof("The input data is empty: %s", dataID)
		return nil, ErrMissingField
	}

	if alertText, err := d.AlertText(); err == nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Infof("Accept alert %s", alertText)
		d.AcceptAlert()
		time.Sleep(200 * time.Millisecond)
		p.clickButtonYes(dataID, d, false, false)
	}
	inputElm.MoveTo(0, 0)
	//err = inputElm.SendKeys(selenium.ControlKey + "a")
	//err = inputElm.SendKeys(selenium.DeleteKey + dataID[1:])
	inputElm.Click()
	inputElm.Clear()
	time.Sleep(400 * time.Millisecond)
	err = inputElm.SendKeys(dataID[1:])
	//d.ExecuteScript(fmt.Sprintf("arguments[0].value =='' || arguments[0].value='%s'", dataID[1:]), []interface{}{inputElm})
	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Could not send key %s", err)
	}
	logger.Root.Infof("Type CIC: %s", dataID[1:])
	time.Sleep(500 * time.Millisecond)
	if alertText, err := d.AlertText(); err == nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Infof("Accept alert %s", alertText)
		d.AcceptAlert()
		time.Sleep(200 * time.Millisecond)
		p.clickButtonYes(dataID, d, false, false)
	}

	modalElement, err := d.FindElement(selenium.ByXPATH, "//*[contains(@class, 'AFModalGlassPane')]")
	if err == nil && modalElement != nil {
		display, _ := modalElement.IsDisplayed()
		if display {
			logger.Root.With("id", p.GetID()).With("dataID", dataID).Infof("There is a modal on the screen")
			basecrawler.WaitUntilElementDisappear(d, "//*[contains(@class, 'AFModalGlassPane')]", 20, 1, 1, 3)
		}
	}

	btnSearchElm, err := d.FindElement(selenium.ByXPATH, "//*[contains(@id, 'region1:btnhotrotimkiem')]")
	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Could not find element region1:btnhotrotimkiem %s", err)
		return nil, err
	}
	btnSearchElm.MoveTo(0, 0)
	err = btnSearchElm.Click()
	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Could not click on Search button %s", err)
	}

	time.Sleep(300 * time.Millisecond)
	var htmlsource string
	if err := p.handleAlertIfAny(dataID, d, htmlsource); err != nil {
		time.Sleep(200 * time.Millisecond)
		p.clickButtonYes(dataID, d, false, false)
		return nil, err
	}

	err = basecrawler.WaitUntilElementDisplay(d, "//*[contains(@id, 'region1:tnYes')]", 20, 1, 1, 3)
	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Could not wait until the element display %s", err)
	}

	if err := p.clickButtonYes(dataID, d, true, true); err != nil {
		return nil, err
	} else {
		basecrawler.WaitUntilElementDisplay(d, "//*[contains(@id, 'region1:txttenkh::content')]", 30, 1, 1, 2)
	}

	if err := p.handleAlertIfAny(dataID, d, htmlsource); err != nil {
		time.Sleep(200 * time.Millisecond)
		p.clickButtonYes(dataID, d, false, false)
		return nil, err
	}

	application := customer.NewApplication(true, customer.SourceCIB, softwareclient.ApplicationCICB)
	err = utils.RetryOperation(func() error {
		htmlsource, _ = d.PageSource()
		if err := parsePageToGetInformation(htmlsource, &application); err != nil {
			logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Could not parse page to get information of application %s", err)
			return err
		}
		return nil
	}, 3, 1)

	if err != nil {
		return nil, err
	}

	//
	//// //*[contains(@id, 'region1:panel_timkiemkh')][last()]//*[contains(@id, 'T:oc_6025695556region1:j_id__')]
	//resultRegionElm, err := d.FindElement(selenium.ByXPATH,
	//	"//*[contains(@id, 'region1:panel_timkiemkh')][last()]//*[contains(@id, 'T:oc_6025695556region1:j_id__')]")
	//if err != nil {
	//	return "", err
	//}

	return &application, nil

}
func (p *Base) clickButtonYes(dataID string, d selenium.WebDriver, onBtnYes bool, mustExist bool) error {
	xpath := "//*[contains(@id, 'region1:tnYes')]"
	if !onBtnYes {
		xpath = "//*[contains(@id, 'region1:tnNo')]"
	}
	btnYesElm, err := d.FindElement(selenium.ByXPATH, xpath)
	if err != nil && mustExist {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Could not find button tnYes. %s", err)
		return err
	}

	err = btnYesElm.Click()
	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Could not click on button yes. %s", err)
	} else {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Infof("click on button Yes")
	}
	time.Sleep(300 * time.Millisecond)
	basecrawler.WaitUntilElementDisappear(d, xpath, 10, 1, 1, 2)
	return nil
}

// return error if the alert text shows that we should stop the process
func (p *Base) handleAlertIfAny(dataID string, d selenium.WebDriver, htmlsource string) error {
	if alertText, _ := d.AlertText(); alertText != "" {
		htmlsource, _ := d.PageSource()
		if strings.Contains(htmlsource, "không hợp lệ") || strings.Contains(htmlsource, "Bạn chưa nhập") {
			logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Error when clicking on Search button: %s", alertText)
			d.AcceptAlert()
			logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Information is not valid")
			return ErrNoCustomer
		}
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Infof("Got alert message %s", alertText)
	}
	return nil
}

func (p *Base) FetchInformationOfProfile(dataID string, fetchFull bool) (*customer.Application, error) {
	if strings.IndexByte("NCTBF", dataID[0]) < 0 {
		// this is a CIC code
		return p.getCustomerWarning(dataID)
	}

	d := *p.Browser.DriverInfo.Driver
	url, _ := d.CurrentURL()
	if url != exploreProfile {
		err := d.Get(exploreProfile)
		if err != nil {
			logger.Root.Errorf("Could not access the exploring page to fetch data id: %v", dataID)
			return nil, err
		}
		time.Sleep(500 * time.Millisecond)
		err = basecrawler.WaitUntilElementDisplay(d, "//input[contains(@name, 'region1:txtsocmt')]", 30, 1, 2, 3)
		if err != nil {
			logger.Root.Errorf("Browerser `%s` Cannot wait until element region1:txtsocmt displays: %v. %v", p.GetID(), dataID, err)
			return nil, err
		}
	}

	if len(dataID) == 0 {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Missing data id")
		return nil, ErrMissingField
	}
	logger.Root.With("id", p.GetID()).With("dataID", dataID).Infof("Selecting customer type")
	err := p.selectCustomerType(d)
	if err != nil {
		return nil, err
	}
	logger.Root.With("id", p.GetID()).With("dataID", dataID).Infof("Selecting customer region")
	err = p.selectCustomerRegion(d)
	if err != nil {
		return nil, err
	}

	logger.Root.With("id", p.GetID()).With("dataID", dataID).Infof("Set customer query info")
	switch dataID[0] {
	case 'N':
		// Name
		err = p.setCustomerName(d, dataID[1:])
	case 'C':
		// CMND
		err = p.setCustomerIdCard(d, dataID[1:])
	case 'T':
		// Ma so thue
		err = p.setCustomerTaxNumber(d, dataID[1:])
	case 'B':
		// Business number
		err = p.setCustomerBusinessNumber(d, dataID[1:])
	case 'F':
		// Business number
		err = p.setCustomerCIF(d, dataID[1:])
	default:
		// CIC code
		err = p.setCustomerCIC(d, dataID[1:])
	}
	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Error when setting customer query info: %v", err)
		return nil, err
	}

	btn, err := d.FindElement(selenium.ByXPATH, "//*[contains(@id, 'region1:btntimkiemkh')]")
	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Error when looking for button btntimkiemkh: %v", err)
		return nil, err
	}
	btn.MoveTo(0, 0)
	btn.Click()
	time.Sleep(300 * time.Millisecond)

	basecrawler.WaitUntilElementDisplay(d, "//td[@class='x10j']", 10, 1, 1, 3)
	tdList, err := d.FindElements(selenium.ByXPATH, "//td[@class='x10j']")
	if err != nil {
		logger.Root.With("id", p.GetID()).With("dataID", dataID).Errorf("Could not find any result: %v", err)
		return nil, err
	}
	if len(tdList) > 0 {
		CIBCode, _ := tdList[0].Text()
		CIBCode = strings.Trim(CIBCode, " \t\n")
		// customerName, _ := tdList[1].Text()
		// customerName = strings.Trim(customerName, " \t\n")
		// address, _ := tdList[2].Text()
		// address = strings.Trim(address, " \t\n")
		// taxCode, _ := tdList[3].Text()
		// taxCode = strings.Trim(taxCode, " \t\n")
		// businessCode, _ := tdList[4].Text()
		// businessCode = strings.Trim(businessCode, " \t\n")
		// idCardNumber, _ := tdList[5].Text()
		// idCardNumber = strings.Trim(idCardNumber, " \t\n")

		// application := customer.NewApplication(true, customer.SourceCIB, softwareclient.ApplicationCICB)
		// application.CurrentCICCode = CIBCode
		// application.PersonalInformation = customer.PersonalInformation{
		// 	FullName: customerName,
		// 	Address: customer.Address{
		// 		CurrentAddress:   address,
		// 		PermenentAddress: address,
		// 	},
		// 	IDCardNumber:             idCardNumber,
		// 	TaxCode:                  taxCode,
		// 	EnterpriseRegistrationNo: businessCode,
		// }

		return p.getCustomerWarning(CIBCode)
		// return &application, nil
	}
	logger.Root.Errorf("Could not find any result")
	logger.Root.With("id", p.GetID()).With("dataID", dataID).Infof("Could not find any result")

	return nil, err

	// return p.getCustomerWarning(dataID)
}

func (p *Base) selectCustomerType(d selenium.WebDriver) error {
	customerTypeElm, err := d.FindElement(selenium.ByXPATH, "//*[contains(@name, 'region1:strLoaiKH')]")
	if err != nil {
		logger.Root.Errorf("Could not find the selectbox CustomerType")
		return err
	} else {
		customerTypeElm.MoveTo(0, 0)
		customerTypeElm.Click()
		time.Sleep(100 * time.Millisecond)
		customerType1Elm, err := d.FindElement(selenium.ByXPATH, "//*[contains(@name, 'region1:strLoaiKH')]//option[@value='1']")
		if err != nil {
			logger.Root.Errorf("Could not find customer type 1")
			return err
		}
		customerType1Elm.Click()
	}
	return nil
}

func (p *Base) selectCustomerRegion(d selenium.WebDriver) error {
	customerRegionElm, err := d.FindElement(selenium.ByXPATH, "//*[contains(@name, 'region1:soc10')]")
	if err != nil {
		logger.Root.Errorf("Could not find the selectbox CustomerRegion")
		return err
	} else {
		customerRegionElm.MoveTo(0, 0)
		customerRegionElm.Click()
		time.Sleep(100 * time.Millisecond)
		customerRegion0Elm, err := d.FindElement(selenium.ByXPATH, "//*[contains(@name, 'region1:soc10')]//option[@value='0']")
		if err != nil {
			logger.Root.Errorf("Could not find customer Region 0")
			return err
		}
		return customerRegion0Elm.Click()
	}
}

func (p *Base) setCustomerIdCard(d selenium.WebDriver, idCardNum string) error {
	customerIdCardElm, err := d.FindElement(selenium.ByXPATH, "//input[contains(@id, 'region1:txtsocmt')]")
	if err != nil {
		logger.Root.Errorf("Could not find the selectbox customer id card number")
		return err
	} else {
		return customerIdCardElm.SendKeys(idCardNum)
	}
}

func (p *Base) setCustomerCIC(d selenium.WebDriver, CIC string) error {
	customerIdCardElm, err := d.FindElement(selenium.ByXPATH, "//input[contains(@id, 'region1:txtmacic')]")
	if err != nil {
		logger.Root.Errorf("Could not find the selectbox Customer cic")
		return err
	} else {
		return customerIdCardElm.SendKeys(CIC)
	}
}

func (p *Base) setCustomerTaxNumber(d selenium.WebDriver, value string) error {
	customerIdCardElm, err := d.FindElement(selenium.ByXPATH, "//input[contains(@name, 'region1:txtmsthue')]")
	if err != nil {
		logger.Root.Errorf("Could not find the selectbox Customer tax number")
		return err
	} else {
		return customerIdCardElm.SendKeys(value)
	}
}

func (p *Base) setCustomerCIF(d selenium.WebDriver, CIC string) error {
	customerIdCardElm, err := d.FindElement(selenium.ByXPATH, "//input[contains(@id, 'region1:txtcif')]")
	if err != nil {
		logger.Root.Errorf("Could not find the selectbox Customer cif")
		return err
	} else {
		return customerIdCardElm.SendKeys(CIC)
	}
}

func (p *Base) setCustomerName(d selenium.WebDriver, value string) error {
	customerIdCardElm, err := d.FindElement(selenium.ByXPATH, "//input[contains(@id, 'region1:txttenkh')]")
	if err != nil {
		logger.Root.Errorf("Could not find the selectbox Customer name")
		return err
	} else {
		return customerIdCardElm.SendKeys(value)
	}
}

func (p *Base) setCustomerBusinessNumber(d selenium.WebDriver, value string) error {
	customerIdCardElm, err := d.FindElement(selenium.ByXPATH, "//input[contains(@id, 'region1:txtdkkd')]")
	if err != nil {
		logger.Root.Errorf("Could not find the selectbox Customer business number")
		return err
	} else {
		return customerIdCardElm.SendKeys(value)
	}
}
