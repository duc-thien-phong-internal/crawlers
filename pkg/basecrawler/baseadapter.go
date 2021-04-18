package basecrawler

import (
	"encoding/json"
	"errors"
	"github.com/duc-thien-phong/techsharedservices/commands"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	nsqmodels "github.com/duc-thien-phong/techsharedservices/nsq/models"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"github.com/golang/glog"
	"math/rand"
	"net/http/cookiejar"
	"sync"
	"time"
)

type BaseAdapter struct {
	*Application

	sync.Mutex
	AllAvailableAccounts   []*UsefulAccountInfo
	AccountsExtractingData []*UsefulAccountInfo
	AccountsCheckingData   []*UsefulAccountInfo
	//
	Producer    ICrawler
	CrawlerList map[string]ICrawler
	CheckerList map[string]IChecker
	//
	NeedToChecked chan MetaCheckingAppRequest
	Checked       chan MetaCheckingAppResult
	//
	NeedToCrawled chan MetaCrawlingAppRequest
	Crawled       chan MetaCrawlingAppResult
	//
	NeedToCloseCrawlers bool
	NeedToCloseCheckers bool
	//
	StartedCrawlerWatcher bool
	StartedCheckerWatcher bool
	//
	NumNeedToCrawl int
	NumNeedToCheck int
	//
	CreateCrawlerFunc CreateCrawlerFunc
	CreateCheckerFunc CreateCheckerFunc
	//
	StillHasPermissionFunc func(string) bool

	BeforeCrawlingFunc func(args interface{})
	AfterCrawlingFunc  func(args interface{})

	Wg sync.WaitGroup
}

type MetaCheckingAppRequest struct {
	// can be workID (FE) or AppNo(MR) or DataID
	DataID string
	// need to re-fetch all information of the customer application ?
	CheckFull bool
	// id of the command
	CommandID commands.CommandID
	// subtype of the command
	CommandSubType commands.CommandSubType
	// Channel of the command
	Channel string
}

type MetaCheckingAppResult struct {
	Request MetaCheckingAppRequest
	// result
	App            *customer.Application
	CommandID      commands.CommandID
	CommandSubType commands.CommandSubType
	Channel        string
}

type MetaCrawlingAppRequest struct {
	// Data ID can be CIC Code, WorkID,...
	DataID      string
	IndexInPage int
	PageNo      int
}

type MetaCrawlingAppResult struct {
	Request MetaCrawlingAppRequest
	App     *customer.Application
}

type CreateCrawlerFunc func(withDocker bool, options map[string]interface{}) (ICrawler, error)
type CreateCheckerFunc func(withDocker bool, options map[string]interface{}) (IChecker, error)

func NewAdapter(
	createCrawlerFunc CreateCrawlerFunc,
	createCheckerFunc CreateCheckerFunc,
	stillHasPermissionFunc func(string) bool) *BaseAdapter {
	c := BaseAdapter{}
	c.CreateCrawlerFunc = createCrawlerFunc
	c.CreateCheckerFunc = createCheckerFunc
	c.StillHasPermissionFunc = stillHasPermissionFunc
	return &c
}

func (c BaseAdapter) GetHostApplication() *Application {
	return c.Application
}

func (c *BaseAdapter) SetHostApplication(a *Application) {
	c.Application = a
}

// just a way to verify if Adapter implements all methods of IClientAdapter or not
//var test IClientAdapter = &BaseAdapter{}

func (c *BaseAdapter) Init() {
	c.CrawlerList = map[string]ICrawler{}
	c.CheckerList = map[string]IChecker{}

	c.NeedToChecked = make(chan MetaCheckingAppRequest, 1000)
	c.Checked = make(chan MetaCheckingAppResult, 1000)
	c.NeedToCrawled = make(chan MetaCrawlingAppRequest, 1000)
	c.Crawled = make(chan MetaCrawlingAppResult, 1000)

	go func() {
		c.startCheckerMonitor(&c.Wg)
		c.startReceivingCheckResultProcess()
		c.startCrawlerMonitor(&c.Wg)
	}()
}

