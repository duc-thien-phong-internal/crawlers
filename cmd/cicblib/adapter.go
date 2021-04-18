package cicblib

import "C"
import (
	"errors"
	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	"sync"
)

// Adapter is a CIC Adapter
type Adapter struct {
	//*basecrawler.Application
	basecrawler.BaseAdapter

	wg sync.WaitGroup
}

// just a way to verify if Adapter implements all methods of IClientAdapter or not
var test basecrawler.IClientAdapter = &Adapter{}

func (Adapter) GetWorkerSoftwareSource() customer.SoftwareSource {
	return customer.SoftwareCIB
}

func CreateAdapter() *Adapter {
	baseAdapter := basecrawler.NewAdapter(nil, nil, nil)
	//baseAdapter.CreateCrawlerFunc = baseAdapter.CreateCrawlerFunc

	baseAdapter.BeforeCrawlingFunc = func(args interface{}) {
		logger.Root.Infof("Need to implement BeforeCrawlingFunc of CICB")
	}
	baseAdapter.AfterCrawlingFunc = func(args interface{}) {
		logger.Root.Infof("Need to implement AfterCrawlingFunc of CICB")
	}

	adapter := &Adapter{
		BaseAdapter: *baseAdapter,
	}
	logger.Root.Infof("Set createCrawlerFunc, createCheckerFunc, stillHasPermissionFunc")
	adapter.BaseAdapter.CreateCrawlerFunc = adapter.CreateCrawler
	adapter.BaseAdapter.CreateCheckerFunc = adapter.CreateChecker
	adapter.BaseAdapter.StillHasPermissionFunc = func(message string) bool {
		return true
	}

	return adapter
}

func (c *Adapter) CreateCrawler(withDocker bool, args map[string]interface{}) (basecrawler.ICrawler, error) {
	logger.Root.Infof("Create crawler from cicb adapter")
	crawler := NewCrawler(withDocker)
	if withDocker && (crawler == nil || crawler.Browser == nil) {
		return nil, errors.New("Could not create browser with docker")
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
	checker := NewChecker(withDocker)
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
