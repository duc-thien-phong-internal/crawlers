package cicblib

import (
	"encoding/gob"
	"errors"
	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/softwareclient"
	nsqmodels "github.com/duc-thien-phong/techsharedservices/nsq/models"
	"os"
	"strings"
	"sync"
)

// Adapter is a CIC Adapter
type Adapter struct {
	basecrawler.BaseAdapter
	CurrentID  int
	StartingID int
	StopID     int

	wg sync.WaitGroup
}

// just a way to verify if Adapter implements all methods of IClientAdapter or not
var _ basecrawler.IClientAdapter = &Adapter{}

func (c *Adapter) HandleRequest(msg nsqmodels.Message) error {
	logger.Root.Infof("Inside function handle request of *Adapter")
	return (&c.BaseAdapter).HandleRequest(msg)
}

func (Adapter) GetWorkerSoftwareSource() softwareclient.AppNoType {
	return softwareclient.ApplicationCICB
}

func CreateAdapter() *Adapter {
	baseAdapter := basecrawler.NewAdapter(nil, nil, nil)
	//baseAdapter.CreateCrawlerFunc = baseAdapter.CreateCrawlerFunc

	baseAdapter.BeforeCrawlingFunc = func(args interface{}) (bool, error) {
		logger.Root.Infof("Need to implement BeforeCrawlingFunc of CICB")
		return false, nil
	}
	baseAdapter.AfterCrawlingFunc = func(args interface{}) (bool, error) {
		logger.Root.Infof("Need to implement AfterCrawlingFunc of CICB")
		return false, nil
	}

	adapter := &Adapter{
		BaseAdapter: *baseAdapter,
	}
	logger.Root.Infof("Set createCrawlerFunc, createCheckerFunc, stillHasPermissionFunc")
	adapter.BaseAdapter.CreateCrawlerFunc = adapter.CreateCrawler
	adapter.BaseAdapter.CreateCheckerFunc = adapter.CreateChecker
	adapter.BaseAdapter.StillHasPermissionFunc = func(errMessage string) bool {
		return !strings.Contains(errMessage, "do not match")
	}
	adapter.BaseAdapter.BeforeCrawlingFunc = adapter.beforeCrawling
	adapter.BaseAdapter.AfterCrawlingFunc = adapter.afterCrawling

	return adapter
}

func (c *Adapter) getIntValue(key string, obj *int) {
	if rawValue, ok := c.GetWorkerConfig().OtherConfig[key]; ok {
		logger.Root.Infof("Get value of `%s` as %v (%T)", key, rawValue, rawValue)
		switch v := rawValue.(type) {
		case float64:
			*obj = int(v)
			logger.Root.Infof("Set `%s` to %v", key, rawValue)
		case int:
			*obj = v
			logger.Root.Infof("Set `%s` to %v", key, rawValue)
		case int64:
			*obj = int(v)
			logger.Root.Infof("Set `%s` to %v", key, rawValue)
		}
	}
}

func (c *Adapter) beforeCrawling(args interface{}) (canContinue bool, error error) {
	logger.Root.Infof("Nothing to do before crawling")
	c.getIntValue("startingID", &c.StartingID)
	c.getIntValue("stoppingID", &c.StopID)
	return true, nil
}

func (c *Adapter) afterCrawling(args interface{}) (canContinue bool, error error) {
	if c.CurrentID == 0 {
		return true, nil
	}
	c.StartingID = c.CurrentID - 1
	file, _ := os.Create("lastid")

	defer file.Close()

	encoder := gob.NewEncoder(file)

	encoder.Encode(c.CurrentID)

	return true, nil
}

func (c *Adapter) CreateCrawler(withDocker bool, args map[string]interface{}) (basecrawler.ICrawler, error) {
	logger.Root.Infof("Create crawler from cicb adapter")
	crawler := NewCrawler(withDocker, args)
	if crawler == nil || crawler.Browser == nil {
		if withDocker {
			return nil, errors.New("Could not create browser with docker")
		} else {
			return nil, errors.New("Could not create browser without docker")
		}
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
	//err = utils.RetryOperation(func() error {
	//	return crawler.GoToReportPage()
	//}, 3, 3)
	//if err != nil {
	//	logger.Root.Errorf("Error when go to Report: %v", err)
	//	crawler.Close()
	//	return nil, err
	//}
	logger.Root.Infof("New crawler created")

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
	//err = utils.RetryOperation(func() error {
	//	return checker.GoToReportPage()
	//}, 3, 3)
	//if err != nil {
	//	logger.Root.Errorf("Error when go to Report: %v", err)
	//	checker.Close()
	//	return nil, err
	//}

	logger.Root.Infof("New checker created")

	return checker, nil
}

// Close closes the client
func (c *Adapter) Close() error {
	logger.Root.Infof("call to adaper Close")
	return c.BaseAdapter.Close()
}