func (c *BaseAdapter) Crawl(config models.DataCrawlerConfig) ([]customer.Application, error) {

	defer func() {
		if r := recover(); r != nil {
			logger.Root.Errorf("Recovered from Crawl of CICB. %v", r)
		}
	}()

	if c.NeedToCloseCrawlers {
		logger.Root.Infof("Stop the crawling process before starting")
		return nil, nil
	}

	logger.Root.Infof("Crawling data....")

	c.MakeSureProducerNotNil(&c.Wg)

	pageSize := 50
	if d, ok := c.GetWorkerConfig().OtherConfig["pageSize"]; ok {
		logger.Root.Infof("Get value of `pageSize` as %v (%T)", d, d)
		if v, ok := d.(float64); ok {
			pageSize = int(v)
		}
	}
	if allow, ok := c.GetWorkerConfig().OtherConfig["allowFetchingAccounts"]; ok {
		logger.Root.Infof("Get value of `allowFetchingAccounts` as %v (%T)", allow, allow)
		if v, ok := allow.(bool); ok && v {
			logger.Root.Infof("Fetching accounts from server")
			c.NetworkManager.FetchAccounts()
		}
	}

	c.beforeCrawling()

	///// START PROCESS to RECEIVE apps and submit them to our server
	finish := make(chan struct{}, 1)
	go func() {
		logger.Root.Debugf("Starting the receiving process....")
	mainLoop:
		for {
			select {
			case _, _ = <-finish:
				logger.Root.Infof("Got the finish signal")
				break mainLoop
			default:
				break
			}
			results := make([]customer.Application, 1)
			a, ok := <-c.Crawled
			if !ok {
				logger.Root.Infof("The channel crawled is broken")
				break mainLoop
			}
			logger.Root.Infof("Got a result app: %s", a.App.WorkID)
			results[0] = *a.App

			// remaining:
			for len(results) < c.GetWorkerConfig().Crawler.WriteAfter {
				select {
				case _, _ = <-finish:
					logger.Root.Infof("Got the finish signal")
					// write the rest
					if len(results) > 0 {
						c.SubmitApps(results, func() {})
						results = []customer.Application{}
					}
					break mainLoop
				case a, ok := <-c.Crawled:
					if !ok {
						logger.Root.Infof("The channel crawled is broken")
						if len(results) > 0 {
							c.SubmitApps(results, func() {})
							results = []customer.Application{}
						}
						break mainLoop
					}
					logger.Root.Infof("Got a result app: %s", a.App.WorkID)
					results = append(results, *a.App)
				default:
					break
				}
			}

			if len(results) > 0 {
				logger.Root.Infof("Submitting results....")
				c.SubmitApps(results, func() {})
				results = []customer.Application{}
			}

		}

		logger.Root.Infof("Exit function receiving process")
	}()

	/// START THE MAIN PROCESS OF THE PRODUCER
	totalSkip := 0
	totalWorkIDs := 0
	hasNextPage := true

	for hasNextPage && !c.NeedToCloseCrawlers {

		if c.Producer.NeedToBeClosed() {
			glog.Infof("Need to close --> skip crawling")
			break
		}

		// crawler.sortThePage()
		glog.Infof("Producer `%s` processes page %d----------------------", c.Producer.GetID(), c.GetConfig().CurrentCrawlingPage)

		var err error
		var workIDs []string
		var hasNextPage bool
		for retries := 0; retries < 3 && len(workIDs) == 0 && !c.Producer.NeedToBeClosed(); retries++ {
			glog.Infof("retries: %d", retries)

			workIDs, hasNextPage, err = c.Producer.ProcessPage(c.GetConfig(), pageSize)
		}
		if err != nil {
			c.Producer.SetNeedToClose(true)
			c.Producer.SetNeedToBeReplaced(true)
			break
		}

		// begin debug
		// workIDs = []string{"CDL-9545028", "LV-1885203", "PLT-116388"}
		// hasNextPage = false
		if len(c.NeedToCrawled) >= c.GetMaxCrawlingInQueue() {
			break
		}
		// end debug

		if len(workIDs) == 0 {
			//filename := utils.WriteContentToPage([]byte(pageContent), "page-", "empty-work-ids")
			//glog.Infof("Log content to file `%s`", filename)
			logger.Root.Infof("There is no data id anymore")
			break
		}
		glog.Infof("Found %d work ids in page %d", len(workIDs), c.GetConfig().CurrentCrawlingPage)

		glog.Infof("has next page: %v", hasNextPage)
		skip := 0
		nedToStopBecauseOfError := false
		totalWorkIDs += len(workIDs)

		for i, id := range workIDs {
			if id == "" || c.CheckIfExtractedBefore(id) {
				// glog.Infof("Skip app `%s`", id)
				skip++
				continue
			}

			// glog.Infof("puts app `%s` to the list", id)
			c.AddCrawlingRequestBackToTheQueue(MetaCrawlingAppRequest{
				DataID:      id,
				IndexInPage: i,
				PageNo:      c.GetConfig().CurrentCrawlingPage,
			})
			c.AddNumNeedToCrawl(1)

			// TODO: update me
			if nedToStopBecauseOfError {
				break
			}
		}

		if skip >= 0 {
			glog.Infof("Skipped %d applications.", skip)
		}
		totalSkip += skip

		if hasNextPage && !nedToStopBecauseOfError {
			c.GetConfig().CurrentCrawlingPage++
		} else {
			// glog.Infof("Sleep 60 seconds before refresh this page (%d)", p.currentPage)
			// time.Sleep(60 * time.Second)
			break
		}
	}

	for i := 0; i < len(c.CrawlerList); i++ {
		c.NeedToCrawled <- MetaCrawlingAppRequest{
			DataID:      "",
			IndexInPage: 0,
			PageNo:      0,
		}
	}
	// p.wg.Wait()
	// p.chanCrawlers <- struct{}{}
	// for range p.crawlerList {
	// 	<-p.chanCrawlers
	// }

	lastNumNeedToCrawl := 0
	for c.NumNeedToCrawl > 0 && !c.NeedToCloseCrawlers {
		if lastNumNeedToCrawl != c.NumNeedToCrawl {
			glog.Infof("Need to crawled remaining: %d (in queue: %d)", c.NumNeedToCrawl, len(c.NeedToCrawled))
			lastNumNeedToCrawl = c.NumNeedToCrawl
		}
		time.Sleep(5 * time.Second)
	}

	glog.Infof("Send signal to stop the process")
	finish <- struct{}{}
	glog.Infof("Sent!")

	glog.Infof("\n\nSTOP CRAWLING.......\n\n")
	if totalSkip >= totalWorkIDs-pageSize {
		sleepTime := int(30*float64(totalSkip)/float64(totalWorkIDs+1)) + (totalSkip+1)/(totalWorkIDs+1)*60
		glog.Infof("We have too many skips (%d/%d). Need to sleep a bit (%d seconds)", totalSkip, totalWorkIDs, sleepTime)
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}

	if c.Producer.GetNeedToBeReplaced() || c.Producer.NeedToBeClosed() {
		c.Producer.Close()
		c.AssignNewProducer()
	}

	c.RemoveOldIds()
	c.afterCrawling()
	return []customer.Application{}, nil
}

