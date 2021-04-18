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

type Checker struct {
	id string
	Base
}

func NewChecker(withDocker bool) *Checker {
	p := Checker{}

	browser := my_selenium.CreateBrowser(withDocker, 24*60*60, 24*60*60)
	if browser == nil {
		logger.Root.Errorf("Could not create selenium browser")
		return nil
	} else {
		logger.Root.Infof("Browser is created!")
	}
	p.Browser = browser

	return &p
}

func (p *Checker) Run(wg *sync.WaitGroup) {
	for {
		if p.GetNeedToBeReplaced() || p.NeedToBeClosed() || p.ClientAdapter.GetNeedToCloseCheckers() || !p.ClientAdapter.CheckIfIsInCheckerList(p.id) {
			logger.Root.Infof("checker: `%s` needToBeReplace or needToClose", p.id)
			break
		}
		wg.Add(1)
		// p.adapter.chanCrawlers <- struct{}{}
		p.startChecking(wg)
		if p.GetNeedToBeReplaced() || p.NeedToBeClosed() || p.ClientAdapter.GetNeedToCloseCheckers() || !p.ClientAdapter.CheckIfIsInCheckerList(p.id) {
			logger.Root.Infof("checker: `%s` needToBeReplace or needToClose", p.id)
			break
		}
	}
	p.ClientAdapter.RemoveChecker(p.id)
	p.Close()
}

func (p *Checker) startChecking(wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = '"+p.id+"'", []interface{}{})
	}()
	defer func() {
		if r := recover(); r != nil {
			logger.Root.Infof("Recover from startChecking of `%s`. Err: %v", p.id, r)
		}
	}()
	(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = 'Checker - "+p.id+"'", []interface{}{})
	// defer func() {
	// 	p.adapter.chanCrawlers <- struct{}{}
	// }()
	logger.Root.Infof("Browser `%s` (idx:%d) start checking....", p.id, p.ClientAdapter.GetNumRunningCheckers())
	timer := time.NewTimer(time.Duration(
		int(p.ClientAdapter.GetHostApplication().GetConfig().WorkerConfigs.Checker.MaxLifeDuration)+
			rand.Intn(10*60)) * time.Second)
	var nedToStopBecauseOfError bool
	adapter, _ := p.ClientAdapter.(*Adapter)
mainFor:
	for !nedToStopBecauseOfError && !p.GetNeedToBeReplaced() && !p.NeedToBeClosed() && !p.ClientAdapter.GetNeedToCloseCheckers() {
		select {
		case <-timer.C:
			p.SetNeedToBeReplaced(true)
			timer.Stop()
			logger.Root.Infof("Set checker `%s` to be replaced", p.id)
			break mainFor
		case meta := <-adapter.NeedToChecked:
			if meta.DataID == "" {
				break mainFor
			}

			logger.Root.Infof("Browser `%s` checks cicb %s ", p.id, meta.DataID)

			var app *customer.Application
			err := utils.RetryOperation(func() error {
				var err error
				app, err = p.FetchInformationOfProfile(meta.DataID, true)
				return err
			}, 3, 1)
			if err == nil {
				logger.Root.Infof("`%s` finishes processing app %s. Sending to processed queue", p.id, meta.DataID)
				adapter.Checked <- basecrawler.MetaCheckingAppResult{
					Request:        meta,
					App:            app,
					CommandID:      meta.CommandID,
					CommandSubType: meta.CommandSubType,
				}
				logger.Root.Infof("`%s` sent check result of %s to processed queue", p.id, meta.DataID)
			}

			if err != nil || app == nil {
				logger.Root.Errorf("`%s` Coud not get the information of cicb: %s %v", p.id, meta.DataID, err)
				if strings.Contains(err.Error(), "NetworkError when attempting to fetch resource") || strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "invalid session id") {
					nedToStopBecauseOfError = true
					p.SetNeedToBeReplaced(true)
				}
				if err == ErrInvalidSession {
					p.SetNeedToBeReplaced(true)
				}
				logger.Root.Infof("`%s` sends check req of application `%s` back to the queue", p.id, meta.DataID)
				adapter.NeedToChecked <- meta
			}
		default:
			break
		}
	}
	logger.Root.Infof("`%s` exit start checking...", p.id)
}
