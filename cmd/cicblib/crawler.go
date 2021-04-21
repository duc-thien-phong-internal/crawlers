package cicblib

import (
	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong-internal/crawlers/pkg/my_selenium"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type Crawler struct {
	Base
}

func NewCrawler(withDocker bool, args map[string]interface{}) *Crawler {
	p := Crawler{}
	p.SetNeedToBeReplaced(false)

	browser := my_selenium.CreateBrowser(withDocker, 24*60*60, 24*60*60, args)
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
	p.ClientAdapter.RemoveCrawler(p.GetID())

	p.Close()
}

func (p *Crawler) ProcessPage(config *basecrawler.Config, pageSize int) (workIDs []string, hasNextPage bool, err error) {
	logger.Root.Infof("Inside fucntion Process Page of CICB")
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
		select {
		case <-timer.C:
			p.SetNeedToBeReplaced(true)
			timer.Stop()
			logger.Root.Infof("Set crawler `%s` to be replaced", p.GetID())
			break mainFor
		case meta := <-adapter.NeedToCrawled:
			if meta.DataID == "" {
				break mainFor
			}

			logger.Root.Infof("Browser `%s` Processes app %s", p.GetID(), meta.DataID)

			adapter.AddNumNeedToCrawl(-1)
			if adapter.CheckIfExtractedBefore(meta.DataID) {
				logger.Root.Infof("`%s` Skips app %s", p.GetID(), meta.DataID)
				break
			}

			var app *customer.Application
			err := utils.RetryOperation(func() error {
				var err error
				app, err = p.WorkerAdapter.FetchInformationOfProfile(meta.DataID, true)
				return err
			}, 3, 1)
			if err == nil {
				logger.Root.Infof("`%s` finishes processing app %s. Sending to processed queue", p.GetID(), meta.DataID)
				adapter.Crawled <- basecrawler.MetaCrawlingAppResult{
					Request: meta,
					App:     app,
				}
				logger.Root.Infof("Sent %s to processed queue", meta.DataID)
			}
			if err != nil || app == nil {
				logger.Root.Errorf("`%s` Coud not get the information of profile: %s %v", p.GetID(), meta.DataID, err)
				if strings.Contains(err.Error(), "NetworkError when attempting to fetch resource") || strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "invalid session id") {
					nedToStopBecauseOfError = true
					p.SetNeedToBeReplaced(true)
				}
				if err == ErrInvalidSession {
					p.SetNeedToBeReplaced(true)
				}
				logger.Root.Infof("Send application profile %s back to the queue", meta.DataID)
				adapter.NeedToCrawled <- meta
				adapter.AddNumNeedToCrawl(1)
			}
		default:
			break
		}
	}
	logger.Root.Infof("`%s` exit start crawling...", p.GetID)
}