func (c *BaseAdapter) beforeCrawling() {
	if c.BeforeCrawlingFunc != nil {
		logger.Root.Infof("Running function BeforeCrawling")
		c.BeforeCrawlingFunc(nil)
	} else {
		logger.Root.Warnf("Function BeforeCrawling is not set properly")
	}
}

func (c *BaseAdapter) afterCrawling() {
	if c.AfterCrawlingFunc != nil {
		logger.Root.Infof("Running function AfterCrawling")
		c.AfterCrawlingFunc(nil)
	} else {
		logger.Root.Warnf("Function AfterCrawling is not set properly")
	}
}

// GetACrawler returns a random created crawler
func (p *BaseAdapter) GetACrawler() ICrawler {
	retries := 0

	var newC ICrawler = nil
	var err error
	for newC == nil && !p.NeedToCloseCrawlers {
		newC, err = p.CreateCrawlerFunc(p.WithDocker(), nil)
		if err == ErrNoUsableAccount {
			logger.Root.Errorf("There is no usable account")
			return nil
		}
		retries++
		time.Sleep(time.Duration(retries) * time.Second)
	}
	return newC
}

func (p *BaseAdapter) RemoveCrawler(id string) {
	if c, ok := p.CrawlerList[id]; ok {
		logger.Root.Infof("Deleting worker `%s` from the crawler list", id)
		p.Lock()
		delete(p.CrawlerList, id)
		p.Unlock()

		// if the deleted crawler is also the producer
		// pick a random crawler and assign it to the role producer
		if p.Producer != nil && c.GetID() == p.Producer.GetID() {
			p.AssignNewProducer()
		}
	}
}

