package my_selenium

import (
	"context"
	"errors"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	"strconv"
	"time"

	"github.com/tebeka/selenium"
)

type F1Crawler struct {
	*F1Browser

	Ctx context.Context
	// Config *Configuration
}

var (
	CRAWLER = "crawler"
	WORKER  = "worker"
)

type CrawlerCallback func(customer.Application)

// // set the status of the crawler: STARTED - STARTING - STOPPING - STOPPED
// // key: cralwer or worker
// func setSystemStatus(status string, key string) {
// 	b, _ := json.Marshal(map[string]interface{}{
// 		"key":    key,
// 		"status": status,
// 	})

// 	resultStr, err := utils.SendGetReq(cfg.WebService.URL+API_SYSTEM_STATUS, nil, map[string]string{"x-special-token": cfg.WebService.Token, "Content-Type": "application/json"}, "POST", bytes.NewBuffer(b))

// 	var result ApiResult
// 	json.Unmarshal([]byte(resultStr), &result)

// 	// logger.Root.Infof("Return: %#v\n", result)

// 	if err != nil || !result.Ok {
// 		logger.Root.Errorf("Error when updating status of %s: %s\n", key, err)
// 	} else {
// 		logger.Root.Infof("Updating system status of %s OK!\n", key)
// 	}
// }

// get the max application No from database
// func getMaxAppNo(database *mongo.Database) (maxLastAppNo int) {
// 	configCollection := database.Collection(ProfileCollectionName)

// 	options := options.Find()
// 	options.SetLimit(1)
// 	options.SetSort(bson.D{{"applicationNo", -1}})
// 	options.SetProjection(bson.M{
// 		"applicationNo": true})

// 	if cur, err := configCollection.Find(context.TODO(), bson.M{}, options); err != nil {
// 		logger.Root.Errorf("Error when getting configuration. Err: %s\n", err)
// 		return 0
// 	} else {
// 		ctx := context.TODO()
// 		var result struct {
// 			number int
// 		}
// 		for cur.Next(ctx) {
// 			logger.Root.Infoln("Decode")
// 			cur.Decode(&result)
// 		}
// 		maxLastAppNo = result.number
// 	}

// 	logger.Root.Infof("Current max AppNo:%d\n", maxLastAppNo)
// 	return
// }

type MaxAppResult struct {
	MaxAppNo int `json: 'maxAppNo'`
}

// get the max application No from database, or from the user set value in the crawler config
// func getMaxAppNo() (maxLastAppNo int) {

// 	content, err := utils.SendGetReq(cfg.WebService.URL+API_MAX_APP_NO, nil, map[string]string{"x-special-token": cfg.WebService.Token}, "GET", nil)
// 	if err == nil {
// 		var result MaxAppResult
// 		json.Unmarshal([]byte(content), &result)
// 		return result.MaxAppNo
// 	}
// 	logger.Root.Errorf("Cannot get max app no. Error: %s\n", err)

// 	logger.Root.Infof("Current max AppNo:%d\n", maxLastAppNo)
// 	return
// }

