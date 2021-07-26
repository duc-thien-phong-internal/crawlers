package pega7lib

import (
	"encoding/json"
	"errors"
	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/softwareclient"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"github.com/golang/glog"
	"strings"
	"sync"
)

// Adapter is a Pega7 Adapter
type Adapter struct {
	basecrawler.BaseAdapter

	wg sync.WaitGroup
}

// just a way to verify if Adapter implements all methods of IClientAdapter or not
var _ basecrawler.IClientAdapter = &Adapter{}

func (Adapter) GetWorkerSoftwareSource() softwareclient.AppNoType {
	return softwareclient.ApplicationPega7
}

func CreateAdapter() *Adapter {
	baseAdapter := basecrawler.NewAdapter(nil, nil, nil)

	adapter := &Adapter{
		BaseAdapter: *baseAdapter,
	}
	logger.Root.Infof("Set createCrawlerFunc, createCheckerFunc, stillHasPermissionFunc")
	adapter.BaseAdapter.CreateCrawlerFunc = adapter.CreateCrawler
	adapter.BaseAdapter.CreateCheckerFunc = adapter.CreateChecker
	adapter.BaseAdapter.StillHasPermissionFunc = func(message string) bool {
		return !strings.Contains(message, "not recognized")
	}
	adapter.BaseAdapter.BeforeCrawlingFunc = adapter.beforeCrawling
	adapter.BaseAdapter.AfterCrawlingFunc = adapter.afterCrawling

	return adapter
}

// before crawling, we need to go to the report, set the time, product options... first
func (c *Adapter) beforeCrawling(args interface{}) (canContinue bool, err error) {
	productOptions := ""
	if d, ok := c.GetWorkerConfig().OtherConfig["productOptions"]; ok {
		if v, ok := d.(string); ok && len(strings.Trim(v, " \n\t")) > 0 {
			productOptions = v
		}
	}
	// p.setTheStartDayToCrawl(`"CRC","FC_CDL","FC_PL","FC_PL     ","FC_PL_X"`)
	if c.Producer == nil {
		glog.Infof("Stop crawling because the producer is nil")
		return false, ErrNoProducer
	}
	if curURL, err := (*c.Producer.(*Crawler).Browser.DriverInfo.Driver).CurrentURL(); err != nil {
		glog.Errorf("\n\nCould not get the current URL of the producer. Error: %v\n", err)
		c.Producer.Close()
		c.AssignNewProducer()
		return false, nil
	} else {
		if !strings.Contains(curURL, "NUMBEROFCURRENTAPPLICATIONSBYSTAGE") {
			// (*p.producer.Browser.DriverInfo.Driver).Get()
			// p.producer.Login()
			// err := utils.RetryOperation(func() error {
			// 	return p.producer.Login()
			// }, 3, 5)
			err := utils.RetryOperation(func() error {
				return c.Producer.(*Crawler).GoToReportPage()
			}, 3, 5)

			if err != nil {
				glog.Infof("\n!!! the current producer `%s` could not go to the report page. %v\n\n", c.Producer.GetID(), err)
				c.Producer.Close()
				c.Producer = nil
				// p.assignNewProducer()
				return false, err
			}
		}
	}
	if err := c.Producer.(*Crawler).setTheStartDayToCrawl(productOptions); err != nil {
		c.Producer.Close()
		c.RemoveCrawler(c.Producer.GetID())
		return false, err
	}
	return true, nil
}

// after crawling, no need to do anything
func (c *Adapter) afterCrawling(args interface{}) (canContinue bool, err error) {
	return true, nil
}

func (c *Adapter) CreateCrawler(withDocker bool, args map[string]interface{}) (basecrawler.ICrawler, error) {
	logger.Root.Infof("Create crawler from cicb adapter")
	crawler := NewCrawler(withDocker, args)
	if crawler == nil || crawler.Browser == nil {
		return nil, errors.New("Could not create browser")
	}

	crawler.WorkerAdapter = crawler
	crawler.ClientAdapter = c
	crawler.AllAvailableAccounts = basecrawler.Shuffle(c.AccountsExtractingData)

	err := crawler.Login()
	if err != nil {
		logger.Root.Errorf("Error when login: %v", err)
		crawler.Close()
		return nil, err
	}
	err = utils.RetryOperation(func() error {
		return crawler.GoToReportPage()
	}, 3, 3)
	if err != nil {
		logger.Root.Errorf("Error when go to Report: %v", err)
		crawler.Close()
		return nil, err
	}
	logger.Root.Infof("New crawler created")

	if cookies, err := (*crawler.Browser.DriverInfo.Driver).GetCookies(); err == nil {
		//cookiesStr, err := json.Marshal(cookies)
		json.Marshal(cookies)
	} else {
		logger.Root.Errorf("Could not get cookies in crawler %s. %v", crawler.GetID(), err)
	}

	return crawler, nil
}

func (c *Adapter) CreateChecker(withDocker bool, args map[string]interface{}) (basecrawler.IChecker, error) {
	checker := NewChecker(withDocker, args)
	if checker == nil {
		return nil, errors.New("Could not create browser")
	}

	if withDocker && checker.Browser == nil {
		return nil, errors.New("Could not create browser with docker")
	}

	checker.WorkerAdapter = checker
	checker.ClientAdapter = c
	checker.AllAvailableAccounts = basecrawler.Shuffle(c.AccountsCheckingData)

	err := checker.Login()
	if err != nil {
		logger.Root.Errorf("Error when login: %v", err)
		checker.Close()
		return nil, err
	}
	err = utils.RetryOperation(func() error {
		return checker.GoToReportPage()
	}, 3, 3)
	if err != nil {
		logger.Root.Errorf("Error when go to Report: %v", err)
		checker.Close()
		return nil, err
	}

	logger.Root.Infof("New checker created")

	return checker, nil
}
