package basecrawler

import (
	"github.com/duc-thien-phong/techsharedservices/models"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	nsq_models "github.com/duc-thien-phong/techsharedservices/nsq/models"
)

type AccountType int

const (
	AccountTypeExtractData AccountType = iota // 0
	AccountTypeCheckData                      // 1
)

type IClientAdapter interface {
	GetWorkerSoftwareSource() customer.SoftwareSource

	// SetOS(os osinfo.OSType)
	Init()

	Crawl(config models.DataCrawlerConfig) ([]customer.Application, error)

	CreateCrawler(withDocker bool, args map[string]interface{}) (ICrawler, error)
	RemoveCrawler(id string)
	CreateChecker(withDocker bool, args map[string]interface{}) (IChecker, error)
	RemoveChecker(id string)

	CanStartCrawler() bool
	CanStartChecker() bool

	GetAChecker() IChecker
	GetACrawler() ICrawler
	GetNeedToCloseCrawlers() bool
	SetNeedToCloseCrawlers(v bool)
	GetNeedToCloseCheckers() bool
	SetNeedToCloseCheckers(v bool)

	CheckIfIsInCrawlerList(id string) bool
	CheckIfIsInCheckerList(id string) bool

	GetNumRunningCrawlers() int
	GetNumRunningCheckers() int

	// OnTunnelStarted() error
	// OnTunnelStopped() error
	// SetConfig(config *Config)

	// ExtractDataToday(config models.DataCrawlerConfig) ([]customer.Application, error)
	// ListenToCheck()
	AssignAccounts([]models.RemoteAccount) error
	HandleRequest(message nsq_models.Message) error

	UpdateAccountMessage(loginMessage string, username string, isActive bool, needToTerminate bool)

	// LoginExtractAccount() error
	// CheckAuthenticateOfExtractingAccount() bool

	// Login(accountType AccountType) error
	// Login(accountInfo *UsefulAccountInfo) error
	// CheckAuthenticate(accountInfo *UsefulAccountInfo) bool

	StartOrStopAllCrawlers(start bool)
	StartOrStopAllCheckers(start bool)

	GetHostApplication() *Application
	SetHostApplication(a *Application)

	Close() error
}
