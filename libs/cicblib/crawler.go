package cicblib

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong-internal/crawlers/pkg/my_selenium"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	"github.com/duc-thien-phong/techsharedservices/utils"
)

type Crawler struct {
	Base
}

func NewCrawler(withDocker bool, args map[string]interface{}) *Crawler {
	p := Crawler{}
	p.SetNeedToBeReplaced(false)

	browser := my_selenium.CreateBrowser(withDocker, 5*60*60, 5*60*60, args)
	if browser == nil {
		logger.Root.Errorf("Could not create selenium browser")
		return nil
	} else {
		logger.Root.Infof("Browser is created!")
	}
	p.Browser = browser

	return &p
}

func (p *Crawler) Prepare() error {
	return nil
}

func (p *Crawler) getMaxLifeDuration() int {
	N := 600
	if d, ok := p.ClientAdapter.GetHostApplication().GetWorkerConfig().OtherConfig["crawlerMaxLifeDuration"]; ok {
		if v, ok := d.(float64); ok {
			N = int(v)
		} else {
			logger.Root.Infof("Could not convert crawlerMaxLifeDuration %T to int", d)
		}
	}
	return N
}

func (p *Crawler) Run(wg *sync.WaitGroup) {
	for {
		if p.GetNeedToBeReplaced() || p.NeedToBeClosed() || p.ClientAdapter.GetNeedToCloseCrawlers() || !p.ClientAdapter.CheckIfIsInCrawlerList(p.GetID()) {
			logger.Root.Infof("browser: `%s` needToBeReplace or needToClose", p.GetID())
			break
		}

		wg.Add(1)
		// p.adapter.chanCrawlers <- struct{}{}
		p.startCrawling(wg)
		if p.GetNeedToBeReplaced() || p.NeedToBeClosed() || p.ClientAdapter.GetNeedToCloseCrawlers() || !p.ClientAdapter.CheckIfIsInCrawlerList(p.GetID()) {
			logger.Root.Infof("browser: `%s` needToBeReplace or needToClose", p.GetID())
			break
		}
	}

	p.Close()
	p.ClientAdapter.RemoveCrawler(p.GetID())
}

func (p *Crawler) ProcessPage(config *basecrawler.Config, pageSize int) (workIDs []string, hasNextPage bool, err error) {

	if p.ClientAdapter.(*Adapter).StartingID == 0 {
		logger.Root.Infof("stop crawling because starting id = 0")
		return
	}

	BatchSize := 500
	if p.ClientAdapter.(*Adapter).CurrentID == 0 {
		p.ClientAdapter.(*Adapter).CurrentID = p.ClientAdapter.(*Adapter).StartingID
	}
	if p.ClientAdapter.(*Adapter).StopID == 0 {
		p.ClientAdapter.(*Adapter).StopID = p.ClientAdapter.(*Adapter).CurrentID + BatchSize
	}
	logger.Root.
		With("currentID", p.ClientAdapter.(*Adapter).CurrentID).
		With("startingID", p.ClientAdapter.(*Adapter).StartingID).
		With("stoppingID", p.ClientAdapter.(*Adapter).StopID).
		With("batchSize", BatchSize).
		Infof("Inside fucntion Process Page of CICB")
	count := 0
	for count <= BatchSize &&
		p.ClientAdapter.(*Adapter).CurrentID < p.ClientAdapter.(*Adapter).StopID {
		count++
		workIDs = append(workIDs, fmt.Sprint(p.ClientAdapter.(*Adapter).CurrentID))
		p.ClientAdapter.(*Adapter).CurrentID++
	}
	hasNextPage = p.ClientAdapter.(*Adapter).CurrentID < p.ClientAdapter.(*Adapter).StopID
	err = nil
	logger.Root.Infof("Found %d workIds", len(workIDs))

	return
}

