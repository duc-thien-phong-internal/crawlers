package my_selenium

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"io/ioutil"
	"log"
	"time"

	"github.com/tebeka/selenium"
)

// Instruction shows the information required to explore all information on a tab
type Instruction struct {
	Name             string        `json:"name"`
	XPath            string        `json:"xpath"`
	Type             string        `json:"type"`
	Label            string        `json:"label"`
	Attribute        string        `json:"attr"`
	Required         bool          `json:"required"`
	SubInstructions  []Instruction `json:"instructions"`
	Function         string        `json:"function"`
	IsMultipleValues bool          `json:"is_multiple" default:"false"`
}

// QDETab represents the information of a tab on QDE page
type QDETab struct {
	Tab          string        `json:"tab"`
	Position     uint          `json:"position"`
	Page         string        `json:"page"`
	TabKey       string        `json:"tabKey"`
	Instructions []Instruction `json:"instructions"`
	Active       bool          `json:"active" default:"true"`
}

var metaTrTdInfo map[int]map[int]string = map[int]map[int]string{
	1: {1: "customerAddress", 3: "assetDetails"},
	2: {1: "rateOfInterest", 3: "amountFinanced"},
	3: {1: "creditTerm", 3: "encodedDate"},
	4: {1: "stage"},
	5: {1: "status", 3: "statusDate"},
	6: {3: "reasonCode"}, 7: {1: "sourceCode", 3: "referrerName"},
	8: {1: "scheme", 3: "CIF"}, 9: {1: "phone", 3: "DOB"},
	10: {1: "lastUserId", 3: "lastUsername"},
	11: {1: "contractNumber"},
}

type InstructionFunction func(string) string

// F1Browser defines the structure of web browser that will be used to check f1 profile
type F1Browser struct {
	*Browser

	HomeTab    string
	CASTab     string
	EnquiryTab string

	// Token used in search
	Token string

	// DB  *mongo.Database
	Ctx context.Context
}

// // GetNeedToBeReplaced returns true if this browser session needs to be replaced by another session
// func (f1 *F1Browser) GetNeedToBeReplaced() bool {
// 	if f1 == nil {
// 		return false
// 	}
// 	now := utils.GetCurrentTime().Unix()
// 	if now-f1.CreatedTime >= MAX_AGE-int64(rand.Intn(3*60)) {
// 		return true
// 	}
// 	return false
// }

// // NeedToBeReset returns true if we need to type and hit Enter into search text box
// func (f1 *F1Browser) NeedToBeReset() bool {
// 	if f1 == nil {
// 		return false
// 	}
// 	now := utils.GetCurrentTime().Unix()
// 	if now-f1.CreatedTime >= MAX_TIME_TO_RESET-int64(rand.Intn(3*60)) {
// 		return true
// 	}
// 	return false
// }

// // SetActive sets the active value for this browser
// func (f1 *F1Browser) SetActive(a bool) {
// 	if f1 == nil {
// 		return
// 	}
// 	f1.Lock()
// 	f1.isActive = a
// 	f1.Unlock()
// }

// // IsActive returns if this browser is active or not
// func (f1 *F1Browser) IsActive() bool {
// 	if f1 == nil {
// 		return false
// 	}
// 	f1.Lock()
// 	defer f1.Unlock()
// 	return f1.isActive
// }

// // SetBusy sets the active value for this browser
// func (f1 *F1Browser) SetBusy(a bool) {
// 	if f1 == nil {
// 		return
// 	}
// 	f1.Lock()
// 	f1.isBusy = a
// 	f1.Unlock()
// }

// // IsBusy returns if this browser is active or not
// func (f1 *F1Browser) IsBusy() bool {
// 	if f1 == nil {
// 		return false
// 	}
// 	f1.Lock()
// 	defer f1.Unlock()
// 	return f1.isBusy
// }