// GetAChecker returns a new random created checker
func (p *BaseAdapter) GetAChecker() IChecker {
	// No need to use it for now
	return nil
}

func (p *BaseAdapter) RemoveChecker(id string) {
	if _, ok := p.CheckerList[id]; ok {
		logger.Root.Infof("Deleting worker `%s` from the checker list", id)
		p.Lock()
		delete(p.CheckerList, id)
		p.Unlock()
	}
}

func (p *BaseAdapter) GetNumRunningCrawlers() int {
	p.Lock()
	defer p.Unlock()
	return len(p.CrawlerList)
}

func (p *BaseAdapter) GetNumRunningCheckers() int {
	p.Lock()
	defer p.Unlock()
	return len(p.CheckerList)
}

func (p BaseAdapter) GetNeedToCloseCrawlers() bool {
	return p.NeedToCloseCrawlers
}
func (p BaseAdapter) GetNeedToCloseCheckers() bool {
	return p.NeedToCloseCheckers
}
func (p *BaseAdapter) SetNeedToCloseCrawlers(v bool) {
	p.NeedToCloseCrawlers = v
}
func (p *BaseAdapter) SetNeedToCloseCheckers(v bool) {
	p.NeedToCloseCheckers = v
}

func (c *BaseAdapter) WithDocker() bool {
	if d, ok := c.GetWorkerConfig().OtherConfig["withDocker"]; ok {
		logger.Root.Infof("Get value of `withDocker` as %v (%T)", d, d)
		if v, ok := d.(bool); ok {
			logger.Root.Infof("useDocker: %v", v)
			return v
		}
	}
	return true
}

func (c *BaseAdapter) startCheckerMonitor(wg *sync.WaitGroup) {
	c.StartedCheckerWatcher = true
	logger.Root.Infof("Starting checker watcher....")
	for {
		if c.CanStartChecker() && len(c.CheckerList) < c.getCheckerParallelism() && !c.NeedToCloseCheckers {
			c.createAndAddCheckerToList(wg)
		} else {
			if time.Now().Unix()%5 == 0 {
				logger.Root.Infof("Number of checkers: %d....", len(c.CheckerList))
			}
			time.Sleep(15 * time.Second)
		}
	}
}

func (c *BaseAdapter) PickRandomColorName() string {
	for {
		name := ColorNames[rand.Intn(len(ColorNames))]
		if _, ok := c.CrawlerList[name]; !ok {
			return name
		}
	}
	// return colorNames[0] + " " + fmt.Sprint(time.Now().Unix())
}

