package my_selenium

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const AjaxTimeoutDefault = 30 // second

var ErrTimeout = errors.New("Timeout")

var (
	TypeWorker  = "worker"
	TypeCrawler = "crawler"
)

// Browser defines the structure of web browser
type Browser struct {
	sync.Mutex

	DriverInfo DriverInfo

	// can we use this browser window
	isActive bool

	// the max
	MaxAge int64
	// within this time, we have to do an action on the page to keep the session
	MaxAllowedIdleTIme int64

	// is working or not
	isBusy bool

	IsInitializing bool

	needToClose bool

	LastLoginTime int64

	Type string
}

func CreateBrowser(withDocker bool, maxAge int64, maxIdleTime int64, args map[string]interface{}) *Browser {
	logger.Root.Infof("Create selenium driver with args: %+v\n", args)
	driver, err := GetSeleniumService(withDocker, "", args)
	if err != nil {
		logger.Root.Infof("Could  not create selenium service\n")
		return nil
	}

	b := Browser{
		isActive:           false,
		isBusy:             false,
		IsInitializing:     true,
		needToClose:        false,
		MaxAge:             maxAge,
		MaxAllowedIdleTIme: maxIdleTime,
		DriverInfo:         driver,
	}

	return &b
}

// NeedToBeReplaced returns true if this browser session needs to be replaced by another session
func (f1 *Browser) NeedToBeReplaced() bool {
	if f1 == nil {
		return false
	}
	now := utils.GetCurrentTime().Unix()
	if now-f1.LastLoginTime >= f1.MaxAge-int64(rand.Intn(3*60)) && f1.MaxAge > 0 {
		return true
	}
	return false
}

// NeedToBeReset returns true if we need to type and hit Enter into search text box
func (f1 *Browser) NeedToBeReset() bool {
	if f1 == nil {
		return false
	}
	now := utils.GetCurrentTime().Unix()
	if now-f1.LastLoginTime >= int64(f1.MaxAllowedIdleTIme)-int64(rand.Intn(int(float64(f1.MaxAllowedIdleTIme)*0.1))) && f1.MaxAllowedIdleTIme > 0 {
		return true
	}
	return false
}

// SetActive sets the active value for this browser
func (f1 *Browser) SetActive(a bool) {
	if f1 == nil {
		return
	}
	f1.Lock()
	f1.isActive = a
	f1.Unlock()
}

// IsActive returns if this browser is active or not
func (f1 *Browser) IsActive() bool {
	if f1 == nil {
		return false
	}
	f1.Lock()
	defer f1.Unlock()
	return f1.isActive
}

// SetBusy sets the active value for this browser
func (f1 *Browser) SetBusy(a bool) {
	if f1 == nil {
		return
	}
	f1.Lock()
	f1.isBusy = a
	f1.Unlock()
}

// IsBusy returns if this browser is active or not
func (f1 *Browser) IsBusy() bool {
	if f1 == nil {
		return false
	}
	f1.Lock()
	defer f1.Unlock()
	return f1.isBusy
}

