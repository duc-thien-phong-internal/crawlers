package cicblib

import (
	"errors"
	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"github.com/tebeka/selenium"
	"net/http/cookiejar"
	"strings"
	"time"
)

type Base struct {
	basecrawler.Worker
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

func (p *Base) FetchInformationOfProfile(dataID string, fetchFull bool) (*customer.Application, error) {
	return nil, errors.New("not implemented yet")
}
