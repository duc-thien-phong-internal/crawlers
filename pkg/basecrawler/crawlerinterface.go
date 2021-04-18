package basecrawler

import (
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	"sync"
)

// WorkerType is type of a worker, can be checker or crawler
type WorkerType int

const (
	// TypeCrawler is a crawler
	TypeCrawler WorkerType = iota
	// TypeChecker is checker
	TypeChecker
)

// IBaseWorker defines signature of a worker
// a worker can be a checker or a crawler
type IBaseWorker interface {
	SetNeedToClose(value bool)
	// SetIsInitializing(value bool)
	// SetInitializingTime(time int64)
	// SetType(WorkerType)

	SetAvailableAccounts([]*UsefulAccountInfo)
	SetDataExtractingAccounts([]*UsefulAccountInfo)
	GetDataExtractingAccounts() []*UsefulAccountInfo
	SetCheckingAccounts([]*UsefulAccountInfo)
	GetCheckingAccounts() []*UsefulAccountInfo

	CheckStopRequestAndClose(actionImmediately bool) bool

	IsActive() bool
	IsBusy() bool
	SetActive(bool)
	SetBusy(bool)

	NeedToBeReset() bool
	GetNeedToBeReplaced() bool
	SetNeedToBeReplaced(v bool)
	NeedToBeClosed() bool

	// get id of the crawler which can be used for management purpose
	GetID() string
	SetID(id string)

	// Prepare the environment
	Prepare() error

	Run(wg *sync.WaitGroup)

	FetchInformationOfProfile(cicCode string, fetchFull bool) (*customer.Application, error)

	// Login to the other company server
	Login() error
	Close() error
}

// ICrawler presents the signature of a crawler
type ICrawler interface {
	IBaseWorker

	ProcessPage(config *Config, pageSize int) (dataIDs []string, hasNextPage bool, err error)

	// Crawl(config models.DataCrawlerConfig) ([]customer.Application, error)
}

// IChecker presents the signature of a checker
type IChecker interface {
	IBaseWorker
}