// checkStopRequest reads the request from server to see if we should stop the crawler or not
// if actionNow = true, we mark the crawler to be closed, and set busy to false
func (f1 *Browser) checkStopRequestAndClose(actionNow bool) (stopCrawler bool) {
	// 	if f1 == nil {
	// 		return true
	// 	}
	// 	configCrawler, _ = getCrawlerConfiguration()
	// 	if configCrawler == nil {
	// 		return false
	// 	}
	// 	logger.Root.Infof("Current status: %s\n", configCrawler.Value.StatusRequested)
	// 	needToClose := false
	// 	if f1 == nil {
	// 		return true
	// 	}
	// 	if f1.Type == TypeCrawler {
	// 		needToClose = configCrawler.Value.StatusRequested == Stopping || configCrawler.Value.StatusRequested == Stopped
	// 	} else {
	// 		needToClose = configWorker.Value.StatusRequested == Stopping || configWorker.Value.StatusRequested == Stopped
	// 	}

	// 	if needToClose {
	// 		if actionNow {
	// 			if f1.Type == TypeCrawler {
	// 				logger.Root.Infoln("Close crawler")
	// 			} else {
	// 				logger.Root.Infoln("Close worker")
	// 			}

	// 			f1.needToClose = true
	// 			f1.SetBusy(false)
	// 		}
	// 		return true
	// 	}
	return false
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

// SendAjax sends an ajax request and wait to get the response
func (f1 *Browser) SendAjax(ajaxURL string, method string, payload string, headers string, timeoutInSecond int, onResult func(string, error)) (content string, err error) {
	if f1 == nil || f1.DriverInfo.Driver == nil {
		return "", nil
	}
	driver := *(f1.DriverInfo.Driver)
	// logger.Root.Infof("Sending ajax request to: %s\n", ajaxURL)
	reqID := utils.GetCurrentTime().UnixNano() //uuid.New().String()

	jsGetResult := ""

	if method == "POST" {
		if headers != "" {
			headers = "," + headers
		}
		jsGetResult = fmt.Sprintf(`
			if(!window.hasOwnProperty("results")) window["results"]={}; 
			fetch("%s"%s).then(response => { 
				return response.text();
			}).then(function (html){ 
				window["results"]["%d"] = html;
			});
		`, ajaxURL, headers, reqID)

		// logger.Root.Infof("%#v\n", javascript)
	} else {
		// headers now become options
		if headers != "" {
			headers = "," + headers
		}
		jsGetResult = fmt.Sprintf(`
				if(!window.hasOwnProperty("results")) window["results"]={}; 
				fetch("%s"%s).then(response => { 
					return response.text();
				}).then(function (html){
					window["results"]["%d"] = html;
				}).catch((err) => {
					window["results"]["%d"] = "error:" + err;
				});
			`, ajaxURL, headers, reqID, reqID)
		// logger.Root.Infof("js: `%s`", jsGetResult)

	}
	_, err = driver.ExecuteScript(jsGetResult, []interface{}{})
	if err != nil {
		logger.Root.Errorf("Error: %s\n", err)
		return
	}
	// logger.Root.Infoln(res)

	jsGetResult = fmt.Sprintf(`return window["results"]["%d"] ? window["results"]["%d"] : "";`, reqID, reqID)
	jsDeleteResult := fmt.Sprintf(`delete window["results"]["%d"];`, reqID)

	resultChan := make(chan string)
	needToExit := false

	go func(needToExit *bool) {
		// c := 0
		errorCount := 0
		params := []interface{}{}
		for {
			if *needToExit || f1.needToClose {
				break
			}
			res, err := driver.ExecuteScript(jsGetResult, params)
			if err == nil {
				if res != nil {
					_, err = driver.ExecuteScript(jsDeleteResult, params)
					if err != nil {
						logger.Root.Errorf("Error when deleting the ajax result. Error: %s\n", err)
					}

					switch v := res.(type) {
					case string:
						if len(v) > 0 {
							// return v, nil
							resultChan <- v
							return
						}
					default:
						resultChan <- ""
						break
					}
					// return
				}

			} else {
				logger.Root.Errorf("Error: %s\n", err)
				errorCount++
				if errorCount >= 100 {
					resultChan <- ""
					break
				}
			}
			time.Sleep(100 * time.Millisecond)

		}
	}(&needToExit)

	select {
	case v := <-resultChan:
		if strings.HasPrefix(v, "error:") {
			onResult("", errors.New("Could not fetch data. "+v))
			return "", errors.New("Could not fetch data. " + v)
		}
		onResult(v, nil)
		return v, nil
	case <-time.After(time.Duration(timeoutInSecond) * time.Second):
		needToExit = true
		onResult("", ErrTimeout)
		return "", ErrTimeout
	}

}

// SendAjax sends an ajax request and wait to get the response
func (f1 *Browser) SendAjaxAndGetNewURL(ajaxURL string, headers string, timeoutInSecond int) (content string, url string, err error) {
	if f1 == nil || f1.DriverInfo.Driver == nil {
		return "", "", nil
	}
	driver := *(f1.DriverInfo.Driver)
	// logger.Root.Infof("Sending ajax request to: %s\n", ajaxURL)
	reqID := utils.GetCurrentTime().UnixNano() //uuid.New().String()

	jsGetResult := ""
	if headers != "" {
		headers = "," + headers
	}
	jsGetResult = fmt.Sprintf(`
			if(!window.hasOwnProperty("results")) window["results"]={}; 
			fetch("%s"%s).then(response => { 
				window["results"]["%d-url"] = response.url || "";
				console.log(response.status);
				console.log(response.statusText);
				console.log(response.type);
				console.log(response.url);
				return response.text();
			}).then(function (html){ 
				window["results"]["%d"] = html;
			}).catch((err) => {
				window["results"]["%d"] = err;
			});
		`, ajaxURL, headers, reqID, reqID, reqID)

	_, err = driver.ExecuteScript(jsGetResult, []interface{}{})
	if err != nil {
		logger.Root.Errorf("Error: %s\n", err)
		return
	}
	// logger.Root.Infoln(res)

	jsGetResult = fmt.Sprintf(`return { content: window["results"]["%d"] || "", url:  window["results"]["%d-url"] || "" } ;`, reqID, reqID)
	jsDeleteResult := fmt.Sprintf(`delete window["results"]["%d"]; delete window["results"]["%d-url"]`, reqID, reqID)

	resultChan := make(chan map[string]interface{})
	needToExit := false

	go func(needToExit *bool) {
		// c := 0
		errorCount := 0
		params := []interface{}{}
		for {
			if *needToExit || f1.needToClose {
				break
			}
			res, err := driver.ExecuteScript(jsGetResult, params)
			if err == nil {
				if res != nil {
					_, err = driver.ExecuteScript(jsDeleteResult, params)
					if err != nil {
						logger.Root.Errorf("Error when deleting the ajax result. Error: %s\n", err)
					}

					// logger.Root.Infof("res: %#v", res)

					switch v := res.(type) {
					case map[string]string:
						// return v, nil
						if v["content"] != "" {
							resultChan <- map[string]interface{}{"content": v["content"], "url": v["url"]}
							return
						}
					case map[string]interface{}:
						// return v, nil
						if content := v["content"]; content != nil {
							if contentInStr, ok := content.(string); ok && len(contentInStr) > 0 {
								resultChan <- v
								return
							}
						}

					default:
						resultChan <- nil
						break
					}
					// return
				}

			} else {
				logger.Root.Errorf("Error: %s\n", err)
				errorCount++
				if errorCount >= 100 {
					resultChan <- nil
					break
				}
			}
			time.Sleep(100 * time.Millisecond)

		}
	}(&needToExit)

	select {
	case v := <-resultChan:
		if v != nil {
			return v["content"].(string), v["url"].(string), nil
		}

		return "", "", nil
	case <-time.After(time.Duration(timeoutInSecond) * time.Second):
		needToExit = true
		return "", "", ErrTimeout
	}

}

// SendAjaxWithAwait sends an ajax request and wait to get the response
// THIS IS A USELESS FUNCTION!!!!!!
func (f1 *Browser) SendAjaxWithAwait(ajaxURL string, options string) (content string, url string, err error) {
	if f1 == nil || f1.DriverInfo.Driver == nil {
		return
	}
	driver := *(f1.DriverInfo.Driver)
	// logger.Root.Infof("Sending ajax request to: %s\n", ajaxURL)
	// reqID := utils.GetCurrentTime().UnixNano() //uuid.New().String()

	jsGetResult := ""
	if options != "" {
		options = "," + options
	}

	jsGetResult = fmt.Sprintf(`
					const resp = await fetch("%s"%s);
					return { content: await resp.text(), url: resp.url };
		`, ajaxURL, options)
	// jsGetResult = fmt.Sprintf(`
	// 			(async () => {
	// 				const resp = await fetch("%s"%s);
	// 				return { content: await resp.text(), url: resp.url };
	// 			})();

	// 	`, ajaxURL, options)
	logger.Root.Infof("send ajax: `%s`", jsGetResult)

	result, err := driver.ExecuteScriptAsync(jsGetResult, []interface{}{})
	if err != nil {
		logger.Root.Errorf("Error: %s\n", err)
		return
	}
	type ajaxResult struct {
		content string
		url     string
	}
	ajaxResultObj := ajaxResult{}
	if result != nil {
		resultInStr := result.(string)
		err = json.Unmarshal([]byte(resultInStr), &ajaxResultObj)
		if err != nil {
			return "", "", err
		}
		return ajaxResultObj.content, ajaxResultObj.url, nil
	}
	// logger.Root.Infoln(res)
	return "", "", nil

}

// Close will clean the resources and log out
func (f1 *Browser) Close() {
	if f1 == nil {
		return
	}
	f1.needToClose = true

	// tabs, _ := getTabs(*driver.Driver)
	// if len(tabs) > 0 {
	// 	LogOut(*driver.Driver, homeTab)
	// 	time.Sleep(3 * time.Second)
	// }
	for f1.IsBusy() {
		time.Sleep(200 * time.Millisecond)
	}
	logger.Root.Infof("Close the browser")
	f1.DriverInfo.Close()
}