// AssignAccounts assigns the fetched account to the checkers and crawler (if needed)
func (c *BaseAdapter) AssignAccounts(accs []models.RemoteAccount) error {
	if len(accs) > 0 {
		{
			extractingAccounts := Filter(accs, func(a models.RemoteAccount) bool {
				return ContainsTag(a.Tags, "export") && c.StillHasPermissionFunc(a.LastMessage)
			})
			numExtractingAccs := len(extractingAccounts)
			c.AccountsExtractingData = []*UsefulAccountInfo{}
			if numExtractingAccs > 0 {
				// for i, acc := range extractingAccounts {
				// 	logger.Root.Infof("EAC (%d) Acc: `%s`", i, acc.Username)
				// }
				for _, a := range extractingAccounts {
					newAcc := UsefulAccountInfo{
						Account:   &models.RemoteAccount{},
						UserAgent: utils.GetRandomUserAgent(),
						Type:      AccountTypeExtractData,
					}
					*newAcc.Account = a
					newAcc.CookieJar, _ = cookiejar.New(nil)
					c.AccountsExtractingData = append(c.AccountsExtractingData, &newAcc)
				}
				// for i, acc := range p.AccountsExtractingData {
				// 	logger.Root.Infof("EAC (%d) Acc: `%s`", i, acc.Account.Username)
				// }

				for _, crawler := range c.CrawlerList {
					crawler.SetAvailableAccounts(append([]*UsefulAccountInfo{}, c.AccountsExtractingData...))
					// rand.Seed(time.Now().UnixNano())
					// c.AllAvailableAccounts = shuffle(c.AllAvailableAccounts)
					// rand.Shuffle(len(c.AllAvailableAccounts), func(i, j int) {
					// 	c.AllAvailableAccounts[i], c.AllAvailableAccounts[j] = c.AllAvailableAccounts[j], c.AllAvailableAccounts[i]
					// })
					// if c.currentAccount == nil {
					// 	c.currentAccount = c.AllAvailableAccounts[0]
					// }
					// for i, acc := range c.AllAvailableAccounts {
					// 	logger.Root.Infof("Pretend %s (%d) Acc: `%s`", c.id, i, acc.Account.Username)
					// }
				}

			} else {
				logger.Root.Infof("There is no exporting account\n")
				return errors.New("There is no exporting account")
			}
		}
		{
			checkingAccounts := Filter(accs, func(a models.RemoteAccount) bool {
				return ContainsTag(a.Tags, "check") && c.StillHasPermissionFunc(a.LastMessage)
			})
			numCheckingAccs := len(checkingAccounts)
			if numCheckingAccs > 0 {
				for _, a := range checkingAccounts {
					newAcc := UsefulAccountInfo{
						Account:   &models.RemoteAccount{},
						UserAgent: utils.GetRandomUserAgent(),
						Type:      AccountTypeCheckData,
					}
					*newAcc.Account = a
					newAcc.CookieJar, _ = cookiejar.New(nil)
					c.AccountsCheckingData = append(c.AccountsCheckingData, &newAcc)
				}

				// for _, c := range p.checkers {
				// 	c.AllAvailableAccounts = append([]*basecrawler.UsefulAccountInfo{}, p.AccountsExtractingData...)
				// 	rand.Seed(time.Now().UnixNano())
				// 	rand.Shuffle(len(c.AllAvailableAccounts), func(i, j int) {
				// 		c.AllAvailableAccounts[i], c.AllAvailableAccounts[j] = c.AllAvailableAccounts[j], c.AllAvailableAccounts[i]
				// 	})
				// 	if c.currentAccount == nil {
				// 		c.currentAccount = c.AllAvailableAccounts[0]
				// 	}
				// }
			} else {

				return errors.New("There is no checking account")
			}
		}
		return nil

	}
	return ErrNoUsableAccount
}

// HandleRequest processes a request and return the response
func (c *BaseAdapter) HandleRequest(msg nsqmodels.Message) error {
	if msg.Command == nil {
		return nil
	}

	switch msg.Command.SubType {
	case commands.SubTypeCheckStageByAppNo:
		c.handleCheckStageRequest(msg)
		break
	}
	return nil
}

// StartOrStopAllCrawlers will start or stop all crawlers based on the value of `start`, then run the function callback
func (c *BaseAdapter) StartOrStopAllCrawlers(start bool) {
	for _, a := range c.CrawlerList {
		a.SetNeedToClose(!start)
	}
	c.NeedToCloseCrawlers = !start
}

// StartOrStopAllCheckers will start or stop all checkers based on the value of `start`, then run the function callback
func (c *BaseAdapter) StartOrStopAllCheckers(start bool) {
	for _, a := range c.CheckerList {
		a.SetNeedToClose(!start)
	}
	c.NeedToCloseCheckers = !start
}