// // checkStopRequest reads the request from server to see if we should stop the crawler or not
// // if actionNow = true, we mark the crawler to be closed, and set busy to false
// func (f1 *F1Browser) checkStopRequestAndClose(actionNow bool) (stopCrawler bool) {
// 	// 	if f1 == nil {
// 	// 		return true
// 	// 	}
// 	// 	configCrawler, _ = getCrawlerConfiguration()
// 	// 	if configCrawler == nil {
// 	// 		return false
// 	// 	}
// 	// 	logger.Root.Infof("Current status: %s\n", configCrawler.Value.StatusRequested)
// 	// 	needToClose := false
// 	// 	if f1 == nil {
// 	// 		return true
// 	// 	}
// 	// 	if f1.Type == TypeCrawler {
// 	// 		needToClose = configCrawler.Value.StatusRequested == Stopping || configCrawler.Value.StatusRequested == Stopped
// 	// 	} else {
// 	// 		needToClose = configWorker.Value.StatusRequested == Stopping || configWorker.Value.StatusRequested == Stopped
// 	// 	}

// 	// 	if needToClose {
// 	// 		if actionNow {
// 	// 			if f1.Type == TypeCrawler {
// 	// 				logger.Root.Infoln("Close crawler")
// 	// 			} else {
// 	// 				logger.Root.Infoln("Close worker")
// 	// 			}

// 	// 			f1.needToClose = true
// 	// 			f1.SetBusy(false)
// 	// 		}
// 	// 		return true
// 	// 	}
// 	return false
// }

// QDETabs the instruction to explore the data of the QDE page
var QDETabs []QDETab

func getTabs(driver selenium.WebDriver) (tabs []string, err error) {
	tabs, err = driver.WindowHandles()
	if err != nil {
		logger.Root.Errorf("Cannot get list of windows handles. Error: %s\n", err)
		return []string{}, err
	}

	return
}

// func openURLInNewTab(driver selenium.WebDriver, url string) (newTab string, err error) {
// 	logger.Root.Infoln("Openning new tab....")
// 	oldTabs, err := getTabs(driver)
// 	if err != nil {
// 		return "", err
// 	}

// 	driver.ExecuteScript(fmt.Sprintf("window.open('%s','_blank');", ENQUIRY_URL), []interface{}{})

// 	time.Sleep(1)

// 	newTabs, err := getTabs(driver)
// 	if err != nil {
// 		return "", err
// 	}

// 	for _, t1 := range newTabs {
// 		found := false
// 		for _, t2 := range oldTabs {
// 			if t1 == t2 {
// 				found = true
// 				break
// 			}
// 		}
// 		if !found {
// 			newTab = t1
// 			break
// 		}
// 	}

// 	return
// }

const QDEFilename = "instructions.json"

func loadQDEInstructions() {
	file, _ := ioutil.ReadFile(QDEFilename)
	if err := json.Unmarshal([]byte(file), &QDETabs); err != nil {
		log.Printf("Error when parsing json to get the instructions: %s\n", err)
		log.Println(file)
		// return err
		return
	}

}

// func (f1 *F1Browser) sendAjax(ajaxURL string, method string, payload string, headers string, timeoutInSecond int) (content string, err error) {
// 	if f1 == nil || f1.Driver == nil {
// 		return "", nil
// 	}
// 	driver := f1.Driver
// 	// logger.Root.Infof("Sending ajax request to: %s\n", ajaxURL)
// 	reqID := utils.GetCurrentTime().UnixNano() //uuid.New().String()

// 	jsGetResult := ""

// 	if method == "POST" {
// 		jsGetResult = fmt.Sprintf(`
// 			if(!window.hasOwnProperty("results")) window["results"]={};
// 			fetch("%s", {
// 				method: "%s",
// 				body: "%s",
// 				headers: %s,
// 				redirect: "follow",
// 				cache: "no-cache"
// 			}).then(response => {
// 				return response.text();
// 			}).then(function (html){
// 				window["results"]["%d"] = html;
// 			});
// 		`, ajaxURL, method, payload, headers, reqID)

// 		// logger.Root.Infof("%#v\n", javascript)
// 	} else {
// 		jsGetResult = fmt.Sprintf(`
// 				if(!window.hasOwnProperty("results")) window["results"]={};
// 				fetch("%s").then(response => {
// 					return response.text();
// 				}).then(function (html){
// 					window["results"]["%d"] = html;
// 				});
// 			`, ajaxURL, reqID)

// 	}
// 	_, err = driver.ExecuteScript(jsGetResult, []interface{}{})
// 	if err != nil {
// 		logger.Root.Errorf("Error: %s\n", err)
// 		return
// 	}
// 	// logger.Root.Infoln(res)

