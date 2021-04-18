package my_selenium

import (
	"context"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"sort"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// max age of browser before we closing it and replace by another one
const MAX_AGE = 10 * 60 * 60      // seconds (10 hours)
const MAX_TIME_TO_RESET = 25 * 60 // seconds
// const MAX_AGE = 2 * 60 // seconds

type WorkerConfiguration struct {
	ID    primitive.ObjectID
	Key   string `json:"key"`
	Value struct {
		StatusRequested string `json:"statusRequested"`

		StartHour          int   `json:"startHour"`
		StopHour           int   `json:"stopHour"`
		AutoRun            bool  `json:"autoRun"`
		NumWorkers         int   `json:"numWorkers"`
		InvalidateDuration int64 `json:"invalidateDuration"`
	} `json:"value"`
}

type CrawlerConfiguration struct {
	ID    primitive.ObjectID
	Key   string `json:"key"`
	Value struct {
		StatusRequested   string `json:"statusRequested"`
		MaxLastAppNo      int    `json:"maxLastAppNo"`
		MinAmountFinanced int    `json:"minAmountFinanced"`
		StartHour         int    `json:"startHour"`
		StopHour          int    `json:"stopHour"`
		AutoRun           bool   `json:"autoRun"`
	} `json:"value"`
}

type BrowserManager struct {
	sync.Mutex

	Workers            []*F1Browser
	AutoCrawlerBrowser *F1Crawler
	// DB                 *mongo.Database

	// workerConfiguration

	CrawlerInitializing     bool
	CrawlerInitializingTime int64
}

func SetupF1Browser(f1 *F1Browser, isWorker bool, d *BrowserManager) (err error) {
	f1.needToClose = false
	f1.IsInitializing = true

	defer func() {
		f1.IsInitializing = false
	}()

	err = f1.Login()
	if err != nil {
		logger.Root.Error("Error when logging in the worker ....")
		f1.Close()
		return
	}

	// // go to CAS screen
	// if err = f1.GoToCAS(); err != nil {
	// 	if err == ErrNoBackEnd {
	// 		if isWorker {
	// 			setSystemStatus(Stopping, WORKER)
	// 		} else {
	// 			setSystemStatus(Stopping, CRAWLER)
	// 		}
	// 	}
	// 	f1.Close()
	// 	return
	// }
	// if err = f1.GoToEnquiryScreen(); err == nil {
	// 	if isWorker && d != nil {
	// 		d.Lock()
	// 		d.Workers = append(d.Workers, f1)
	// 		f1.CreatedTime = GetCurrentTime().Unix()
	// 		d.Unlock()
	// 	}
	// 	return
	// }
	f1.Close()

	return nil
}

func (d *BrowserManager) LaunchF1Browser(isWorker bool, withDocker bool) *F1Browser {
	// curDir, _ := os.Getwd()
	// geckoPath := filepath.Join(curDir, "drivers", GECKOFILE)
	driver, err := GetSeleniumService(withDocker, "")
	if err != nil {
		logger.Root.Fatalf("Error when creating selenium driver. Error: %s\n", err)
	}
	if driver.Driver == nil {
		logger.Root.Fatal("The driver instance is nil")
	}

	f1 := F1Browser{
		Browser: &Browser{DriverInfo: driver, isActive: true, isBusy: false},
	}

	f1.Ctx = context.Background()
	if isWorker {
		f1.Type = TypeWorker
	} else {
		f1.Type = TypeCrawler
	}

	if err = SetupF1Browser(&f1, isWorker, d); err == nil {
		return &f1
	}

	return nil
}

// StartAutoCrawlerBrowser start a browser session that will be used to crawl data from F1 automatically
func (d *BrowserManager) StartAutoCrawlerBrowser(withDocker bool) *F1Crawler {
	d.CrawlerInitializing = true
	d.CrawlerInitializingTime = utils.GetCurrentTime().Unix()
	defer func() {
		d.CrawlerInitializing = false
	}()

	var f1 *F1Browser
	for f1 == nil {
		f1 = d.LaunchF1Browser(false, withDocker)
		if f1.checkStopRequestAndClose(false) {
			break
		}
	}
	if f1 != nil {
		c := F1Crawler{F1Browser: f1}
		c.Type = TypeCrawler
		d.AutoCrawlerBrowser = &c
		return &c
	}
	return nil
}

// StartAutoWorkers start a browser session that will be used to crawl data from F1 automatically
func (d *BrowserManager) StartAutoWorkers(defaultNumWorkers int, withDocker bool) {
	// start the workers for searching profile by CMND
	// numBrowsers := cfg.Application.NumWorkers
	if configWorker == nil {
		return
	}

	if configWorker.Value.NumWorkers == 0 {
		configWorker.Value.NumWorkers = defaultNumWorkers
	}

	// add a default browser to check id card, we add it here to provide the service faster
	d.AddF1Browsers(configWorker.Value.NumWorkers, withDocker)
}

// StopAutoCrawlerBrowser stops a browser session that will be used to crawl data from F1 automatically
func (d *BrowserManager) StopAutoCrawlerBrowser() error {
	setSystemStatus(Stopping, CRAWLER)
	// d.AutoCrawlerBrowser.Close()
	d.closeCrawler()

	setSystemStatus(Stopped, CRAWLER)
	return nil
}

// StopAutoWorkers close all workers
func (d *BrowserManager) StopAutoWorkers() error {
	setSystemStatus(Stopping, WORKER)
	// d.AutoCrawlerBrowser.Close()
	d.closeWorkers()

	setSystemStatus(Stopped, WORKER)
	return nil
}

// AddF1Browser creates a new browser
func (d *BrowserManager) AddF1Browser(withDocker bool) *F1Browser {
	return d.LaunchF1Browser(true, withDocker)
}

// AddF1Browsers add multiple browsers
func (d *BrowserManager) AddF1Browsers(numBrowsers int, withDocker bool) {
	if numBrowsers > 0 {
		c := 0
		if b := d.AddF1Browser(withDocker); b != nil {
			c++
			logger.Root.Infof("Started browser %d OK\n", c)
			b.Type = TypeWorker
		}

		if numBrowsers > 1 {
			// finishChan := make(chan bool, 1)

			// for c <= numBrowsers-1 {
			// 	go func(idx int) {
			// 		if b := d.AddF1Browser(); b != nil {
			// 			b.Type = TypeWorker
			// 			logger.Root.Infof("Started browser %d OK\n", idx)
			// 			finishChan <- true
			// 		}

			// 		time.Sleep(5 * time.Second)
			// 		if configWorker.Value.StatusRequested == Stopped || configWorker.Value.StatusRequested == Stopping {
			// 			logger.Root.Infoln("Stopping workers (by user)")
			// 			setSystemStatus(d.DB, Stopped, WORKER)
			// 			break
			// 		}
			// 	}(c)
			// 	<- finishChan
			// }

			go func(configWorker *WorkerConfiguration) {
				needToStop := configWorker.Value.StatusRequested == Stopped || configWorker.Value.StatusRequested == Stopping
				for len(d.Workers) <= numBrowsers-1 && c < numBrowsers-1 {
					logger.Root.Infof("Current worker requested status: %s\n", configWorker.Value.StatusRequested)
					if needToStop {
						logger.Root.Info("Stopping workers (by user)")
						setSystemStatus(Stopped, WORKER)
						break
					}
					if b := d.AddF1Browser(withDocker); b != nil {
						b.Type = TypeWorker
						logger.Root.Infof("Started browser %d OK\n", c)
						c++
					}
					time.Sleep(5 * time.Second)
					if needToStop {
						logger.Root.Info("Stopping workers (by user)")
						setSystemStatus(Stopped, WORKER)
						break
					}
				}
				if len(d.Workers) > 0 && !needToStop {
					setSystemStatus(Running, WORKER)
				} else {
					setSystemStatus(Stopped, WORKER)
				}
			}(configWorker)
		} else {
			if configWorker.Value.StatusRequested == Stopped || configWorker.Value.StatusRequested == Stopping {
				logger.Root.Info("Stopping workers (by user)")
				setSystemStatus(Stopped, WORKER)
			}
		}
	}
}

func (d *BrowserManager) closeWorkers() {
	ch := make(chan int)
	for i, b := range d.Workers {
		logger.Root.Infof("Closing browser %d \n", i)
		go func(_b *F1Browser, idx int) {
			_b.Close()
			ch <- idx

		}(b, i)
	}
	num := len(d.Workers)
	i := 0
	for i < num {
		idx := <-ch
		d.Lock()
		if idx < len(d.Workers) {
			logger.Root.Infof("Deleted browser at index %d\n", idx)
			d.Workers[idx] = nil
			// d.Workers = append(d.Workers[:idx], d.Workers[idx+1:]...)
		}

		d.Unlock()
		logger.Root.Infof("Closed browser %d \n", idx)
		i++
	}
	d.Workers = []*F1Browser{}

	d.updateNumberWorkersToDB()
}

func (d *BrowserManager) closeCrawler() {
	ch := make(chan int)
	go func() {
		if d.AutoCrawlerBrowser != nil {
			d.AutoCrawlerBrowser.Close()
		}
		ch <- 999
	}()
	<-ch
	d.AutoCrawlerBrowser = nil
	logger.Root.Infof("Closed crawler \n")
}

// Close will try to log out all account and close all browsers
func (d *BrowserManager) Close() {
	d.closeWorkers()
	d.closeCrawler()
}

// GetRandomBrowser returns an active random F1Browser
func (d *BrowserManager) GetRandomBrowser() *F1Browser {

	// idx := rand.Intn(len(d.Browsers))
	for i := range d.Workers {
		if d.Workers[i].IsActive() && !d.Workers[i].IsBusy() {
			logger.Root.Infof("Get browser %d\n", i)
			return d.Workers[i]
		}
	}

	// if all are active, just select the first active
	for i := range d.Workers {
		if d.Workers[i].IsActive() {
			logger.Root.Infof("Get browser %d\n", i)
			return d.Workers[i]
		}
	}
	if len(d.Workers) > 0 {
		return d.Workers[0]
	}
	return nil
}

func (d *BrowserManager) updateNumberWorkersToDB() {
	logger.Root.Infof("updateNumberWorkersToDB is not implemented yet")
	// numWorkers := len(d.Workers)

	// b, _ := json.Marshal(map[string]interface{}{
	// 	"numWorkers": numWorkers,
	// })

	// // logger.Root.Errorf("Error when updating the number of workers %s\n", err)

	// resultStr, err := utils.SendGetReq(cfg.WebService.URL+API_WORKER_NUMER, nil, map[string]string{"x-special-token": cfg.WebService.Token, "Content-Type": "application/json"}, "POST", bytes.NewBuffer(b))

	// var result ApiResult
	// json.Unmarshal([]byte(resultStr), &result)

	// // logger.Root.Infof("Return: %#v\n", result)

	// if err != nil || !result.Ok {
	// 	logger.Root.Errorf("Error when updating the number of workers %s\n", err)
	// } else {
	// 	// logger.Root.Errorf("Updating the number of workers OK!\n")
	// }
}

// RefreshAllWorkers marks old browser as inactive, and create new browser
func (d *BrowserManager) RefreshAllWorkers(withDocker bool) {
	go func() {
		counterIP := -1
		// number of times that we use the wrong network connection
		// networkFail := 0

		for {
			// numNewBrowsers := 0
			// now := GetCurrentTime().Unix()
			counterIP = (counterIP + 1) % 100

			// if counterIP%6 == 0 || networkFail > 0 {
			// 	currentPubIP := utils.GetPublicIp()
			// 	if currentPubIP == "ProhititIP" {
			// 		logger.Root.Warningln()
			// 		logger.Root.Warningln("+==================================================================+")
			// 		logger.Root.Warningln("||                                                                ||")
			// 		logger.Root.Warningln("||   /!\\  WRONG PUBLIC IP. PLEASE USE THE CORRECT NETWORK  /!\\    ||")
			// 		logger.Root.Warningln("||                                                                ||")
			// 		logger.Root.Warningln("+==================================================================+")
			// 		logger.Root.Warningln()

			// 		if cfg.Application.ConnectNetworkCommand != "" {
			// 			logger.Root.Infoln("Execute command to auto connect to the correct network")
			// 			utils.ExecuteBashCommand(strings.Fields(cfg.Application.ConnectNetworkCommand))
			// 		}

			// 		time.Sleep(20 * time.Second)
			// 		currentPubIP = GetPublicIp()
			// 		if currentPubIP == cfg.Application.ProhitbitIP {
			// 			networkFail++
			// 		} else {
			// 			networkFail = 0
			// 		}
			// 		logger.Root.Infof("Network fail: %d\n", networkFail)

			// 		if networkFail == 2 {

			// 			configCrawler, _ = getCrawlerConfiguration()
			// 			configWorker, _ = getWorkerConfiguration()
			// 			if configWorker.Value.StatusRequested != Stopping && configWorker.Value.StatusRequested != Stopped {
			// 				setSystemStatus(Stopping, WORKER)
			// 				logger.Root.Infoln("Update the worker status to stopping because we are using wrong network")
			// 			}

			// 			if configCrawler.Value.StatusRequested != Stopping && configCrawler.Value.StatusRequested != Stopped {
			// 				setSystemStatus(Stopping, CRAWLER)
			// 				logger.Root.Infoln("Update the crawler status to stopping because we are using wrong network")
			// 			}

			// 			utils.SendGetReq(cfg.WebService.URL+API_CRAWLER_NETWORK_FAIL, nil, map[string]string{"x-special-token": cfg.WebService.Token, "Content-Type": "application/json"}, "GET", nil)

			// 			networkFail = 0
			// 		}
			// 	} else {
			// 		networkFail = 0
			// 	}
			// }

			for i, b := range d.Workers {
				needReset := b.NeedToBeReset()
				// logger.Root.Infof("Browser %d needReset=%v busy=%v\n", i, needReset, b.IsBusy())
				// we add a random noise to avoid close all browsers at the same time
				if needReset && !b.IsBusy() {
					// numNewBrowsers++
					logger.Root.Infof("Need to reset the session for browser %d\n", i)

					// refresh session for browser
					// if b.SetDummyIDCardNumber() != nil {
					// 	logger.Root.Infoln("The browser can not be reseted. Need to re-create it")
					// 	b.CreatedTime = GetCurrentTime().Unix() - MAX_AGE
					// }
				}

			}

			for i, b := range d.Workers {
				// we add a random noise to avoid close all browsers at the same time
				if b.NeedToBeReplaced() && b.IsActive() {
					// numNewBrowsers++
					logger.Root.Info("Created new browser")
					for d.AddF1Browser(withDocker) == nil {
						if configWorker.Value.StatusRequested == Stopped {
							break
						}
						logger.Root.Info("The created browser has problem. Need to re-create it")
					}

					b.SetActive(false)
					logger.Root.Infof("Set browser at index %d to inactive\n", i)
				}
			}

			deleted := []int{}

			for i, b := range d.Workers {
				if !b.IsActive() && !b.IsBusy() {
					logger.Root.Infof("Close browser at index %d\n", i)
					b.Close()
					deleted = append(deleted, i)
				}
			}

			sort.Slice(deleted, func(i, j int) bool {
				return deleted[i] > deleted[j]
			})

			d.Lock()
			for _, idx := range deleted {
				if idx < len(d.Workers)-1 {
					logger.Root.Infof("Deleted browser at index %d\n", idx)

					d.Workers = append(d.Workers[:idx], d.Workers[idx+1:]...)
				} else {
					d.Workers = d.Workers[:idx]
				}
			}
			d.Unlock()
			d.updateNumberWorkersToDB()

			// for i := 0; i < numNewBrowsers; i++ {
			// 	logger.Root.Infoln("Created new browser")
			// 	d.AddF1Browser()
			// }

			time.Sleep(10 * time.Second)
		}
	}()
}

// StartCrawlerWatcher starts the watcher. This watcher will read configuration from database, such as:
// - startHour: the time we should start the crawler
// - stopHour: the time we should stop the crawler
// - maxLastAppNo: the maximum application number that we crawled
// - minAmountFinanced: the minimum financed amount of the profile that we should crawl
var configCrawler *CrawlerConfiguration

func (d *BrowserManager) StartCrawlerWatcher(withDocker bool) {
	go func() {
		lastStatus := Stopped

		for {
			now := utils.GetCurrentTime()

			// configCrawler, _ = getCrawlerConfiguration()

			statusBeforeUpdated := configCrawler.Value.StatusRequested
			logger.Root.Infof("last status: %s status before updated: %s\n", lastStatus, statusBeforeUpdated)

			if statusBeforeUpdated == Starting {

				// the status is starting, but it is the status from the last session
				// or if the starting status is last for more than 30 mins
				if !d.CrawlerInitializing || utils.GetCurrentTime().Unix()-d.CrawlerInitializingTime >= 30*60 {
					if d.AutoCrawlerBrowser != nil {
						d.closeCrawler()
					}
					// d.StartAutoCrawlerBrowser(d.DB)
					lastStatus = Stopped
				}

			}

			if (statusBeforeUpdated == Stopping || statusBeforeUpdated == Running) && d.AutoCrawlerBrowser == nil {
				setSystemStatus(Stopped, "crawler")
				lastStatus = Stopped
				continue
			}

			if configCrawler.Value.StartHour < 0 {
				configCrawler.Value.StartHour = 0
			}
			if configCrawler.Value.StopHour < 0 {
				configCrawler.Value.StopHour = 2359
			}

			nowInNumber := now.Hour()*100 + now.Minute()
			// logger.Root.Infof("crawler__ now: %d startHour:%d stopHour:%d requestedStatus:%s\n", nowInNumber, configCrawler.Value.StartHour, configCrawler.Value.StopHour, configCrawler.Value.StatusRequested)
			if configCrawler.Value.AutoRun {
				if nowInNumber >= configCrawler.Value.StartHour && nowInNumber <= configCrawler.Value.StopHour {
					if configCrawler.Value.StatusRequested == Stopped {
						configCrawler.Value.StatusRequested = Starting
						logger.Root.Infof("Override crawler status to %s (autorun)\n", Starting)
						setSystemStatus(Starting, CRAWLER)
					}
				} else {
					if configCrawler.Value.StatusRequested == Running {
						logger.Root.Infof("Override crawler status to %s (autorun)\n", Stopping)
						configCrawler.Value.StatusRequested = Stopping
						setSystemStatus(Stopping, CRAWLER)
					}
				}
			}

			if lastStatus == Stopped && configCrawler.Value.StatusRequested == Starting {
				logger.Root.Infof("Staring the crawler...\n")
				c := d.StartAutoCrawlerBrowser(withDocker)
				if err := c.Crawl(); err != nil {
					logger.Root.Errorf("Error when crawling. Error: %s\n", err)
				}
			} else if configCrawler.Value.StatusRequested == Stopping && d.AutoCrawlerBrowser != nil && d.AutoCrawlerBrowser.needToClose {
				logger.Root.Infof("Stopping the crawler...\n")
				d.StopAutoCrawlerBrowser()
			}

			lastStatus = statusBeforeUpdated
			time.Sleep(10 * time.Second)
		}

	}()
}

var configWorker *WorkerConfiguration

func (d *BrowserManager) StartWorkerWatcher(defaultNumWorkers int, withDocker bool) {

	// configWorker, _ = getWorkerConfiguration()
	statusBeforeUpdated := configWorker.Value.StatusRequested

	var firstTimeNoWorker int64
	if (statusBeforeUpdated == Stopping || statusBeforeUpdated == Running) && len(d.Workers) == 0 {
		setSystemStatus(Stopped, "worker")
	}

	go func() {
		lastStatus := Stopped
		firstTimeNoWorker = utils.GetCurrentTime().Unix()

		for {
			now := utils.GetCurrentTime()

			// configWorker, _ = getWorkerConfiguration()
			statusBeforeUpdated := configWorker.Value.StatusRequested

			if (statusBeforeUpdated == Stopping || statusBeforeUpdated == Running) && len(d.Workers) == 0 {
				// if there is no worker in 60 min, all workers seems to be stopped
				if now.Unix()-firstTimeNoWorker >= 60*60 || statusBeforeUpdated == Stopping {
					setSystemStatus(Stopped, "worker")
					firstTimeNoWorker = now.Unix()
				}
				continue
			}

			if configWorker.Value.StartHour < 0 {
				configWorker.Value.StartHour = 0
			}
			if configWorker.Value.StopHour < 0 {
				configWorker.Value.StopHour = 2359
			}

			nowInNumber := now.Hour()*100 + now.Minute()
			// logger.Root.Infof("worker__ now: %d startHour:%d stopHour:%d requestedStatus:%s\n", nowInNumber, configWorker.Value.StartHour, configWorker.Value.StopHour, configWorker.Value.StatusRequested)
			if configWorker.Value.AutoRun {
				if nowInNumber >= configWorker.Value.StartHour && nowInNumber <= configWorker.Value.StopHour {
					if configWorker.Value.StatusRequested == Stopped || len(d.Workers) == 0 {
						configWorker.Value.StatusRequested = Starting
						setSystemStatus(Starting, WORKER)
					}
				} else {
					if configWorker.Value.StatusRequested == Running || configWorker.Value.StatusRequested == Starting {
						configWorker.Value.StatusRequested = Stopping
						setSystemStatus(Stopping, WORKER)
					}
				}
			}

			needToClose := false
			for _, b := range d.Workers {
				if b != nil && !b.needToClose {
					needToClose = true
					break
				}
			}

			if (lastStatus == Stopped || len(d.Workers) == 0) && configWorker.Value.StatusRequested == Starting {
				logger.Root.Infof("Staring the worker...\n")
				d.StartAutoWorkers(defaultNumWorkers, withDocker)
				logger.Root.Infof("Started the worker...\n")
			} else if (lastStatus == Running || needToClose) && configWorker.Value.StatusRequested == Stopping {
				logger.Root.Infof("Stopping the worker...\n")
				d.StopAutoWorkers()
			}

			lastStatus = statusBeforeUpdated
			time.Sleep(10 * time.Second)
			d.updateNumberWorkersToDB()
		}

	}()
}
