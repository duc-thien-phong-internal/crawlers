package basecrawler

import (
	"github.com/duc-thien-phong/techsharedservices/models"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	nsq_models "github.com/duc-thien-phong/techsharedservices/nsq/models"
	"math/rand"
)

type MockClient struct{}

func (MockClient) GetWorkerSoftwareSource() customer.SoftwareSource { return customer.SoftwareUnknown }

// SetOS(os osinfo.OSType)
func (MockClient) Init(a *Application) {}
func (MockClient) Crawl(config models.DataCrawlerConfig) ([]customer.Application, error) {
	return nil, nil
}

func (MockClient) CreateCrawler(withDocker bool, args map[string]interface{}) (ICrawler, error) {
	return nil, nil
}
func (MockClient) CreateChecker(withDocker bool, args map[string]interface{}) (IChecker, error) {
	return nil, nil
}

func (MockClient) CanStartCrawler() bool      { return rand.Intn(100)%3 == 0 }
func (MockClient) CanStartChecker() bool      { return rand.Intn(100)%3 == 0 }
func (MockClient) GetAChecker() IChecker      { return nil }
func (MockClient) GetACrawler() ICrawler      { return nil }
func (MockClient) GetNumRunningCrawlers() int { return rand.Intn(3) }
func (MockClient) GetNumRunningCheckers() int { return rand.Intn(3) }

func (MockClient) AssignAccounts([]models.RemoteAccount) error    { return nil }
func (MockClient) HandleRequest(message nsq_models.Message) error { return nil }

func (MockClient) StartOrStopAllCrawlers(start bool) {}
func (MockClient) StartOrStopAllCheckers(start bool) {}

func (MockClient) Close() error { return nil }

//
//func TestCoreClient_Start(t *testing.T) {
//	defer goleak.VerifyNone(t)
//	type fields struct {
//		cookieJarServer             *cookiejar.Jar
//		AuthorizationToken          string
//		LastTimeRefreshToken        time.Time
//		crawledIDNos                map[string]bool
//		cachedIdsFile               string
//		config                      Config
//		client                      IClientAdapter
//		conn                        *websocket.Conn
//		resultMap                   map[commands.CommandID]chan string
//		connectedToServer           bool
//		onClosing                   func()
//		tunnelStarted               bool
//		needToExit                  bool
//		fetchedConfigFromServer     bool
//		stopTunnelFunc              func()
//		lastTimeSendingPing         time.Time
//		lastTimeGetConfigFromServer time.Time
//		signalInterruption          chan bool
//		send                        chan PretendMsg
//		errorChan                   chan error
//		numErrorWhenCommunicating   int
//		needToStartCrawler          bool
//		needToStartChecker          bool
//		Mutex                       sync.Mutex
//	}
//	type args struct {
//		client        IClientAdapter
//		cachedIdsFile string
//		debugMode     bool
//		onClosing     func()
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		{
//			name:   "go routine leak detection",
//			fields: fields{},
//			args: args{
//				client:        MockClient{},
//				cachedIdsFile: "mock_test",
//				debugMode:     true,
//				onClosing:     func() {},
//			},
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			mc := &Application{
//				cookieJarServer:             tt.fields.cookieJarServer,
//				AuthorizationToken:          tt.fields.AuthorizationToken,
//				LastTimeRefreshToken:        tt.fields.LastTimeRefreshToken,
//				crawledIDNos:                tt.fields.crawledIDNos,
//				cachedIdsFile:               tt.fields.cachedIdsFile,
//				config:                      tt.fields.config,
//				client:                      tt.fields.client,
//				conn:                        tt.fields.conn,
//				resultMap:                   tt.fields.resultMap,
//				connectedToServer:           tt.fields.connectedToServer,
//				onClosing:                   tt.fields.onClosing,
//				tunnelStarted:               tt.fields.tunnelStarted,
//				needToExit:                  tt.fields.needToExit,
//				fetchedConfigFromServer:     tt.fields.fetchedConfigFromServer,
//				stopTunnelFunc:              tt.fields.stopTunnelFunc,
//				lastTimeSendingPing:         tt.fields.lastTimeSendingPing,
//				lastTimeGetConfigFromServer: tt.fields.lastTimeGetConfigFromServer,
//				signalInterruption:          tt.fields.signalInterruption,
//				send:                        tt.fields.send,
//				errorChan:                   tt.fields.errorChan,
//				numErrorWhenCommunicating:   tt.fields.numErrorWhenCommunicating,
//				needToStartCrawler:          tt.fields.needToStartCrawler,
//				needToStartChecker:          tt.fields.needToStartChecker,
//				Mutex:                       tt.fields.Mutex,
//			}
//			mc.cachedIdsFile = tt.args.cachedIdsFile
//			mc.signalInterruption = make(chan bool, 1)
//			mc.setTunnelState(false)
//			mc.SetNeedToStopGettingData(false)
//			mc.errorChan = make(chan error, 1000)
//			mc.send = make(chan PretendMsg, 1000)
//			tt.args.client.Init(mc)
//
//			mc.client = tt.args.client
//			// mc.Start(tt.args.client, tt.args.cachedIdsFile, tt.args.debugMode, tt.args.onClosing)
//			mc._startGettingData()
//		})
//	}
//}