// 	jsGetResult = fmt.Sprintf(`return window["results"]["%d"] ? window["results"]["%d"] : "";`, reqID, reqID)
// 	jsDeleteResult := fmt.Sprintf(`delete window["results"]["%d"];`, reqID)

// 	resultChan := make(chan string)
// 	needToExit := false

// 	go func(needToExit *bool) {
// 		// c := 0
// 		params := []interface{}{}
// 		for {
// 			if *needToExit || f1.needToClose {
// 				break
// 			}
// 			res, err := driver.ExecuteScript(jsGetResult, params)
// 			if err == nil {
// 				if res != nil {
// 					_, err = driver.ExecuteScript(jsDeleteResult, params)
// 					if err != nil {
// 						logger.Root.Errorf("Error when deleting the ajax result. Error: %s\n", err)
// 					}

// 					switch v := res.(type) {
// 					case string:
// 						if len(v) > 0 {
// 							// return v, nil
// 							resultChan <- v
// 							return
// 						}
// 					default:
// 						break
// 					}
// 					// resultChan <- ""
// 					// return
// 				}

// 			} else {
// 				logger.Root.Errorf("Error: %s\n", err)
// 			}
// 			time.Sleep(100 * time.Millisecond)

// 		}
// 	}(&needToExit)

// 	select {
// 	case v := <-resultChan:
// 		return v, nil
// 	case <-time.After(time.Duration(timeoutInSecond) * time.Second):
// 		needToExit = true
// 		return "", errors.New("Timeout")
// 	}

// }

// Close will clean the resources and log out
func (f1 *F1Browser) Close() {
	if f1 == nil {
		return
	}
	f1.needToClose = true
	if len(f1.HomeTab) > 0 {
		tabs, err := getTabs(*f1.DriverInfo.Driver)
		if err == nil && len(tabs) > 0 {
			for _, t := range tabs {
				if f1.HomeTab == t {
					f1.LogOut()
					time.Sleep(3 * time.Second)
				}
			}
		}

	}
	// // tabs, _ := getTabs(*driver.Driver)
	// // if len(tabs) > 0 {
	// // 	LogOut(*driver.Driver, homeTab)
	// // 	time.Sleep(3 * time.Second)
	// // }
	// for f1.IsBusy() {
	// 	time.Sleep(200 * time.Millisecond)
	// }
	// f1.Driver.Close()
	f1.Browser.Close()
}

// LogOut tries to click the Logout button on home page and close that tab
func (f1 *F1Browser) Login() (err error) {
	return errors.New("Not implemented")
}

// LogOut tries to click the Logout button on home page and close that tab
func (f1 *F1Browser) LogOut() (err error) {

	driver := *f1.DriverInfo.Driver
	driver.SwitchWindow(f1.HomeTab)
	time.Sleep(300 * time.Millisecond)
	logoutBtn, err := driver.FindElement(selenium.ByXPATH, "//button[@name='btnEXIT']")
	if err == nil {
		logoutBtn.Click()
		time.Sleep(3 * time.Second)
		logger.Root.Info("Clicked on logout button")
		driver.CloseWindow(f1.HomeTab)
	} else {
		logger.Root.Errorf("Error when finding the logout button. Error: %s\n", err)
		return err
	}
	f1.SetBusy(false)

	return nil
}

func setSystemStatus(status string, key string) {
	logger.Root.Infof("Not implemeented yet")
	// b, _ := json.Marshal(map[string]interface{}{
	// 	"key":    key,
	// 	"status": status,
	// })

	// resultStr, err := utils.SendGetReq(cfg.WebService.URL+API_SYSTEM_STATUS, nil, map[string]string{"x-special-token": cfg.WebService.Token, "Content-Type": "application/json"}, "POST", bytes.NewBuffer(b))

	// var result ApiResult
	// json.Unmarshal([]byte(resultStr), &result)

	// // logger.Root.Infof("Return: %#v\n", result)

	// if err != nil || !result.Ok {
	// 	logger.Root.Errorf("Error when updating status of %s: %s\n", key, err)
	// } else {
	// 	logger.Root.Infof("Updating system status of %s OK!\n", key)
	// }
}
