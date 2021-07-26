package basecrawler

import (
	"github.com/duc-thien-phong-internal/crawlers/pkg/my_selenium"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	"math/rand"
	"sync"
	"time"
)

type WorkerAdapterInterface interface {
	// login using a given account
	LoginWithAccount(currentAccount *UsefulAccountInfo) (errMessage string)
	// check if we are still authorized
	CheckAuthenticate() (bool, error)
	// fetch information of a customer application using a given data id
	FetchInformationOfProfile(dataID string, fetchFull bool) (*customer.Application, error)
	// check if after login, we still has permission to do task with this account based on login result message
	StillEnoughPermission(lastMessage string) bool
	// update the login message of an account to our database
	//UpdateAccountMessage(loginMessage string, username string, isActive bool, needToTerminate bool)
	// Get config of the worker (from database)
	//GetWorkerConfig() *models.DataClientConfig
}

// Worker is the base class for producer/crawler/checker
type Worker struct {
	// in case we need to use a browser as behalf of this worker
	Browser   *my_selenium.Browser
	CookieStr string

	sync.Mutex
	//
	CurrentAccount         *UsefulAccountInfo
	AllAvailableAccounts   []*UsefulAccountInfo
	dataExtractingAccounts []*UsefulAccountInfo
	dataCheckingAccounts   []*UsefulAccountInfo
	//
	LastAccountByUsername map[string]models.RemoteAccount
	//
	NeedToClose      bool
	needToBeReplaced bool
	isInitializing   bool
	initializingTime uint64
	//
	isBusy   bool
	isActive bool
	//
	id string

	LoginURL string

	//Adapter IClientAdapter
	// with different providers, and worker type (worker, checker, producer...)
	// we can have different ways to login or to check authentication
	// --> it is better if we can plug the worker adapter here according to our need
	WorkerAdapter WorkerAdapterInterface // for checker/crawler

	ClientAdapter IClientAdapter // for provider
}

// SetNeedToClose mark the worker to be closed
func (p *Worker) SetNeedToClose(value bool) {
	p.NeedToClose = value
}

// SetNeedToClose mark the worker to be closed
func (p *Worker) SetNeedToBeReplaced(value bool) {
	p.needToBeReplaced = value
}

func (p *Worker) CheckStopRequestAndClose(actionImmediately bool) bool {
	return false
}

func (p *Worker) IsActive() bool {
	return p.isActive
}

func (p *Worker) IsBusy() bool {
	return p.isBusy
}
func (p *Worker) SetActive(v bool) {
	p.isActive = v
}
func (p *Worker) SetBusy(v bool) {
	p.isBusy = v
}

func (p *Worker) NeedToBeReset() bool {
	return p.needToBeReplaced
}
func (p Worker) GetNeedToBeReplaced() bool {
	return p.needToBeReplaced
}

func (p *Worker) NeedToBeClosed() bool {
	return p.NeedToClose
}

func (p *Worker) Prepare() error {
	return nil
}

func (p *Worker) SetAvailableAccounts(accs []*UsefulAccountInfo) {
	p.AllAvailableAccounts = accs
}

func Shuffle(accs []*UsefulAccountInfo) []*UsefulAccountInfo {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(accs), func(i, j int) {
		accs[i], accs[j] = accs[j], accs[i]
	})
	return accs
}

func (p *Worker) Login() error {

	// logger.Root.Infof("all available accounts: %#v", p.allAvailableAccounts)
	logger.Root.Infof("Try to login with accounts:")
	for i, a := range p.AllAvailableAccounts {
		logger.Root.Infof("Acc %d: %s %v %s\n", i, a.Account.Username, a.Account.Active, a.Account.LastMessage)
	}
	accs := Shuffle(p.AllAvailableAccounts)
	for _, acc := range accs {
		if !acc.Account.Active {
			continue
		}
		p.CurrentAccount = acc
		logger.Root.Infof("\u1F464 Trying to login with `%s`", acc.Account.Username)

		errMessage := p.WorkerAdapter.LoginWithAccount(acc)
		if errMessage != "" {
			// refresh page for the next try
			// (*p.Browser.DriverInfo.Driver).Refresh()
			time.Sleep(500 * time.Millisecond)
			stillActive := p.WorkerAdapter.StillEnoughPermission(errMessage)
			acc.Account.Active = stillActive
			if !stillActive {
				logger.Root.Infof("!!!!!!!! Set account `%s` to inactive", acc.Account.Username)
				for i, a := range p.AllAvailableAccounts {
					logger.Root.Infof(" after inactive Acc %d: %s %v %s\n", i, a.Account.Username, a.Account.Active, a.Account.LastMessage)
				}
			}
			if p.WorkerAdapter != nil {
				p.ClientAdapter.UpdateAccountMessage(errMessage+" when login", acc.Account.Username, stillActive, false)
			} else {
				logger.Root.Warn("Did not set the adapter for the crawler")
			}
		} else {
			acc.LastTimeLogin = time.Now()
			logger.Root.Infof("Logging in user %s OK!\n", acc.Account.Username)
			if p.WorkerAdapter != nil {
				p.ClientAdapter.UpdateAccountMessage("Login OK", acc.Account.Username, true, false)
			} else {
				logger.Root.Warn("Did not set the adapter for the checker")
			}
			return nil
		}
	}

	return ErrNoUsableAccount
}

func (p *Worker) Close() error {
	if p.Browser != nil {
		p.Browser.Close()
	}

	return nil
}

func (p *Worker) SetDataExtractingAccounts(infos []*UsefulAccountInfo) {
	p.dataExtractingAccounts = infos
}

func (p Worker) GetDataExtractingAccounts() []*UsefulAccountInfo {
	return p.dataExtractingAccounts
}

func (p *Worker) SetCheckingAccounts(infos []*UsefulAccountInfo) {
	p.dataCheckingAccounts = infos
}

func (p Worker) GetCheckingAccounts() []*UsefulAccountInfo {
	return p.dataCheckingAccounts
}

func (p Worker) GetID() string {
	return p.id
}

func (p *Worker) SetID(id string) {
	p.id = id
}