var dataInStr string

func (c *BaseAdapter) sendCheckResult(a MetaCheckingAppResult) {
	var finalErr error
	var err error
	var app customer.Application

	if a.App != nil {
		//app = *a.app
		if !a.Request.CheckFull {
			var stageSummary = customer.ApplicationStageSummary{
				ApplicationStage:    app.ApplicationStage,
				ApplicationSubStage: app.ApplicationSubStage,
			}
			if rInBytes, err := json.Marshal(stageSummary); err == nil {
				dataInStr = string(rInBytes)
			} else {
				dataInStr = "@FAIL_TO_CHECK"
				finalErr = err
			}
		} else {
			logger.Root.Infof("Check full %s\n", a.App.WorkID)
			if rInBytes, err := json.Marshal(app); err == nil {
				dataInStr = string(rInBytes)
			} else {
				dataInStr = "@FAIL_TO_CHECK"
				finalErr = err
			}
		}

	} else {
		// logger.Root.Errorf("Error SearchAppByAppNo: %s\n", err)
		// if strings.Contains(strings.ToLower(err.Error()), "permission") {
		// 	panic("Could not search by app no")
		// }
		dataInStr = "@FAIL_TO_CHECK"
		finalErr = err
	}

	result := commands.CommandRespCheck{
		DataID: a.Request.DataID,
		Data:   dataInStr,
		Ok:     finalErr == nil,
	}
	if finalErr != nil {
		result.Error = finalErr.Error()
	}

	resultInByte, err := json.Marshal(result)
	if err != nil {
		logger.Root.Errorf("Error when marshal check result to json. Error:%s\n", err)
	} else {
		logger.Root.Infof("Sending check result of app `%s`", a.Request.DataID)
		c.SendResponseCommand(a.CommandID, a.CommandSubType, string(resultInByte), a.Channel)
	}
}

func (c *BaseAdapter) startReceivingCheckResultProcess() {
	logger.Root.Infof("\nStarting the receiving process......\n")

mainLoop:
	for {
		select {
		// case <-finish:
		// 	logger.Root.Infof("Got the finish signal")
		// 	// write the rest
		// 	if len(results) > 0 {
		// 		p.submitApps(results, func() {})
		// 		results = []customer.Application{}
		// 	}
		// 	break mainLoop
		case a, ok := <-c.Checked:
			if ok && a.App != nil {
				logger.Root.Infof("Got a check result app: %s", a.App.WorkID)
				c.sendCheckResult(a)
			} else {
				logger.Root.Infof("Channel `checked` is broken")
				break mainLoop
			}

		default:
			// logger.Root.Infof("Break inside the select")
			break
		}

	}

	logger.Root.Infof("Exit function receiving process")
}

func (c *BaseAdapter) handleCheckStageRequest(msg nsqmodels.Message) {
	logger.Root.Infof(">>>>> CHECK STAGE REQ")
	checkCm := commands.CommandReqCheck{}

	body := msg.GetRawDataFromBody()
	var err error
	if err = json.Unmarshal([]byte(body), &checkCm); err == nil {
		// logger.Root.Infof("Get command:%#v\n", checkCm)

		var dataInStr string
		var finalErr error

		if !c.NeedToCloseCheckers && c.CanStartChecker() {
			logger.Root.Infof("Send check req of `%s` to queue", checkCm.DataID)
			c.NeedToChecked <- MetaCheckingAppRequest{
				DataID:         checkCm.DataID,
				CheckFull:      checkCm.CheckFull,
				CommandID:      msg.Command.ID,
				CommandSubType: msg.Command.SubType,
				Channel:        msg.Channel,
			}
			return

		}

		dataInStr = "@FAIL_TO_CHECK_NOT_ALLOW_NOW"
		finalErr = errors.New("The operation is not allowed now")

		result := commands.CommandRespCheck{
			DataID: checkCm.DataID,
			Data:   dataInStr,
			Ok:     false,
		}

		result.Error = finalErr.Error()

		resultInByte, err := json.Marshal(result)
		if err != nil {
			logger.Root.Errorf("Error when marshal check result to json. Error:%s\n", err)
		} else {
			c.SendResponseCommand(msg.Command.ID, msg.Command.SubType, string(resultInByte), msg.Channel)
		}

	} else {
		logger.Root.Errorf("Error when parsing string :'%s' into object. Error:'%s'\n", body, err)
	}
}