func (p *Crawler) startCrawling(wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = '"+p.GetID()+"'", []interface{}{})
	}()
	defer func() {
		if r := recover(); r != nil {
			logger.Root.Infof("Recover from startCrawling of `%s`. Err: %v", p.GetID(), r)
		}
	}()
	(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = 'Crawler - "+p.GetID()+"'", []interface{}{})
	// defer func() {
	// 	p.adapter.chanCrawlers <- struct{}{}
	// }()

	logger.Root.Infof("Browser `%s` (idx:%d) start crawling....", p.GetID(), p.ClientAdapter.GetNumRunningCrawlers())
	timer := time.NewTimer(time.Duration(p.getMaxLifeDuration()+rand.Intn(10)) * time.Minute)
	defer timer.Stop()
	var nedToStopBecauseOfError bool

	adapter, _ := (p.ClientAdapter).(*Adapter)
mainFor:
	for !nedToStopBecauseOfError && !p.GetNeedToBeReplaced() && !p.NeedToBeClosed() && !p.ClientAdapter.GetNeedToCloseCrawlers() {
		if !p.ClientAdapter.CheckIfIsInCrawlerList(p.GetID()) {
			// I don't belong to the crawler list anymore
			// maybe I am promoted to be a producer
			break
		}
		if p.numFails >= 10 {
			(*p.Browser.DriverInfo.Driver).Refresh()
			logger.Root.With("id", p.GetID()).Infof("Refresh because the numFails=%d", p.numFails)
			time.Sleep(5 * time.Second)
		} else if p.numFails >= 50 {
			logger.Root.With("id", p.GetID()).Infof("Need to be replaced because the numFails=%d", p.numFails)
			p.SetNeedToBeReplaced(true)
		}
		select {
		case <-timer.C:
			p.SetNeedToBeReplaced(true)
			timer.Stop()
			logger.Root.Infof("Set crawler `%s` to be replaced", p.GetID())
			break mainFor
		case meta := <-adapter.NeedToCrawled:
			adapter.AddNumNeedToCrawl(-1)
			if meta.DataID == "" {
				break mainFor
			}

			logger.Root.Infof("Browser `%s` Processes app %s", p.GetID(), meta.DataID)

			if adapter.CheckIfExtractedBefore(meta.DataID) || meta.Tried >= 7 {
				logger.Root.With("crawlerID", p.GetID()).Infof("`%s` Skips app %s. Current tried=%d", p.GetID(), meta.DataID, meta.Tried)
				adapter.MarkAsExtracted([]string{meta.DataID})
				break
			}

			var app *customer.Application
			err := utils.RetryOperation(func() error {
				var err error
				app, err = p.WorkerAdapter.FetchInformationOfProfile(meta.DataID, true)
				if err == ErrNoCustomer {
					return nil
				}
				return err
			}, 3, 1)
			if err == nil {
				p.numFails = 0
			}
			if err == nil && app != nil {
				logger.Root.Infof("`%s` finishes processing app %s. Sending to processed queue", p.GetID(), meta.DataID)
				adapter.Crawled <- basecrawler.MetaCrawlingAppResult{
					Request: meta,
					App:     app,
				}
				logger.Root.Infof("Sent %s to processed queue", meta.DataID)
			}
			if err != nil || app == nil {
				if err != nil && (strings.Contains(err.Error(), "NetworkError when attempting to fetch resource") || strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "invalid session id")) {
					nedToStopBecauseOfError = true
					p.SetNeedToBeReplaced(true)
				}
				if err == ErrInvalidSession {
					p.SetNeedToBeReplaced(true)
				}
				if err != nil && err != ErrNoCustomer {
					meta.Tried++
					p.numFails++
					adapter.AddCrawlingRequestBackToTheQueue(meta)
					logger.Root.Errorf("`%s` Coud not get the information of profile: %s %v", p.GetID(), meta.DataID, err)
					logger.Root.With("crawlerID", p.GetID()).Infof("Send application profile %s back to the queue", meta.DataID)
				} else if err == ErrNoCustomer {
					logger.Root.Errorf("`%s` could not found any customer with dataid: %s %v", p.GetID(), meta.DataID, err)
					adapter.MarkAsExtracted([]string{meta.DataID})
				}
			}
		default:
			break
		}
	}
	logger.Root.Infof("`%s` exit start crawling...", p.GetID)
}
