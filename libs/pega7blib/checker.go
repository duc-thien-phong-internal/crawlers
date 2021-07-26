package pega7lib

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
	Base
}

func NewChecker(withDocker bool, args map[string]interface{}) *Checker {
	p := Checker{}

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

func (p *Checker) Run(wg *sync.WaitGroup) {
	for {
		if p.GetNeedToBeReplaced() || p.NeedToBeClosed() || p.ClientAdapter.GetNeedToCloseCheckers() || !p.ClientAdapter.CheckIfIsInCheckerList(p.GetID()) {
			logger.Root.Infof("checker: `%s` needToBeReplace or needToClose", p.GetID())
			break
		}
		wg.Add(1)
		// p.adapter.chanCrawlers <- struct{}{}
		p.startChecking(wg)
		if p.GetNeedToBeReplaced() || p.NeedToBeClosed() || p.ClientAdapter.GetNeedToCloseCheckers() || !p.ClientAdapter.CheckIfIsInCheckerList(p.GetID()) {
			logger.Root.Infof("checker: `%s` needToBeReplace or needToClose", p.GetID())
			break
		}
	}
	p.ClientAdapter.RemoveChecker(p.GetID())
	p.Close()
}

func (p *Checker) startChecking(wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = '"+p.GetID()+"'", []interface{}{})
	}()
	defer func() {
		if r := recover(); r != nil {
			logger.Root.Infof("Recover from startChecking of `%s`. Err: %v", p.GetID(), r)
		}
	}()
	(*p.Browser.DriverInfo.Driver).ExecuteScript("document.title = 'Checker - "+p.GetID()+"'", []interface{}{})
	// defer func() {
	// 	p.adapter.chanCrawlers <- struct{}{}
	// }()
	logger.Root.Infof("Browser `%s` (idx:%d) start checking....", p.GetID(), p.ClientAdapter.GetNumRunningCheckers())
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
			logger.Root.Infof("Set checker `%s` to be replaced", p.GetID())
			break mainFor
		case meta := <-adapter.NeedToChecked:
			if meta.DataID == "" {
				break mainFor
			}

			logger.Root.Infof("Browser `%s` checks cicb %s ", p.GetID(), meta.DataID)

			var app *customer.Application
			err := utils.RetryOperation(func() error {
				var err error
				app, err = p.FetchInformationOfProfile(meta.DataID, true)
				return err
			}, 3, 1)
			if err == nil {
				logger.Root.Infof("`%s` finishes processing app %s. Sending to processed queue", p.GetID(), meta.DataID)
				adapter.Checked <- basecrawler.MetaCheckingAppResult{
					Request:        meta,
					App:            app,
					CommandID:      meta.CommandID,
					CommandSubType: meta.CommandSubType,
					Channel:        meta.Channel,
				}
				logger.Root.Infof("`%s` sent check result of %s to processed queue", p.GetID(), meta.DataID)
			}

			if err != nil || app == nil {
				logger.Root.Errorf("`%s` Coud not get the information of cicb: %s %v", p.GetID(), meta.DataID, err)
				if strings.Contains(err.Error(), "NetworkError when attempting to fetch resource") || strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "invalid session id") {
					nedToStopBecauseOfError = true
					p.SetNeedToBeReplaced(true)
				}
				if err == ErrInvalidSession {
					p.SetNeedToBeReplaced(true)
				}
				logger.Root.Infof("`%s` sends check req of application `%s` back to the queue", p.GetID(), meta.DataID)
				adapter.NeedToChecked <- meta
			}
		default:
			break
		}
	}
	logger.Root.Infof("`%s` exit start checking...", p.GetID())
}