// Close closes the client
func (c *BaseAdapter) Close() error {
	c.NeedToCloseCrawlers = true
	c.NeedToCloseCheckers = true
	c.Application.SetNeedToStopGettingData(true)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for _, a := range c.CrawlerList {
			a.SetNeedToClose(true)
			a.Close()
		}
		wg.Done()
	}()
	go func() {
		for _, a := range c.CheckerList {
			a.SetNeedToClose(true)
			a.Close()
		}
		wg.Done()
	}()
	if c.Producer != nil {
		c.Producer.SetNeedToClose(true)
		c.Producer.Close()
	}
	logger.Root.Infof("WG wait!")
	c.Wg.Wait()
	wg.Wait()
	for len(c.CrawlerList) > 0 {
		logger.Root.Infof("Wait for all crawler stop")
		time.Sleep(1 * time.Second)
	}
	for len(c.CheckerList) > 0 {
		logger.Root.Infof("Wait for all checker stop")
		time.Sleep(1 * time.Second)
	}

	close(c.NeedToChecked)
	close(c.NeedToCrawled)
	close(c.Crawled)
	close(c.Checked)

	time.Sleep(3 * time.Second)

	return nil
}

func (p *BaseAdapter) CheckIfIsInCrawlerList(id string) bool {
	p.Lock()
	defer p.Unlock()
	_, ok := p.CrawlerList[id]
	return ok
}

func (c *BaseAdapter) CheckIfIsInCheckerList(id string) bool {
	_, ok := c.CheckerList[id]
	return ok
}

func (c *BaseAdapter) AssignNewProducer() {
	logger.Root.Infof("Need to use new producer")
	emptyCrawlerList := false

	c.Lock()
	defer c.Unlock()

	c.Producer = nil
	emptyCrawlerList = len(c.CrawlerList) == 0

	if emptyCrawlerList {
		return
	}

	i := rand.Intn(len(c.CrawlerList))
	for _, v := range c.CrawlerList {
		if i == 0 {
			// could not use removeCrawler here because of the deadlock when using `c.Lock()`
			delete(c.CrawlerList, v.GetID())
			logger.Root.Infof("---> Use worker `%s` as the producer", v.GetID())
			c.Producer = v
			break
		}
		i--
	}

}

func (c *BaseAdapter) MakeSureProducerNotNil(wg *sync.WaitGroup) {
	for !c.NeedToCloseCrawlers && c.Producer == nil {
		if len(c.CrawlerList) != 0 {
			c.AssignNewProducer()
		} else {
			logger.Root.Infof("There is no producer or crawler yet...")
			// c.createAndAddCrawlerToList(Wg)
			time.Sleep(3 * time.Second)
		}
	}
}

func (p *BaseAdapter) getCrawlerParallelism() int {
	N := 3
	if d, ok := p.GetWorkerConfig().OtherConfig["crawlerParallelism"]; ok {
		switch v := d.(type) {
		case float64:
			N = int(v)
		case int:
			N = v
		default:
			logger.Root.Infof("Could not convert crawlerParallelism %T to int", d)
		}
	}
	return N
}

func (p *BaseAdapter) getCheckerParallelism() int {
	N := 2
	if p.GetWorkerConfig().Checker.MaxNumWorkers > 0 {
		return p.GetWorkerConfig().Checker.MaxNumWorkers
	}
	return N
}

func (p *BaseAdapter) GetMaxCrawlingInQueue() int {
	N := 500
	if d, ok := p.GetWorkerConfig().OtherConfig["crawlingQueue"]; ok {
		switch v := d.(type) {
		case float64:
			N = int(v)
		case int:
			N = v
		default:
			glog.Infof("Could not convert crawlingQueue %T to int", d)
		}
	}
	return N
}