type ApiResult struct {
	Ok      bool        `json:"ok"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
	Code    int         `json:"code"`
}

// set the current max applition No to the crawler config
// func setMaxAppNo(lastMaxAppNo int) {
// 	// filter := bson.M{
// 	// 	"key": bson.M{
// 	// 		"$eq": "crawler",
// 	// 	},
// 	// }

// 	// update := bson.M{
// 	// 	"$set": bson.M{
// 	// 		"value.maxLastAppNo": lastMaxAppNo,
// 	// 	},
// 	// }

// 	// _, err := database.Collection(ConfigurationCollectionName).UpdateOne(
// 	// 	context.Background(),
// 	// 	filter, update,
// 	// )

// 	var jsonStr = []byte(fmt.Sprintf(`{ "maxAppNo" : %d }`, lastMaxAppNo))

// 	resultStr, err := utils.SendGetReq(cfg.WebService.URL+API_MAX_APP_NO, nil, map[string]string{"x-special-token": cfg.WebService.Token, "Content-Type": "application/json"}, "POST", bytes.NewBuffer(jsonStr))

// 	var result ApiResult
// 	json.Unmarshal([]byte(resultStr), &result)

// 	// logger.Root.Infof("Return: %#v\n", result)

// 	if err != nil || !result.Ok {
// 		logger.Root.Errorf("Error when updating maxLastAppNo: %s. Code=%d\n", err, result.Code)
// 	} else {
// 		logger.Root.Errorf("Updating maxLastAppNo OK!\n")
// 	}
// }

// get the crawler configuration such as start hour, stop hour, maxLastAppNo, minAmountFinanced
// func getCrawlerConfiguration() (configuration *CrawlerConfiguration, err error) {
// 	resultStr, err := utils.SendGetReq(cfg.WebService.URL+API_SYSTEM_CRAWLER_STATUS, nil, map[string]string{"x-special-token": cfg.WebService.Token, "Content-Type": "application/json"}, "GET", nil)

// 	var result ApiResult
// 	json.Unmarshal([]byte(resultStr), &result)

// 	config := CrawlerConfiguration{
// 		Key: "crawler",
// 	}

// 	if err != nil || !result.Ok || result.Data == nil {

// 		logger.Root.Errorf("Error when getting worker configuration %s\n", err)
// 	} else {

// 		if result.Data != nil {
// 			tmp := result.Data.(map[string]interface{})
// 			switch v := tmp["value"].(type) {
// 			case map[string]interface{}:
// 				config.Value.StartHour = int(v["startHour"].(float64))
// 				config.Value.StopHour = int(v["stopHour"].(float64))
// 				config.Value.AutoRun = v["autoRun"].(bool)
// 				config.Value.MaxLastAppNo = int(v["maxLastAppNo"].(float64))
// 				config.Value.MinAmountFinanced = int(v["minAmountFinanced"].(float64))
// 				config.Value.StatusRequested = v["statusRequested"].(string)
// 			}

// 		}
// 		// logger.Root.Errorf("Getting worker configuration OK!\n")
// 	}

// 	// logger.Root.Infof("Current crawler config: %v\n", config)
// 	return &config, nil
// }

// // get the worker configuration such as start hour, stop hour, maxLastAppNo, minAmountFinanced
// func getWorkerConfiguration() (configuration *WorkerConfiguration, err error) {

// 	resultStr, err := utils.SendGetReq(cfg.WebService.URL+API_SYSTEM_WORKER_STATUS, nil, map[string]string{"x-special-token": cfg.WebService.Token, "Content-Type": "application/json"}, "GET", nil)

// 	var result ApiResult
// 	json.Unmarshal([]byte(resultStr), &result)

// 	config := WorkerConfiguration{
// 		Key: "worker",
// 	}

// 	if err != nil || !result.Ok || result.Data == nil {

// 		logger.Root.Errorf("Error when getting worker configuration %s\n", err)
// 	} else {
// 		if result.Data != nil {
// 			tmp := result.Data.(map[string]interface{})
// 			switch v := tmp["value"].(type) {
// 			case map[string]interface{}:
// 				config.Value.StartHour = int(v["startHour"].(float64))
// 				config.Value.StopHour = int(v["stopHour"].(float64))
// 				config.Value.AutoRun = v["autoRun"].(bool)
// 				config.Value.NumWorkers = int(v["numWorkers"].(float64))
// 				config.Value.InvalidateDuration = int64(v["invalidateDuration"].(float64))
// 				config.Value.StatusRequested = v["statusRequested"].(string)
// 			}

// 		}
// 		// logger.Root.Errorf("Getting worker configuration OK!\n")
// 	}

// 	// logger.Root.Infof("Current worker config: %v\n", config)
// 	return &config, nil
// }

// InitCrawler initials the crawler the first time by clicking the search button
func (f1 *F1Crawler) InitCrawler() (err error) {
	logger.Root.Infof("Initializing crawler....\n")
	// f1.Driver.Refresh()
	time.Sleep(5 * time.Second)
	d := *(f1.DriverInfo.Driver)

	// try to select the product as PERSONAL
	elmSelProduct, err := d.FindElement(selenium.ByXPATH, "//select[@name='selProduct']/option[@value='PERSONAL']")
	if err != nil {
		logger.Root.Errorf("Error when searching element selProduct. Error: %s\n", err)
		return err
	}

	d.ExecuteScript(`arguments[0].setAttribute("selected", "selected")`, []interface{}{elmSelProduct})

	// to to select the Activity is BDE
	elmSelActivityID, err := d.FindElement(selenium.ByXPATH, "//select[@id='selActivityId']/option[@value='BDE']")
	if err != nil {
		logger.Root.Errorf("Error when searching element selSelActivityID. Error: %s\n", err)
		return err
	}
	d.ExecuteScript(`arguments[0].setAttribute("selected", "selected")`, []interface{}{elmSelActivityID})

	// click the Search button
	btnSearch, err := d.FindElement(selenium.ByXPATH, "//input[@name='btnSearch']")
	if err != nil {
		logger.Root.Errorf("Error when searching element btnSearch. Error: %s\n", err)
		return err
	}

	btnSearch.MoveTo(0, 0)
	btnSearch.Click()

	time.Sleep(5 * time.Second)

	return nil

}

// // checkStopRequest reads the request from server to see if we should stop the crawler or not
// // if actionNow = true, we mark the crawler to be closed, and set busy to false
// func (f1 *F1Crawler) checkStopRequestAndClose(actionNow bool) (stopCrawler bool) {
// 	// config, err := getCrawlerConfiguration(f1.DB)
// 	// if err != nil {
// 	// 	return true
// 	// }
// 	// return config.Value.StatusRequested == Stopping

// 	logger.Root.Infof("Current status: %s\n", configCrawler.Value.StatusRequested)
// 	if configCrawler.Value.StatusRequested == Stopping || configCrawler.Value.StatusRequested == Stopped {
// 		if actionNow {
// 			logger.Root.Infoln("Close crawler")

// 			f1.needToClose = true
// 			f1.SetBusy(false)
// 		}
// 		return true
// 	}
// 	return false
// }

const (
	Starting = "STARTING"
	Running  = "RUNNING"
	Stopping = "STOPPING"
	Stopped  = "STOPPED"
)

func (f1 *F1Crawler) updateTheCrawlerStatus(status string) {
	// connect to server and upate the status
}

func (f1 *F1Crawler) tryToGetTheLatestPage() (totalPage int64, err error) {
	var totalPageOpt selenium.WebElement
	d := *(f1.DriverInfo.Driver)

	retries := 0
	for {
		totalPageOpt, err = d.FindElement(selenium.ByXPATH, "//select[@id='selPageIndex']//option[last()]")
		if err != nil {
			logger.Root.Errorf("Error when getting the select box of pages: %s. Will refresh in 10 seconds...\n", err)
			retries++
			if retries > 100 {
				return 0, err
			}
			logger.Root.Infof("Reloading page (%d)....\n", retries)
			d.Refresh()
			time.Sleep(10 * time.Second)
		} else {
			retries = 0
			break
		}
	}

	totalPageStr, err := totalPageOpt.GetAttribute("value")
	if err != nil || totalPageStr == "" {
		logger.Root.Errorf("Error when getting total page: %s\n", err)
		return 0, err
	}
	totalPage, err = strconv.ParseInt(totalPageStr, 10, 64)
	return
}

// Crawl extracts information from the last page to the first page
func (f1 *F1Crawler) Crawl() (err error) {
	// 	if f1 == nil {
	// 		return errors.New("Closed crawler by user requested")
	// 	}
	// 	if f1.checkStopRequestAndClose(true) {
	// 		return nil
	// 	}

	// 	go func() {
	// 		for {
	// 			if f1 == nil {
	// 				// this browser is closed
	// 				break
	// 			}
	// 			// setSystemStatus(Running, CRAWLER)
	// 			if err = _crawl(f1); err == nil {
	// 				logger.Root.Infof("Stopped the crawler\n")
	// 				break
	// 			}
	// 			logger.Root.Infof("Refreshing the crawler...\n")
	// 			SetupF1Browser(f1.F1Browser, false, nil)
	// 		}
	// 		if f1 != nil {
	// 			// setSystemStatus(Stopped, CRAWLER)
	// 		}
	// 	}()
	return errors.New("Craw() is not implemented yet")
}

func checkIfNeedToStopThenStop() {

}

// func _crawl(f1 *F1Crawler) (err error) {
// 	logger.Root.Infof("Start crawling.....\n")
// 	// maxLastAppNo := f1.Config.Value.MaxLastAppNo
// 	// minAmountFinanced := float64(f1.Config.Value.MinAmountFinanced)

// 	f1.InitCrawler()

// 	defer func() {
// 		f1.Close()
// 		// f1.updateTheCrawlerStatus(Stopped)
// 		// setSystemStatus(Stopped, CRAWLER)
// 	}()
// 	f1.Token, _ = f1.GetToken()

// 	f1.Driver.SetPageLoadTimeout(120 * time.Second)
// 	f1.Driver.SetImplicitWaitTimeout(120 * time.Second)

// 	totalPage, err := f1.tryToGetTheLatestPage()
// 	if err != nil {
// 		return err
// 	}

// 	page := totalPage

// 	logger.Root.Infof("Total page: %d Page %d\n", totalPage, page)
// 	hasError := false
// 	reachMaxAppNo := false

// 	crawlerConfig, _ := getCrawlerConfiguration()
// 	if crawlerConfig == nil {
// 		logger.Root.Errorf("Cannot get the configuration from db.\n")
// 		return
// 	}

// 	logger.Root.Infof("Crawling with maxLastAppNo: %d minFinancedAmount: %d\n", crawlerConfig.Value.MaxLastAppNo, crawlerConfig.Value.MinAmountFinanced)

// 	for {
// 		if lastConfig, errC := getCrawlerConfiguration(); lastConfig != nil && errC == nil {
// 			crawlerConfig = lastConfig
// 		}

// 		stopRequested := f1.checkStopRequestAndClose(false)

// 		logger.Root.Infof("Page=%d hasError:%v ReachMaxAppNo:%v\n", page, hasError, reachMaxAppNo)
// 		// if we extract all of the page, we now go to  the last page
// 		// note that F1 adds the profiles all of the day
// 		// then the profiles on the last page are the latest ones
// 		if page <= 0 || hasError || reachMaxAppNo || stopRequested {

// 			// get the max app number from profiles in DB, and update that value to the crawler configuration
// 			if latestMaxAppNo := getMaxAppNo(); latestMaxAppNo > 0 {
// 				crawlerConfig.Value.MaxLastAppNo = latestMaxAppNo
// 				// update the max app No to the crawler config
// 				setMaxAppNo(latestMaxAppNo)
// 				logger.Root.Infof("Updated the lastMappNo to %d\n", latestMaxAppNo)
// 			}

// 			if crawlerConfig.Value.StatusRequested == Stopping {
// 				logger.Root.Infof("Stopping crawler by user request or by scheduled....")
// 				break
// 			}

// 			logger.Root.Infof("Going to the latest page....")
// 			page, err = f1.similateClickingLastestPage()
// 			if err != nil {
// 				return err
// 			}

// 			logger.Root.Infof("Crawling with maxLastAppNo: %d minFinancedAmount: %d\n", crawlerConfig.Value.MaxLastAppNo, crawlerConfig.Value.MinAmountFinanced)
// 		}

// 		if stopRequested {
// 			logger.Root.Infof("Stop the crawler\n")
// 			break
// 		}

// 		_, reachMaxAppNo, err = f1.crawlPage(crawlerConfig.Value.MaxLastAppNo, int(page), int(totalPage), float64(crawlerConfig.Value.MinAmountFinanced))

// 		page--
// 	}

// 	// f1.Close()
// 	// f1.updateTheCrawlerStatus(Stopped)
// 	return nil
// }

// func (f1 *F1Crawler) updateMaxLastAppNo(maxLastAppNo int) {
// 	f1.DB.Collection(ProfileCollectionName)
// }

func (f1 *F1Browser) similateClickingLastestPage() (page int64, err error) {
	retries := 0
	var selectboxElm selenium.WebElement
	var optionElms []selenium.WebElement
	d := *(f1.DriverInfo.Driver)

	for {
		selectboxElm, err = d.FindElement(selenium.ByID, "selPageIndex")
		if err != nil {
			if retries < 100 {
				retries++
				logger.Root.Infof("Reloading page (%d)....\n", retries)
				continue
			}
			return 0, err
		}

		optionElms, err = d.FindElements(selenium.ByXPATH, "//*[@id='selPageIndex']//option")
		if err != nil {
			if retries < 100 {
				retries++
				logger.Root.Infof("Reloading page (%d)....\n", retries)
				continue
			}
			return 0, err
		}
		break
	}

	page = int64(len(optionElms) - 0)
	logger.Root.Infof("Going to page %d after 20 seconds...\n", page)

	js := `var event = new Event('change'); var t = 20; setTimeout(() => {
                var option = arguments[0].options[arguments[1]];
                option.setAttribute("selected", "selected");
                arguments[0].dispatchEvent(event);
			}, t*1000);`
	_, err = d.ExecuteScript(js, []interface{}{selectboxElm, page})
	time.Sleep(30 * time.Second)
	return
}

var processedAppId = map[int]bool{}

func addProcessAppID(id int) {
	processedAppId[id] = true
}

func hasProcessedAppID(id int) bool {
	if _, ok := processedAppId[id]; ok {
		return true
	}
	return false
}

// checkAndCloseOnUserRequest checks if we need to close worker or not
// if yes, set needToClose=true, and return true
// func (f1 *F1Crawler) checkAndCloseOnUserRequest() bool {
// 	if configCrawler.Value.StatusRequested == Stopping || configCrawler.Value.StatusRequested == Stopped {
// 		logger.Root.Infoln("Close crawler by user")

// 		f1.needToClose = true
// 		f1.SetBusy(false)
// 		return true
// 	}
// 	return false
// }

// crawlPage gets all profile of the current the day
// if CMND is empty, it will return the latest profiles on F1
// func (f1 *F1Crawler) crawlPage(maxLastAppNo int, page int, totalPage int, minAmountFinanced float64) (results []F1Profile, reachMaxAppNo bool, err error) {
// 	f1.SetBusy(true)
// 	defer f1.SetBusy(false)

// 	// driver := f1.Driver

// 	payload := fmt.Sprintf(`org.apache.struts.taglib.html.TOKEN=%s&hidProcessFlag=S&applicationid=&customer=&selProduct=PERSONAL&selScheme=&selSchemeDesc=&txtNicNo=&txtIdCardNo=&txtCIFNo=&signed=&signedTo=&txtContractNo=&selActivityId=BDE&selPageIndex=%d&hidPageAction=&hidCurrentPage=%d&hidTotalPage=%d&hidCategory=&pageNo=%d&hidModifyFlag=T&productId=&allBranch=`, f1.Token, page, page, totalPage, page)

// 	retries := 0
// 	var content string

// 	// we use the for loop to re-try in case of error
// 	for {
// 		content, err = f1.sendAjax(urlTracking, "POST", payload,
// 			`{"Content-Type": "application/x-www-form-urlencoded"}`, 60)
// 		if err != nil {
// 			if retries < 3 {
// 				retries++
// 				// time.Sleep(100 * time.Millisecond)
// 				continue
// 			} else {
// 				logger.Root.Errorf("Error when sending post ajax: %s\n", err)
// 				return
// 			}
// 		}

// 		// if !strings.Contains(content, "</html>") {
// 		content = "<html>" + content + "</html>"
// 		// }

// 		doc, err1 := htmlquery.Parse(strings.NewReader(content))
// 		if err1 != nil {
// 			log.Printf("Error when parsing xml: %s\n", err1)
// 			retries++
// 			time.Sleep(1000 * time.Millisecond)
// 			continue
// 		}

// 		results, reachMaxAppNo, err = f1.extractAllProfilesOnPage(&maxLastAppNo, minAmountFinanced, doc, false)
// 		if err != nil {
// 			if retries < 3 {
// 				retries++
// 				// time.Sleep(100 * time.Millisecond)
// 				continue
// 			} else {
// 				logger.Root.Errorf("Error when extracting profiles on page: %s\n", err)
// 				return
// 			}
// 		}

// 		break
// 	}

// 	return results, reachMaxAppNo, nil
// }

// func (f1 *F1Crawler) Close() {
// 	if f1 == nil {
// 		return
// 	}
// 	if f1.F1Browser != nil {
// 		f1.F1Browser.Close()
// 	}
// 	setSystemStatus(Stopped, CRAWLER)
// }
