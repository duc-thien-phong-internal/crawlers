package cicblib

//
//import (
//	"encoding/json"
//	"errors"
//	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
//	"github.com/duc-thien-phong/techsharedservices/commands"
//	"github.com/duc-thien-phong/techsharedservices/models"
//	"github.com/duc-thien-phong/techsharedservices/models/customer"
//	nsqmodels "github.com/duc-thien-phong/techsharedservices/nsq/models"
//	"github.com/duc-thien-phong/techsharedservices/utils"
//	"github.com/golang/glog"
//	"math/rand"
//	"net/http/cookiejar"
//	"sync"
//	"time"
//)
//
//type Adapter struct {
//	*basecrawler.Application
//
//	sync.Mutex
//	accountsExtractingData []*basecrawler.UsefulAccountInfo
//	accountsCheckingData   []*basecrawler.UsefulAccountInfo
//	//
//	producer    *Crawler
//	crawlerList map[string]*Crawler
//	checkerList map[string]*Checker
//	//
//	needToChecked chan metaCheckingAppRequest
//	checked       chan metaCheckingAppResult
//	//
//	needToCrawled chan metaCrawlingAppRequest
//	crawled       chan metaCrawlingAppResult
//	//
//	needToCloseCrawlers bool
//	needToCloseCheckers bool
//	//
//	startedCrawlerWatcher bool
//	startedCheckerWatcher bool
//	//
//	numNeedToCrawl int
//	numNeedToCheck int
//	//
//	wg sync.WaitGroup
//}
//
//type metaCheckingAppRequest struct {
//	// can be workID (FE) or AppNo(MR) or DataID
//	dataID string
//	// need to re-fetch all information of the customer application ?
//	checkFull bool
//	// id of the command
//	commandID commands.CommandID
//	// subtype of the command
//	commandSubType commands.CommandSubType
//	// channel of the command
//	channel string
//}
//
//type metaCheckingAppResult struct {
//	request metaCheckingAppRequest
//	// result
//	app            *customer.Application
//	commandID      commands.CommandID
//	commandSubType commands.CommandSubType
//	channel        string
//}
//
//type metaCrawlingAppRequest struct {
//	cicCode string
//}
//
//type metaCrawlingAppResult struct {
//	request metaCrawlingAppRequest
//	app     *customer.Application
//}
//
//// just a way to verify if Adapter implements all methods of AdapterInterface or not
//var test basecrawler.AdapterInterface = &Adapter{}
//
//func (Adapter) GetWorkerSoftwareSource() customer.SoftwareSource {
//	return customer.SoftwareCIB
//}
//
//func (c *Adapter) Init() {
//	c.crawlerList = map[string]*Crawler{}
//	c.checkerList = map[string]*Checker{}
//
//	c.needToChecked = make(chan metaCheckingAppRequest, 1000)
//	c.checked = make(chan metaCheckingAppResult, 1000)
//	c.needToCrawled = make(chan metaCrawlingAppRequest, 1000)
//	c.crawled = make(chan metaCrawlingAppResult, 1000)
//}
//
//func (c *Adapter) Crawl(config models.DataCrawlerConfig) ([]customer.Application, error) {
//	panic("implement me")
//}
//
//func (c *Adapter) CreateCrawler(withDocker bool, args map[string]interface{}) (basecrawler.CrawlerInterface, error) {
//	crawler := NewCrawler(withDocker)
//	if withDocker && (crawler == nil || crawler.Browser == nil) {
//		return nil, errors.New("Could not create browser with docker")
//	}
//
//	crawler.adapter = c
//	crawler.allAvailableAccounts = shuffle(c.accountsExtractingData)
//
//	err := crawler.Login()
//	if err != nil {
//		logger.Root.Errorf("Error when login: %v", err)
//		crawler.Close()
//		return nil, err
//	}
//	//err = utils.RetryOperation(func() error {
//	//	return crawler.GoToReportPage()
//	//}, 3, 3)
//	//if err != nil {
//	//	logger.Root.Errorf("Error when go to Report: %v", err)
//	//	crawler.Close()
//	//	return nil, err
//	//}
//	logger.Root.Infof("New crawler created")
//
//	return crawler, nil
//}
//
//// GetACrawler returns a random created crawler
//func (p *Adapter) GetACrawler() basecrawler.CrawlerInterface {
//	retries := 0
//
//	var newC basecrawler.CrawlerInterface = nil
//	var err error
//	for newC == nil && !p.needToCloseCrawlers {
//		newC, err = p.CreateCrawler(p.withDocker(), nil)
//		if err == basecrawler.ErrNoUsableAccount {
//			logger.Root.Errorf("There is no usable account")
//			return nil
//		}
//		retries++
//		time.Sleep(time.Duration(retries) * time.Second)
//	}
//	return newC
//}
//
//func (p *Adapter) removeCrawler(id string) {
//	if c, ok := p.crawlerList[id]; ok {
//		logger.Root.Infof("Deleting worker `%s` from the crawler list", id)
//		p.Lock()
//		delete(p.crawlerList, id)
//		p.Unlock()
//
//		// if the deleted crawler is also the producer
//		// pick a random crawler and assign it to the role producer
//		if p.producer != nil && c.id == p.producer.id {
//			p.assignNewProducer()
//		}
//	}
//}
//
//func (c *Adapter) CreateChecker(withDocker bool, args map[string]interface{}) (basecrawler.CheckerInterface, error) {
//	checker := NewChecker(withDocker)
//	if checker == nil {
//		return nil, errors.New("Could not create browser")
//	}
//
//	if withDocker && (checker == nil || checker.Browser == nil) {
//		return nil, errors.New("Could not create browser with docker")
//	}
//
//	checker.adapter = c
//	checker.allAvailableAccounts = shuffle(c.accountsCheckingData)
//
//	err := checker.Login()
//	if err != nil {
//		logger.Root.Errorf("Error when login: %v", err)
//		checker.Close()
//		return nil, err
//	}
//	//err = utils.RetryOperation(func() error {
//	//	return checker.GoToReportPage()
//	//}, 3, 3)
//	//if err != nil {
//	//	logger.Root.Errorf("Error when go to Report: %v", err)
//	//	checker.Close()
//	//	return nil, err
//	//}
//
//	logger.Root.Infof("New checker created")
//
//	return checker, nil
//}
//
//// GetAChecker returns a new random created checker
//func (p *Adapter) GetAChecker() basecrawler.CheckerInterface {
//	// No need to use it for now
//	return nil
//}
//
//func (p *Adapter) removeChecker(id string) {
//	if _, ok := p.checkerList[id]; ok {
//		logger.Root.Infof("Deleting worker `%s` from the checker list", id)
//		p.Lock()
//		delete(p.checkerList, id)
//		p.Unlock()
//	}
//}
//
//func (p *Adapter) GetNumRunningCrawlers() int {
//	p.Lock()
//	defer p.Unlock()
//	return len(p.crawlerList)
//}
//
//func (p *Adapter) GetNumRunningCheckers() int {
//	p.Lock()
//	defer p.Unlock()
//	return len(p.checkerList)
//}
//
//func (c *Adapter) withDocker() bool {
//	if d, ok := c.GetWorkerConfig().OtherConfig["withDocker"]; ok {
//		logger.Root.Infof("Get value of `withDocker` as %v (%T)", d, d)
//		if v, ok := d.(bool); ok {
//			logger.Root.Infof("useDocker: %v", v)
//			return v
//		}
//	}
//	return false
//}
//
//func (c *Adapter) startCheckerMonitor(wg *sync.WaitGroup) {
//	c.startedCheckerWatcher = true
//	logger.Root.Infof("Starting checker watcher....")
//	for {
//		if c.CanStartChecker() && len(c.checkerList) < c.getCheckerParallelism() && !c.needToCloseCheckers {
//			c.createAndAddCheckerToList(wg)
//		} else {
//			if time.Now().Unix()%5 == 0 {
//				logger.Root.Infof("Number of checkers: %d....", len(c.checkerList))
//			}
//			time.Sleep(15 * time.Second)
//		}
//	}
//}
//
//func (c *Adapter) pickRandomColorName() string {
//	for {
//		name := basecrawler.ColorNames[rand.Intn(len(basecrawler.ColorNames))]
//		if _, ok := c.crawlerList[name]; !ok {
//			return name
//		}
//	}
//	// return colorNames[0] + " " + fmt.Sprint(time.Now().Unix())
//}
//
//// AssignAccounts assigns the fetched account to the checkers and crawler (if needed)
//func (c *Adapter) AssignAccounts(accs []models.RemoteAccount) error {
//	if len(accs) > 0 {
//		{
//			extractingAccounts := basecrawler.Filter(accs, func(a models.RemoteAccount) bool {
//				return basecrawler.ContainsTag(a.Tags, "export") && stillEnoughPermission(a.LastMessage)
//			})
//			numExtractingAccs := len(extractingAccounts)
//			c.accountsExtractingData = []*basecrawler.UsefulAccountInfo{}
//			if numExtractingAccs > 0 {
//				// for i, acc := range extractingAccounts {
//				// 	logger.Root.Infof("EAC (%d) Acc: `%s`", i, acc.Username)
//				// }
//				for _, a := range extractingAccounts {
//					newAcc := basecrawler.UsefulAccountInfo{
//						Account:   &models.RemoteAccount{},
//						UserAgent: utils.GetRandomUserAgent(),
//						Type:      basecrawler.AccountTypeExtractData,
//					}
//					*newAcc.Account = a
//					newAcc.CookieJar, _ = cookiejar.New(nil)
//					c.accountsExtractingData = append(c.accountsExtractingData, &newAcc)
//				}
//				// for i, acc := range p.accountsExtractingData {
//				// 	logger.Root.Infof("EAC (%d) Acc: `%s`", i, acc.Account.Username)
//				// }
//
//				for _, crawler := range c.crawlerList {
//					crawler.allAvailableAccounts = append([]*basecrawler.UsefulAccountInfo{}, c.accountsExtractingData...)
//					// rand.Seed(time.Now().UnixNano())
//					// c.allAvailableAccounts = shuffle(c.allAvailableAccounts)
//					// rand.Shuffle(len(c.allAvailableAccounts), func(i, j int) {
//					// 	c.allAvailableAccounts[i], c.allAvailableAccounts[j] = c.allAvailableAccounts[j], c.allAvailableAccounts[i]
//					// })
//					// if c.currentAccount == nil {
//					// 	c.currentAccount = c.allAvailableAccounts[0]
//					// }
//					// for i, acc := range c.allAvailableAccounts {
//					// 	logger.Root.Infof("Pretend %s (%d) Acc: `%s`", c.id, i, acc.Account.Username)
//					// }
//				}
//
//			} else {
//				logger.Root.Infof("There is no exporting account\n")
//				return errors.New("There is no exporting account")
//			}
//		}
//		{
//			checkingAccounts := basecrawler.Filter(accs, func(a models.RemoteAccount) bool {
//				return basecrawler.ContainsTag(a.Tags, "check") && stillEnoughPermission(a.LastMessage)
//			})
//			numCheckingAccs := len(checkingAccounts)
//			if numCheckingAccs > 0 {
//				for _, a := range checkingAccounts {
//					newAcc := basecrawler.UsefulAccountInfo{
//						Account:   &models.RemoteAccount{},
//						UserAgent: utils.GetRandomUserAgent(),
//						Type:      basecrawler.AccountTypeCheckData,
//					}
//					*newAcc.Account = a
//					newAcc.CookieJar, _ = cookiejar.New(nil)
//					c.accountsCheckingData = append(c.accountsCheckingData, &newAcc)
//				}
//
//				// for _, c := range p.checkers {
//				// 	c.allAvailableAccounts = append([]*basecrawler.UsefulAccountInfo{}, p.accountsExtractingData...)
//				// 	rand.Seed(time.Now().UnixNano())
//				// 	rand.Shuffle(len(c.allAvailableAccounts), func(i, j int) {
//				// 		c.allAvailableAccounts[i], c.allAvailableAccounts[j] = c.allAvailableAccounts[j], c.allAvailableAccounts[i]
//				// 	})
//				// 	if c.currentAccount == nil {
//				// 		c.currentAccount = c.allAvailableAccounts[0]
//				// 	}
//				// }
//			} else {
//
//				return errors.New("There is no checking account")
//			}
//		}
//		return nil
//
//	}
//	return basecrawler.ErrNoUsableAccount
//}
//
//// HandleRequest processes a request and return the response
//func (c *Adapter) HandleRequest(msg nsqmodels.Message) error {
//	if msg.Command == nil {
//		return nil
//	}
//
//	switch msg.Command.SubType {
//	case commands.SubTypeCheckStageByAppNo:
//		c.handleCheckStageRequest(msg)
//		break
//	}
//	return nil
//}
//
//// StartOrStopAllCrawlers will start or stop all crawlers based on the value of `start`, then run the function callback
//func (c *Adapter) StartOrStopAllCrawlers(start bool) {
//	for _, a := range c.crawlerList {
//		a.SetNeedToClose(!start)
//	}
//	c.needToCloseCrawlers = !start
//}
//
//// StartOrStopAllCheckers will start or stop all checkers based on the value of `start`, then run the function callback
//func (c *Adapter) StartOrStopAllCheckers(start bool) {
//	for _, a := range c.checkerList {
//		a.SetNeedToClose(!start)
//	}
//	c.needToCloseCheckers = !start
//}
//
//var dataInStr string
//
//func (c *Adapter) sendCheckResult(a metaCheckingAppResult) {
//	var finalErr error
//	var err error
//	var app customer.Application
//
//	if a.app != nil {
//		//app = *a.app
//		if !a.request.checkFull {
//			var stageSummary = customer.ApplicationStageSummary{
//				ApplicationStage:    app.ApplicationStage,
//				ApplicationSubStage: app.ApplicationSubStage,
//			}
//			if rInBytes, err := json.Marshal(stageSummary); err == nil {
//				dataInStr = string(rInBytes)
//			} else {
//				dataInStr = "@FAIL_TO_CHECK"
//				finalErr = err
//			}
//		} else {
//			logger.Root.Infof("Check full %s\n", a.app.WorkID)
//			if rInBytes, err := json.Marshal(app); err == nil {
//				dataInStr = string(rInBytes)
//			} else {
//				dataInStr = "@FAIL_TO_CHECK"
//				finalErr = err
//			}
//		}
//
//	} else {
//		// logger.Root.Errorf("Error SearchAppByAppNo: %s\n", err)
//		// if strings.Contains(strings.ToLower(err.Error()), "permission") {
//		// 	panic("Could not search by app no")
//		// }
//		dataInStr = "@FAIL_TO_CHECK"
//		finalErr = err
//	}
//
//	result := commands.CommandRespCheck{
//		DataID: a.request.dataID,
//		Data:   dataInStr,
//		Ok:     finalErr == nil,
//	}
//	if finalErr != nil {
//		result.Error = finalErr.Error()
//	}
//
//	resultInByte, err := json.Marshal(result)
//	if err != nil {
//		logger.Root.Errorf("Error when marshal check result to json. Error:%s\n", err)
//	} else {
//		logger.Root.Infof("Sending check result of app `%s`", a.request.dataID)
//		c.SendResponseCommand(a.commandID, a.commandSubType, string(resultInByte), a.channel)
//	}
//}
//
//func (c *Adapter) startReceivingCheckResultProcess() {
//	logger.Root.Infof("\nStarting the receiving process......\n")
//
//mainLoop:
//	for {
//		select {
//		// case <-finish:
//		// 	logger.Root.Infof("Got the finish signal")
//		// 	// write the rest
//		// 	if len(results) > 0 {
//		// 		p.submitApps(results, func() {})
//		// 		results = []customer.Application{}
//		// 	}
//		// 	break mainLoop
//		case a, ok := <-c.checked:
//			if ok && a.app != nil {
//				logger.Root.Infof("Got a check result app: %s", a.app.WorkID)
//				c.sendCheckResult(a)
//			} else {
//				logger.Root.Infof("Channel `checked` is broken")
//				break mainLoop
//			}
//
//		default:
//			// logger.Root.Infof("Break inside the select")
//			break
//		}
//
//	}
//
//	logger.Root.Infof("Exit function receiving process")
//}
//
//func (c *Adapter) handleCheckStageRequest(msg nsqmodels.Message) {
//	logger.Root.Infof(">>>>> CHECK STAGE REQ")
//	checkCm := commands.CommandReqCheck{}
//
//	body := msg.GetRawDataFromBody()
//	var err error
//	if err = json.Unmarshal([]byte(body), &checkCm); err == nil {
//		// logger.Root.Infof("Get command:%#v\n", checkCm)
//
//		var dataInStr string
//		var finalErr error
//
//		if !c.needToCloseCheckers && c.CanStartChecker() {
//			logger.Root.Infof("Send check req of `%s` to queue", checkCm.DataID)
//			c.needToChecked <- metaCheckingAppRequest{
//				dataID:         checkCm.DataID,
//				checkFull:      checkCm.CheckFull,
//				commandID:      msg.Command.ID,
//				commandSubType: msg.Command.SubType,
//				channel:        msg.Channel,
//			}
//			return
//
//		}
//
//		dataInStr = "@FAIL_TO_CHECK_NOT_ALLOW_NOW"
//		finalErr = errors.New("The operation is not allowed now")
//
//		result := commands.CommandRespCheck{
//			DataID: checkCm.DataID,
//			Data:   dataInStr,
//			Ok:     false,
//		}
//
//		result.Error = finalErr.Error()
//
//		resultInByte, err := json.Marshal(result)
//		if err != nil {
//			logger.Root.Errorf("Error when marshal check result to json. Error:%s\n", err)
//		} else {
//			c.SendResponseCommand(msg.Command.ID, msg.Command.SubType, string(resultInByte), msg.Channel)
//		}
//
//	} else {
//		logger.Root.Errorf("Error when parsing string :'%s' into object. Error:'%s'\n", body, err)
//	}
//}
//
//// Close closes the adapter
//func (c *Adapter) Close() error {
//	c.needToCloseCrawlers = true
//	c.needToCloseCheckers = true
//	c.Application.SetNeedToStopGettingData(true)
//	var wg sync.WaitGroup
//	wg.Add(2)
//	go func() {
//		for _, a := range c.crawlerList {
//			a.SetNeedToClose(true)
//			a.Close()
//		}
//		wg.Done()
//	}()
//	go func() {
//		for _, a := range c.checkerList {
//			a.SetNeedToClose(true)
//			a.Close()
//		}
//		wg.Done()
//	}()
//	if c.producer != nil {
//		c.producer.SetNeedToClose(true)
//		c.producer.Close()
//	}
//	logger.Root.Infof("WG wait!")
//	c.wg.Wait()
//	wg.Wait()
//	for len(c.crawlerList) > 0 {
//		logger.Root.Infof("Wait for all crawler stop")
//		time.Sleep(1 * time.Second)
//	}
//	for len(c.checkerList) > 0 {
//		logger.Root.Infof("Wait for all checker stop")
//		time.Sleep(1 * time.Second)
//	}
//
//	close(c.needToChecked)
//	close(c.needToCrawled)
//	close(c.crawled)
//	close(c.checked)
//
//	time.Sleep(3 * time.Second)
//
//	return nil
//}
//
//func (p *Adapter) checkIfIsInCrawlerList(id string) bool {
//	p.Lock()
//	defer p.Unlock()
//	_, ok := p.crawlerList[id]
//	return ok
//}
//
//func (c *Adapter) checkIfIsInCheckerList(id string) bool {
//	_, ok := c.checkerList[id]
//	return ok
//}
//
//func (c *Adapter) assignNewProducer() {
//	logger.Root.Infof("Need to use new producer")
//	emptyCrawlerList := false
//
//	c.Lock()
//	defer c.Unlock()
//
//	c.producer = nil
//	emptyCrawlerList = len(c.crawlerList) == 0
//
//	if emptyCrawlerList {
//		return
//	}
//
//	i := rand.Intn(len(c.crawlerList))
//	for _, v := range c.crawlerList {
//		if i == 0 {
//			// could not use removeCrawler here because of the deadlock when using `c.Lock()`
//			delete(c.crawlerList, v.id)
//			logger.Root.Infof("---> Use worker `%s` as the producer", v.id)
//			c.producer = v
//			break
//		}
//		i--
//	}
//
//}
//
//func (c *Adapter) makeSureProducerNotNil(wg *sync.WaitGroup) {
//	for !c.needToCloseCrawlers && c.producer == nil {
//		if len(c.crawlerList) != 0 {
//			c.assignNewProducer()
//		} else {
//			logger.Root.Infof("There is no producer or crawler yet...")
//			// c.createAndAddCrawlerToList(wg)
//			time.Sleep(3 * time.Second)
//		}
//	}
//}
//
//func (p *Adapter) getCrawlerParallelism() int {
//	N := 3
//	if d, ok := p.GetWorkerConfig().OtherConfig["crawlerParallelism"]; ok {
//		switch v := d.(type) {
//		case float64:
//			N = int(v)
//		case int:
//			N = v
//		default:
//			logger.Root.Infof("Could not convert crawlerParallelism %T to int", d)
//		}
//	}
//	return N
//}
//func (p *Adapter) getCheckerParallelism() int {
//	N := 2
//	if p.GetWorkerConfig().Checker.MaxNumWorkers > 0 {
//		return p.GetWorkerConfig().Checker.MaxNumWorkers
//	}
//	return N
//}
//
//func (p *Adapter) startCrawlerMonitor(wg *sync.WaitGroup) {
//	p.startedCrawlerWatcher = true
//	logger.Root.Infof("Starting crawler watcher....")
//	for {
//		if p.CanStartCrawler() && len(p.crawlerList) < p.getCrawlerParallelism() && !p.needToCloseCrawlers {
//			p.createAndAddCrawlerToList(wg)
//		} else {
//			if time.Now().Unix()%5 == 0 {
//				logger.Root.Infof("Number of crawlers: %d....", len(p.crawlerList))
//			}
//			time.Sleep(15 * time.Second)
//		}
//	}
//}
//
//func (p *Adapter) hasNoProducer() bool {
//	p.Lock()
//	defer p.Unlock()
//	return p.producer == nil
//}
//func (p *Adapter) setProducer(c *Crawler) {
//	p.Lock()
//	defer p.Unlock()
//	p.producer = c
//}
//
//func (p *Adapter) createAndAddCrawlerToList(wg *sync.WaitGroup) {
//
//	crawler, err := p.CreateCrawler(p.withDocker(), nil)
//	if err == nil && crawler != nil {
//		c := crawler.(*Crawler)
//		c.id = p.pickRandomColorName()
//
//		if p.hasNoProducer() {
//			logger.Root.Infof("---> Use browser `%s` as producer", c.id)
//			p.setProducer(c)
//			p.producer.id = "Producer - " + p.producer.id
//		} else {
//			c.id = "crawler-" + c.id
//			p.Lock()
//			p.crawlerList[c.id] = c
//			p.Unlock()
//			logger.Root.Infof("Added crawler `%s`", c.id)
//			go c.Run(wg)
//		}
//
//		time.Sleep(5 * time.Second)
//	}
//
//}
//func (p *Adapter) createAndAddCheckerToList(wg *sync.WaitGroup) {
//
//	checker, err := p.CreateChecker(p.withDocker(), nil)
//	if err == nil && checker != nil {
//		c := checker.(*Checker)
//		c.id = "checker-" + p.pickRandomColorName()
//		p.Lock()
//		p.checkerList[c.id] = c
//		logger.Root.Infof("Added checker `%s`", c.id)
//		p.Unlock()
//		go c.Run(wg)
//
//		time.Sleep(5 * time.Second)
//	}
//	// if len(p.crawlerList) == 0 {
//	// 	logger.Root.Infof("Don't create crawler because needToCloseCrawlers=%v", p.needToCloseCrawlers)
//	// }
//}
//
//func (p *Adapter) addNumNeedToCrawl(value int) {
//	p.Lock()
//	p.numNeedToCrawl += value
//	p.Unlock()
//}
//func (p *Adapter) addNumNeedToCheck(value int) {
//	p.Lock()
//	p.numNeedToCheck += value
//	p.Unlock()
//}