func (p *BaseAdapter) startCrawlerMonitor(wg *sync.WaitGroup) {
	p.StartedCrawlerWatcher = true
	logger.Root.Infof("Starting crawler watcher....")
	for {
		if p.CanStartCrawler() && len(p.CrawlerList) < p.getCrawlerParallelism() && !p.NeedToCloseCrawlers {
			p.createAndAddCrawlerToList(wg)
		} else {
			if time.Now().Unix()%5 == 0 {
				logger.Root.Infof("Number of crawlers: %d....", len(p.CrawlerList))
			}
			time.Sleep(15 * time.Second)
		}
	}
}

func (p *BaseAdapter) HasNoProducer() bool {
	p.Lock()
	defer p.Unlock()
	return p.Producer == nil
}
func (p *BaseAdapter) SetProducer(c ICrawler) {
	p.Lock()
	defer p.Unlock()
	p.Producer = c
}

func (p *BaseAdapter) createAndAddCrawlerToList(wg *sync.WaitGroup) {

	crawler, err := p.CreateCrawlerFunc(p.WithDocker(), nil)
	if err == nil && crawler != nil {
		c := crawler //.(*Crawler)
		c.SetID(p.PickRandomColorName())

		if p.HasNoProducer() {
			logger.Root.Infof("---> Use browser `%s` as producer", c.GetID())
			p.SetProducer(c)
			p.Producer.SetID("Producer - " + p.Producer.GetID())
		} else {
			c.SetID("crawler-" + c.GetID())
			p.Lock()
			p.CrawlerList[c.GetID()] = c
			p.Unlock()
			logger.Root.Infof("Added crawler `%s`", c.GetID())
			go c.Run(wg)
		}

		time.Sleep(5 * time.Second)
	}
}

func (p *BaseAdapter) createAndAddCheckerToList(wg *sync.WaitGroup) {

	if p.CreateCheckerFunc == nil {
		logger.Root.Fatalf("function CreateCheckerFunc need to be set. %#v", *p)
	}
	checker, err := p.CreateCheckerFunc(p.WithDocker(), nil)
	if err == nil && checker != nil {
		c := checker //.(*Checker)
		c.SetID("checker-" + p.PickRandomColorName())
		p.Lock()
		p.CheckerList[c.GetID()] = c
		logger.Root.Infof("Added checker `%s`", c.GetID())
		p.Unlock()
		go c.Run(wg)

		time.Sleep(5 * time.Second)
	}
	// if len(p.crawlerList) == 0 {
	// 	logger.Root.Infof("Don't create crawler because NeedToCloseCrawlers=%v", p.NeedToCloseCrawlers)
	// }
}

func (p *BaseAdapter) SubmitApps(results []customer.Application, onOK func()) {
	if len(results) == 0 {
		return
	}
	ids := []string{}
	for _, a := range results {
		ids = append(ids, a.WorkID)
	}
	p.SendAddingApplicationsReq(append([]customer.Application{}, results...), 3, func() {
		p.MarkAsExtracted(ids)
		p.WriteCrawledIDNos()
		for _, r := range results {
			if r.ApplicationDate.Unix() > p.GetWorkerConfig().Crawler.LastCrawlingDate.Unix() {
				p.GetWorkerConfig().Crawler.LastCrawlingDate = time.Unix(r.ApplicationDate.Unix(), 0)
			}
		}
		if onOK != nil {
			onOK()
		}
	})
}

func (p *BaseAdapter) AddCrawlingRequestBackToTheQueue(request MetaCrawlingAppRequest) {
	p.NeedToCrawled <- request
	p.AddNumNeedToCrawl(1)
}

func (p *BaseAdapter) AddNumNeedToCrawl(value int) {
	p.Lock()
	p.NumNeedToCrawl += value
	p.Unlock()
}
func (p *BaseAdapter) addNumNeedToCheck(value int) {
	p.Lock()
	p.NumNeedToCheck += value
	p.Unlock()
}
