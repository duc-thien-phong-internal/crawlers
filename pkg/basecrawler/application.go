package basecrawler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/duc-thien-phong-internal/crawlers/pkg/osinfo"
	"github.com/duc-thien-phong/techsharedservices/commandmanager"
	"github.com/duc-thien-phong/techsharedservices/commands"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	nsq_models "github.com/duc-thien-phong/techsharedservices/nsq/models"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"runtime/debug"

	"gopkg.in/yaml.v2"
)

type Application struct {
	NetworkManager NetworkManager

	sync.Mutex
	crawledIDNos  map[string]bool
	cachedIdsFile string
	//
	config Config
	client IClientAdapter
	//
	resultMap map[commands.CommandID]chan string
	//
	onClosing          func()
	needToExit         bool
	stopTunnelFunc     func()
	signalInterruption chan bool
	//
	send                      chan PretendMsg
	errorChan                 chan error
	numErrorWhenCommunicating int
	//
	needToStartCrawler  bool
	needToStartChecker  bool
	lastTimeSendingPing time.Time
}

func NewApplication(
	checkConnFunc func() ConnectionState,
	openTunnelFunc func() error,
	closeFunnelFunc func() error,
	workerSoftwareSource customer.SoftwareSource,
) *Application {
	app := Application{}
	app.loadConfigFromDisk()
	app.NetworkManager = *CreateNetworkManager(&app, checkConnFunc, openTunnelFunc, closeFunnelFunc, workerSoftwareSource)
	return &app
}

type registerResult struct {
	Body      string    `json:"body" bson:"body"`
	Channel   string    `json:"Channel" bson:"Channel"`
	FromUser  string    `json:"fromUser" bson:"Channel"`
	ToUser    string    `json:"toUser" bson:"toUser"`
	TimeStamp time.Time `json:"timestamp" bson:"timestamp"`
}

// SetOS sets the operating system
func (mc *Application) SetOS(os osinfo.OSType) {
	// mc.OperatingSystem = os
}

func (mc *Application) GetWorkerConfig() *models.DataClientConfig {
	return &mc.config.WorkerConfigs
}

// func (mc *Crawler) GetCurrentStartingDate() time.Time {
// 	return mc.config.CurentStartingCrawlingTime
// }

func (mc *Application) GetCurrentCrawlingPage() int {
	return mc.config.CurrentCrawlingPage
}

func (mc *Application) GetConfig() *Config {
	return &mc.config
}

func (mc *Application) loadConfigFromDisk() {

	logger.Root.Infof(" Reading configuration...\n")

	c := *NewConfig()
	c.NsqConfig = mc.config.NsqConfig

	var curDir = getCurrentPath()
	if curDir == "" {
		curDir, _ = os.Getwd()
	}

	configPath := path.Join(curDir, configFileName)

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		logger.Root.Warnf(" Warning: There is an error when openning file '%s'. Error: %#v.\nWill use default configuration\n", configPath, err)

		if err2, ok := err.(*os.PathError); ok {
			fmt.Printf("err2 has type %T\n", err2.Error)
			if errno, ok := err2.Err.(syscall.Errno); ok {
				if int64(errno) == 0x2 {
					logger.Root.Infof("Create a default configuration file")
					mc.config = c
					mc.writeCurrentConfig()
				}
				fmt.Fprintf(os.Stderr, "errno=%d\n", int64(errno))
			}
		}
		mc.config = c
		return
	}
	defer file.Close()

	logger.Root.Infof(" Config path: %s\n", configPath)

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&c); err != nil {
		logger.Root.Errorf(" Error when decoding file '%s'. Error: %s\n", configPath, err)
	}

	mc.Lock()
	defer mc.Unlock()
	mc.config = c
	logger.Root.Infof("current config: %#v", mc.config)
}

func (mc *Application) writeCurrentConfig() error {
	b, err := yaml.Marshal(mc.config)
	if err != nil {
		return err
	}
	var curDir = getCurrentPath()
	if curDir == "" {
		curDir, _ = os.Getwd()
	}

	configPath := path.Join(curDir, configFileName)

	// _, err = os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY, 0644)
	err = ioutil.WriteFile(configPath, b, 0644)
	return err
}

//func (mc *Application) startTunnel() {
//	// breakTunnel := false
//
//	// doneTunnel := make(chan error)
//	onOK := func() {
//		// successChan <- true
//		mc.NetworkManager.setTunnelState(StateConnected)
//		logger.Root.Info("Tunnel started  OK .....")
//	}
//	onError := func() {
//		// logger.Root.Infoln("Tunnel could not be started.....")
//		mc.NetworkManager.setTunnelState(StateDisconnected)
//	}
//	sshTun := simplesshtun.New(mc.config.Tunnel.LocalPort, mc.config.Tunnel.Host, mc.config.Tunnel.RemotePort)
//	sshTun.SetUser(mc.config.Tunnel.Username)
//	if mc.config.Tunnel.PrivateKey != "" {
//		sshTun.SetKeyFile(mc.config.Tunnel.PrivateKey)
//	} else {
//		sshTun.SetPassword(mc.config.Tunnel.Password)
//	}
//	defer sshTun.Stop()
//
//	mc.stopTunnelFunc = func() {
//		logger.Root.Infof("Force stop the tunnel (maybe to restart it)")
//		// mc.setTunnelState(false)
//		sshTun.Stop()
//	}
//
//	go func() {
//		for {
//			if mc.config.Tunnel.Host != "" && mc.NetworkManager.isTunnelStarted() && !mc.canStartTunnel() {
//				logger.Root.Infof("Stop the tunnel\n")
//				sshTun.Stop()
//
//				mc.NetworkManager.setTunnelState(StateDisconnected)
//			}
//			time.Sleep(5 * time.Second)
//		}
//	}()
//
//	for {
//		// mc.loadConfigFromDisk()
//		logger.Root.Infof("Can start tunnel: %v Tunnel is started:%v\n", mc.canStartTunnel(), mc.NetworkManager.isTunnelStarted())
//		if mc.canStartTunnel() && !mc.NetworkManager.isTunnelStarted() {
//			logger.Root.Info("Starting the tunnel....")
//			// We want to connect to port 8080 on our machine to access port 80 on my.super.host.com
//
//			// We enable debug messages to see what happens
//			sshTun.SetDebug(true)
//			sshTun.SetPort(mc.config.Tunnel.SSHPort)
//
//			// We set a callback to know when the tunnel is ready
//			sshTun.SetConnState(func(tun *simplesshtun.SSHTun, state simplesshtun.ConnState) {
//				switch state {
//				case simplesshtun.StateStarting:
//					log.Printf("Tunnel is Starting")
//				case simplesshtun.StateStarted:
//					log.Printf("Tunnel is Started")
//					onOK()
//				case simplesshtun.StateStopped:
//					log.Printf("Tunnel is Stopped")
//					onError()
//				}
//			})
//
//			if err := sshTun.Start(); err != nil {
//				log.Printf("SSH tunnel stopped: %s", err.Error())
//			}
//		}
//
//		mc.NetworkManager.setTunnelState(StateDisconnected)
//		sshTun.Stop()
//
//		// mc.config.WorkerConfigs.Crawler.RequestedStatus = models.WorkerStopped
//		// mc.config.WorkerConfigs.Checker.RequestedStatus = models.WorkerStopped
//
//		// if mc.needToExit {
//		// 	logger.Root.Infoln(" Stop on request!")
//		// 	break
//		// }
//
//		logger.Root.Info(" Sleeping 3 seconds before next try to restart tunnel...")
//
//		time.Sleep(3 * time.Second)
//	}
//}

// var clientStarted = false
// var tunnelStarted bool

func (mc *Application) isInOfficeTime() bool {
	return mc.isInOfficeTimeOfCrawler() //|| mc.isInOfficeTimeOfChecker()
}
func (mc *Application) isInOfficeTimeOfCrawler() bool {
	today := utils.GetCurrentTime()
	nowInNumber := today.Hour()*100 + today.Minute()
	return (nowInNumber >= mc.config.WorkerConfigs.Crawler.StartHour && nowInNumber <= mc.config.WorkerConfigs.Crawler.StopHour) && (today.Weekday() != time.Sunday || mc.config.WorkerConfigs.Crawler.RunOnSunday) && (today.Weekday() != time.Saturday || mc.config.WorkerConfigs.Crawler.RunOnSaturday)
}
func (mc *Application) isInOfficeTimeOfChecker() bool {
	today := utils.GetCurrentTime()
	nowInNumber := today.Hour()*100 + today.Minute()
	return (nowInNumber >= mc.config.WorkerConfigs.Checker.StartHour && nowInNumber <= mc.config.WorkerConfigs.Checker.StopHour) && (today.Weekday() != time.Sunday || mc.config.WorkerConfigs.Checker.RunOnSunday) && (today.Weekday() != time.Saturday || mc.config.WorkerConfigs.Checker.RunOnSaturday)
}

func (mc *Application) CanStartCrawler() bool {

	if mc.needToExit {
		logger.Root.Infof("Should not start crawler because needToExit=true\n")
		return false
	}
	if mc.NetworkManager.stateConnectionToOurServer != StateConnected {
		return false
	}

	dataConfig := mc.config.WorkerConfigs

	if dataConfig.Crawler.RunningMode == models.RunningModeManually {
		return (dataConfig.Crawler.RequestedStatus == models.WorkerStopped ||
			dataConfig.Crawler.RequestedStatus == models.WorkerStopping) &&
			(dataConfig.Crawler.Status != models.WorkerStopped && dataConfig.Crawler.Status != models.WorkerStopping)
	}

	isInOfficeTime := mc.isInOfficeTimeOfCrawler()
	if !isInOfficeTime {
		logger.Root.Debugf("Should not start crawler because out of office time")
	}
	// if the status is Auto
	return mc.isInOfficeTimeOfCrawler() && !mc.needToExit
}
func (mc *Application) CanStartChecker() bool {

	if mc.needToExit {
		logger.Root.Infof("Should not start checker because needToExit=true\n")
		return false
	}
	if mc.NetworkManager.stateConnectionToOurServer != StateConnected {
		return false
	}
	dataConfig := mc.config.WorkerConfigs

	// if current status is not stopX, but the requested status is StopX
	if dataConfig.Checker.RunningMode == models.RunningModeManually {
		return (dataConfig.Checker.RequestedStatus == models.WorkerStopped ||
			dataConfig.Checker.RequestedStatus == models.WorkerStopping) &&
			(dataConfig.Checker.Status != models.WorkerStopped && dataConfig.Checker.Status != models.WorkerStopping)
	}

	// if the status is Auto
	return mc.isInOfficeTimeOfChecker() && !mc.needToExit
}

func (mc *Application) canStartTunnel() bool {
	// if in the office hour
	// or if is not in office hour, but it is allowed to send ping mesage
	return mc.CanStartCrawler() || mc.CanStartChecker() || (mc.config.WorkerConfigs.SendPing && mc.config.WorkerConfigs.SendPingOutOfOffice)
}

func (mc *Application) sendPingMessage() {
	needToGetAppConfig := false
	lastTimeUpdate := utils.GetCurrentTime().Unix()
	// if it is the first time we send the ping, our status is out of dated --> better to say NotChanged
	var currentCrawlerStatus models.WorkerStatusType = models.WorkerStopped
	var currentCheckerStatus models.WorkerStatusType = models.WorkerStopped

	if mc.NetworkManager.lastTimeGetConfigFromServer.Unix()+1*60*60 < utils.GetCurrentTime().Unix() {
		needToGetAppConfig = true
		lastTimeUpdate = mc.config.WorkerConfigs.UpdatedAt.Unix()
	}

	// after fetching the status from server
	// and that config is still "up to date"
	if mc.NetworkManager.fetchConfigFromServer && mc.NetworkManager.lastTimeGetConfigFromServer.Unix()+1*60*60 >= utils.GetCurrentTime().Unix() {
		currentCrawlerStatus = mc.config.WorkerConfigs.Crawler.Status
		currentCheckerStatus = mc.config.WorkerConfigs.Checker.Status
		lastTimeUpdate = utils.GetCurrentTime().Unix()
	}

	logger.Root.Infof("Sending ping. Need to get config=%v....\n", needToGetAppConfig)
	commandData := commands.CommandReqPingPong{
		Time:                 lastTimeUpdate,
		CurrentStatusCrawler: currentCrawlerStatus,
		CurrentStatusChecker: currentCheckerStatus,
		WorkerSource:         mc.client.GetWorkerSoftwareSource(),
		GetApplicationConfig: needToGetAppConfig,
	}

	message := commandmanager.CreateMessageCommand(nil, commandData, commands.TypeRequest, commands.SubTypePingPong, mc.config.NsqConfig.MyNsqID, mc.config.NsqConfig.SystemNsqID, mc.config.NsqConfig.MainChannel)

	// message.ConstructBody(string(appsInByte))
	go func() {
		err, res := mc.NetworkManager.SendMessage(message, 20)
		if err != nil {
			logger.Root.Infof("Sending ping message Fail: %s\n", err)
		} else {
			if res.Data != nil {
				dbByte, _ := json.Marshal(res.Data)
				var pingPongData commands.CommandRespPingPong
				if err := json.Unmarshal(dbByte, &pingPongData); err == nil && needToGetAppConfig {
					if pingPongData.ApplicationConfig != nil {
						logger.Root.Infof("Updating worker configuration....\n")

						mc.config.WorkerConfigs = *pingPongData.ApplicationConfig
						logger.Root.Infof("Last crawling date: %s", mc.config.WorkerConfigs.Crawler.LastCrawlingDate.Format(time.RFC1123))
						mc.NetworkManager.fetchConfigFromServer = true
						mc.compareRunningModesAndStartCrawlerAndChecker()
						mc.NetworkManager.lastTimeGetConfigFromServer = utils.GetCurrentTime()
					}
				}
			}
			logger.Root.Infof("Sending ping message OK\n")
			//mc.updateLastTimeSendingPing()
		}
	}()
	mc.updateLastTimeSendingPing()

}

func (mc *Application) handleTheError(err error) {
	if err == nil {
		return
	}
	if (strings.Contains(err.Error(), "An established connection was aborted by the software in your host machine") ||
		strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host")) &&
		mc.NetworkManager.isTunnelStarted() {
		logger.Root.Infof("Connection corrupted")
		mc.numErrorWhenCommunicating++

		if mc.stopTunnelFunc != nil && mc.numErrorWhenCommunicating >= 3 {
			logger.Root.Infof("The tunnel seems to be broken. Set isTunnelStarted to false in order to restart it")
			mc.stopTunnelFunc()
			mc.numErrorWhenCommunicating = 0
		}
		go func() {
			mc.errorChan <- err
		}()
		logger.Root.Infof("Notified the socket")
	}
}

func (mc *Application) updateLastTimeSendingPing() {
	logger.Root.Infof("Update last time sending ping....\n")
	mc.lastTimeSendingPing = utils.GetCurrentTime()
}

func (mc *Application) startSendPingMessageRoutine() {
	for {
		if mc.config.WorkerConfigs.SendPing && mc.canStartTunnel() &&
			mc.NetworkManager.isTunnelStarted() &&
			mc.NetworkManager.stateConnectionToOurServer == StateConnected && !mc.needToExit {
			if mc.config.NsqConfig.SystemNsqID != "" && mc.config.NsqConfig.MyNsqID != "" && mc.config.NsqConfig.MainChannel != "" {
				var timeGap int64
				if mc.isInOfficeTime() {
					// time.Sleep(time.Duration(rand.Intn(10)+mc.config.DataExtractingConfig.SendPingInterval) * time.Second)
					timeGap = rand.Int63n(10) + mc.config.WorkerConfigs.SendPingInterval
				} else {
					timeGap = rand.Int63n(10) + mc.config.WorkerConfigs.SendPingOutOfOfficeInterval
				}

				if mc.lastTimeSendingPing.Unix()+timeGap <= utils.GetCurrentTime().Unix() {
					mc.sendPingMessage()
				} else {
					// logger.Root.Info("No need to send Ping message. Last time send ping:%v timegap:%d\n", mc.lastTimeSendingPing, timeGap)
				}
			} else {
				logger.Root.Errorf(" Invalid configuration. hostNsqId:%s systemNsqId:%s\n", mc.config.NsqConfig.MyNsqID, mc.config.NsqConfig.SystemNsqID)
			}
		}

		time.Sleep(10 * time.Second)

	}
}

func (mc *Application) getCrawlingInterval() int {
	var crawlingInterval int = 7 * 60 // seconds
	if d, ok := mc.config.WorkerConfigs.OtherConfig["crawlingInterval"]; ok {
		logger.Root.Infof("Get value of `crawlingInterval` as %v (%T)", d, d)
		if v, ok := d.(float64); ok {
			crawlingInterval = int(v)
		}
	}
	return crawlingInterval

}

func (mc *Application) onFetchedAccounts(v EventValue) {
	if v != nil {
		result := v.([]models.RemoteAccount)
		if len(result) > 0 {
			if err := mc.client.AssignAccounts(result); err != nil {
				logger.Root.Errorf("Error when assigning accounts: %s\n", err)
				logger.Root.Infof("Set need to close=true when assigning accounts")

				mc.SetNeedToStopGettingData(true)
			} else {
				logger.Root.Infof("Assigning accounts ok")
			}
		} else {
			logger.Root.Infof("There is no account")
		}
	}
}

func (mc *Application) listenToEventConnectedToOurServer(eventChan chan EventValue) {
	go func() {
		for {
			select {
			case v := <-eventChan:
				if v != nil {
					result := v.(ConnectionState)
					if result == StateConnected {
						logger.Root.Infof("Connected To Our server")

					} else if result == StateDisconnected {
						logger.Root.Infof("Disconnected To Our server")
					}
				}
			}
		}
	}()
}

func (mc *Application) onConnectionToOurServerChanged(v EventValue) {
	logger.Root.Infof("Connection to our server changed: %s", v)
}

func (mc *Application) onReceivingRequestCommand(v EventValue) {
	logger.Root.Infof("Receiving command request")
	msg, ok := v.(nsq_models.Message)
	if !ok {
		logger.Root.Errorf("Wrong value for event handler onReceivingRequestCommand")
		return
	}

	switch msg.Command.SubType {
	case commands.SubTypeChangeAppStatus:
		logger.Root.Infof("Received message ChangeApppStatus")
		go mc.handleRequestChangingAppStatus(msg)
	case commands.SubTypeExecuteRemoteCommand:
		logger.Root.Infof("Received message ExecuteRemoteCommand")
		go mc.handleRequestRunCommand(msg)
	case commands.SubTypeChangeWorkerAppConfig:
		logger.Root.Infof("Received message ChangeAppConfig")
		go mc.handleRequestChangingAppConfig(msg)
	default:
		go mc.client.HandleRequest(msg)
	}
}

// Start runs the crawler client with some options
func (mc *Application) Start(client IClientAdapter, cachedIdsFile string, debugMode bool, onClosing func()) {
	mc.cachedIdsFile = cachedIdsFile
	mc.signalInterruption = make(chan bool, 1)

	mc.SetNeedToStopGettingData(false)
	mc.errorChan = make(chan error, 1000)
	mc.send = make(chan PretendMsg, 1000)

	mc.NetworkManager.RegisterEventHandler(EventFetchingAccountDone, mc.onFetchedAccounts)
	mc.NetworkManager.RegisterEventHandler(EventConnectionToOurServerChanged, mc.onConnectionToOurServerChanged)
	mc.NetworkManager.RegisterEventHandler(EventOnReceivingRequestCommand, mc.onReceivingRequestCommand)

	defer func() {
		if r := recover(); r != nil {
			logger.Root.Infof("Recover from core client start")
			logger.Root.Info(string(debug.Stack()))
		}
	}()

	client.SetHostApplication(mc)
	client.Init()
	//mc.loadConfigFromDisk()
	mc.config.WorkerConfigs.Crawler.RequestedStatus = models.WorkerStopped
	mc.config.WorkerConfigs.Checker.RequestedStatus = models.WorkerStopped
	mc.NetworkManager.setStateConnectionToServer(StateDisconnected)

	mc.client = client

	go mc.NetworkManager.Run()

	// go client.ListenToCheck()

	if mc.config.Tunnel.Host != "" {
		// waiting for the tunnel before register app with server
		for !mc.NetworkManager.isTunnelStarted() {
			logger.Root.Infof("Sleeping 3 second. Waiting for the tunnel started\n")
			time.Sleep(3 * time.Second)
		}
	}

	if !debugMode {
		mc.readCrawledIdNos()
	} else {
		mc.crawledIDNos = make(map[string]bool)
	}

	go mc.startSendPingMessageRoutine()
	go mc.startCrawlerStatusWatcher()
	go mc.startCheckerStatusWatcher()

	lastTimeFetchAccs := time.Time{}
	// the main for loop
	for {
		if mc.NetworkManager.stateConnectionToOurServer != StateConnected {
			time.Sleep(1 * time.Second)
			continue
		}
		if (mc.isInOfficeTimeOfCrawler() || mc.isInOfficeTimeOfChecker()) && time.Now().Unix()-lastTimeFetchAccs.Unix() >= int64((15*time.Minute).Seconds()) {
			mc.NetworkManager.FetchAccounts()
			lastTimeFetchAccs = time.Now()
		}
		mc._startGettingData()
		time.Sleep(5 * time.Second)

		// mc.loadConfigFromDisk()
	}
}

func (mc *Application) _startGettingData() {
	var errCrawler error
	var debugMode bool
	if d, ok := mc.config.WorkerConfigs.OtherConfig["debug"]; ok {
		if v, ok := d.(bool); ok {
			debugMode = v
		}
	}
	if d, ok := mc.config.WorkerConfigs.OtherConfig["exit"]; ok {
		if v, ok := d.(bool); ok && v {
			os.Exit(100)
		}
	}

	logger.Root.Infof("Starting function _startGettingData")
	if mc.CanStartChecker() {
		go func() {
			mc.client.GetAChecker()
			mc.config.WorkerConfigs.Checker.Status = models.WorkerRunning
			mc.client.StartOrStopAllCheckers(true)
			mc.adjustCheckerStatus(true)
		}()
	} else {
		logger.Root.Infof("Out of checker running time...\n")
		// if mc.config.WorkerConfigs.Checker.RunningMode == models.RunningModeAuto {
		logger.Root.Infof("Stopping checkers (if any)...")
		mc.client.StartOrStopAllCheckers(false)
		mc.adjustCheckerStatus(false)
		// }
	}

	canStartCrawler := mc.CanStartCrawler()
	if canStartCrawler {
		logger.Root.Infof("Can start tunnel")
		for !mc.NetworkManager.isTunnelStarted() {
			if mc.needToExit {
				break
			}
			logger.Root.Infof("Sleeping 3 second. Waiting for the tunnel started\n")
			time.Sleep(3 * time.Second)
		}

		for mc.NetworkManager.stateConnectionToOurServer != StateConnected {
			if mc.needToExit {
				break
			}
			logger.Root.Infof("Sleep 3 seconds to connect to server\n")
			time.Sleep(3 * time.Second)
		}
		if mc.needToExit {
			return
		}

		logger.Root.Infof("\n=========== STARTING TO GET DATA ===========\n\n")
		// mc.sendPingMessage()
		mc.config.WorkerConfigs.Crawler.Status = models.WorkerRunning
		mc.config.WorkerConfigs.Checker.Status = models.WorkerRunning

		mc.NetworkManager.RefreshToken(false)
		mc.client.StartOrStopAllCrawlers(true)
		mc.adjustCrawlerStatus(true)

		errCrawler = mc.startExtractingData()
		if errCrawler == ErrNoUsableAccount {
			if mc.NetworkManager.isTunnelStarted() {
				mc.NetworkManager.FetchAccounts()
			}
		} else if errCrawler == ErrMaintain {
			logger.Root.Infof("The website is in maintenance mode. Need to sleep 10 minutes more")
			time.Sleep(10 * time.Minute)
		}
	} else {
		logger.Root.Infof("Out of crawler running time...\n")
		// if mc.config.WorkerConfigs.Crawler.RunningMode == models.RunningModeAuto {
		logger.Root.Infof("Stopping crawler (if any)...")
		mc.client.StartOrStopAllCrawlers(false)
		mc.adjustCrawlerStatus(false)
		// }
	}

	if debugMode || !canStartCrawler || errCrawler == ErrCouldNotCreateService {
		sleepTime := rand.Intn(mc.getCrawlingInterval()) + 5
		logger.Root.Infof("Sleeping %d seconds before each phase of getting data (debug mode)...\n", sleepTime)
		time.Sleep(time.Duration(sleepTime) * time.Second)
	} else {
		sleepTime := rand.Intn(mc.getCrawlingInterval()) + 5
		logger.Root.Infof("Sleeping %d seconds before each phase of getting data...\n", sleepTime)
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
}

func (mc *Application) getNeedToStartCrawler() bool {
	mc.Lock()
	defer mc.Unlock()
	return mc.needToStartCrawler
}
func (mc *Application) setNeedToStartCrawler(value bool) {
	logger.Root.Infof("set needToStartCrawler = %v", value)
	mc.Lock()
	defer mc.Unlock()
	mc.needToStartCrawler = value
}

func (mc *Application) getNeedToStartChecker() bool {
	mc.Lock()
	defer mc.Unlock()
	return mc.needToStartChecker
}

func (mc *Application) setNeedToStartChecker(value bool) {
	logger.Root.Infof("set needToStartChecker = %v", value)
	mc.Lock()
	defer mc.Unlock()
	mc.needToStartChecker = value
}

func (mc *Application) startCrawlerStatusWatcher() {
	for {
		mc.adjustCrawlerStatus(mc.CanStartCrawler())

		needToStart := mc.getNeedToStartCrawler()
		if !needToStart && mc.client.GetNumRunningCrawlers() == 0 {
			mc.config.WorkerConfigs.Crawler.Status = models.WorkerStopped
			break
		}
		if needToStart && mc.client.GetNumRunningCrawlers() != 0 {
			mc.config.WorkerConfigs.Crawler.Status = models.WorkerRunning
			break
		}
		time.Sleep(5 * time.Second)
	}
}

func (mc *Application) startCheckerStatusWatcher() {
	for {
		mc.adjustCheckerStatus(mc.CanStartChecker())

		needToStart := mc.getNeedToStartChecker()

		if !needToStart && mc.client.GetNumRunningCheckers() == 0 {
			mc.config.WorkerConfigs.Checker.Status = models.WorkerStopped
			break
		}
		if needToStart && mc.client.GetNumRunningCheckers() != 0 {
			mc.config.WorkerConfigs.Checker.Status = models.WorkerRunning
			break
		}
		time.Sleep(5 * time.Second)
	}
}

func (mc *Application) adjustCrawlerStatus(needToStart bool) {
	mc.setNeedToStartCrawler(needToStart)
	if (needToStart &&
		(mc.config.WorkerConfigs.Crawler.Status != models.WorkerStarting &&
			mc.config.WorkerConfigs.Crawler.Status != models.WorkerRunning)) ||

		(!needToStart &&
			(mc.config.WorkerConfigs.Crawler.Status != models.WorkerStopping &&
				mc.config.WorkerConfigs.Crawler.Status != models.WorkerStopped)) {
		if needToStart {
			mc.config.WorkerConfigs.Crawler.Status = models.WorkerStarting
		} else {
			mc.config.WorkerConfigs.Crawler.Status = models.WorkerStopping
		}
	}
}

func (mc *Application) adjustCheckerStatus(needToStart bool) {
	mc.setNeedToStartChecker(needToStart)
	if (needToStart &&
		(mc.config.WorkerConfigs.Checker.Status != models.WorkerStarting &&
			mc.config.WorkerConfigs.Checker.Status != models.WorkerRunning)) ||

		(!needToStart &&
			(mc.config.WorkerConfigs.Checker.Status != models.WorkerStopping &&
				mc.config.WorkerConfigs.Checker.Status != models.WorkerStopped)) {
		if needToStart {
			mc.config.WorkerConfigs.Checker.Status = models.WorkerStarting
		} else {
			mc.config.WorkerConfigs.Checker.Status = models.WorkerStopping
		}
	}
}

func (mc *Application) startExtractingData() error {
	if mc.crawledIDNos == nil {
		mc.crawledIDNos = make(map[string]bool)
	}
	_, err := mc.client.Crawl(mc.config.WorkerConfigs.Crawler)

	if err != nil {
		if err == ErrNoUsableAccount {
			logger.Root.Infof("There is no usable accounts. Need to fetch accounts again\n")
			mc.NetworkManager.FetchAccounts()
		}

		return err
	}
	mc.WriteCrawledIDNos()

	return nil
}

// CheckIfExtractedBefore returns if an appID is extracted before
func (mc *Application) CheckIfExtractedBefore(appNo string) bool {
	_, ok := mc.crawledIDNos[appNo]
	return ok
}

// MarkAsExtracted marks an application as extracted
func (mc *Application) MarkAsExtracted(appNos []string) {
	mc.Lock()
	for _, v := range appNos {
		mc.crawledIDNos[v] = true
	}
	mc.Unlock()
}

// WriteCrawledIDNos write the id of extracted application to disk
func (mc *Application) WriteCrawledIDNos() error {
	ids := []string{}
	for k := range mc.crawledIDNos {
		ids = append(ids, k)
	}
	logger.Root.Infof("Write the crawled ids no with %d entries\n", len(ids))

	file, err := os.OpenFile(mc.cachedIdsFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		logger.Root.Errorf("Cannot create file to store existing ids\n")
		return err
	}

	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range ids {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}

// readCrawledIdNos reads the id of extracted application from the disk
func (mc *Application) readCrawledIdNos() error {
	if !utils.FileExists(mc.cachedIdsFile) {
		return nil
	}

	file, err := os.Open(mc.cachedIdsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	mc.crawledIDNos = make(map[string]bool)
	for scanner.Scan() {
		txt := strings.Trim(scanner.Text(), " ")
		// id, err := strconv.Atoi(txt)
		// if err == nil {
		mc.crawledIDNos[txt] = true
		// }
	}

	return scanner.Err()

}

func makeRequest(url string, method string, payload interface{}) (string, error) {
	b, _ := json.Marshal(payload)
	var header map[string]string = nil
	if method == http.MethodPost {
		header = map[string]string{
			"accept":       "*/*",
			"Content-Type": "application/json;charset=utf-8",
		}
	}
	logger.Root.Infof(" Send http request to %s\n", url)
	return utils.RealHTTPService{}.SendRequest(url, nil, header, method, bytes.NewBuffer(b), nil)
}

// SendAddingApplicationsReq sends the application to server to add or edit
func (mc *Application) SendAddingApplicationsReq(apps []customer.Application, retries int, callback func()) {
	if mc.config.NsqConfig.SystemNsqID != "" && mc.config.NsqConfig.MyNsqID != "" {
		cm := commandmanager.CreateMessageCommand(nil, apps,
			commands.TypeRequest, commands.SubTypeAddCustomerApplications,
			mc.config.NsqConfig.MyNsqID, mc.config.NsqConfig.SystemNsqID, mc.config.NsqConfig.MainChannel)

		go func() {
			logger.Root.Debugf("before Sending message")
			err, res := mc.NetworkManager.SendMessage(cm, 60)
			logger.Root.Debugf("after Sending message")
			if err != nil {
				logger.Root.Infof("Sending adding application error: %s retries=%d\n", err, retries)
				mc.handleTheError(err)
				if retries < 3 {
					logger.Root.Infof("Wait 5 seconds before another try")
					time.Sleep(5 * time.Second)
					mc.SendAddingApplicationsReq(apps, retries+1, callback)
				}
			} else {
				logger.Root.Infof("Sending adding application result: %v\n", res)
				if res.Success && callback != nil {
					callback()
				}
			}
		}()

	} else {
		logger.Root.Errorf(" Invalid configuration. hostNsqId:%s systemNsqId:%s", mc.config.NsqConfig.MyNsqID, mc.config.NsqConfig.SystemNsqID)
	}
}

// var resultMap map[commands.CommandID]chan string

// SendResponseCommand sends a response command after receiving a request
func (mc *Application) SendResponseCommand(commandID commands.CommandID, subtype commands.CommandSubType, body string, channel string) {
	cm := commandmanager.CreateMessageCommand(&commandID, body, commands.TypeResponse, subtype, mc.config.NsqConfig.MyNsqID, mc.config.NsqConfig.SystemNsqID, channel)
	logger.Root.Infof("Sending respose message id %s in Channel %s from %s to %s", commandID, channel, mc.config.NsqConfig.MyNsqID, mc.config.NsqConfig.SystemNsqID)
	go func() {
		err, _ := mc.NetworkManager.SendMessage(cm, 0)
		if err != nil {
			logger.Root.Infof("could not send the response message because %v", err)
		}
	}()
}

//func (mc *Application) hasNullSocket() bool {
//	mc.Lock()
//	defer mc.Unlock()
//	return mc.conn == nil
//}

//func (mc *Application) setSocket(conn *websocket.Conn) {
//	mc.Lock()
//	mc.conn = conn
//	mc.Unlock()
//}

// SendMessage to our server
//func (mc *Application) SendMessage(message nsq_models.Message, onOK func(models.HTTPResult), onError func(error), timeout int) {
//
//	if timeout == 0 {
//		timeout = 120
//	}
//
//	// mc.Lock()
//	// defer mc.Unlock()
//	defer func() {
//		if r := recover(); r != nil {
//			fmt.Printf("Recovering from panic in SendMessage error is: %v \n", r)
//		}
//	}()
//	if mc.hasNullSocket() {
//		logger.Root.Warning("The socket is not initialized yet\n")
//		return
//	}
//
//	mc.send <- PretendMsg{Msg: message, OnOK: onOK, OnError: onError}
//
//	if message.Command.Type != commands.TypeRequest {
//		logger.Root.Infof("Sent command id:%s...", message.Command.ID)
//		return
//	}
//
//	mc.Lock()
//	if mc.resultMap == nil {
//		mc.resultMap = make(map[commands.CommandID]chan string)
//	}
//	// waiting for the response
//	mc.resultMap[message.Command.ID] = make(chan string)
//	mc.Unlock()
//	logger.Root.Infof("Sent! Wait for the response for the command id:%s...", message.Command.ID)
//
//	go func() {
//		select {
//		case data, ok := <-mc.resultMap[message.Command.ID]:
//			if !ok {
//				logger.Root.Errorf("The result Channel has been broken")
//				break
//			}
//			// logger.Root.Infof(" Got data: %s\n", data)
//			resp := models.HTTPResult{}
//			err := json.Unmarshal([]byte(data), &resp)
//			if err != nil {
//				logger.Root.Errorf("Error when sending message: %s\n", err)
//				onError(err)
//			} else {
//				logger.Root.Infof("Sending message OK. Get the response")
//				onOK(resp)
//			}
//
//			break
//		case <-mc.signalInterruption:
//			break
//		case <-time.After(time.Duration(timeout) * time.Second):
//			logger.Root.Errorf("Timeout when sending message %#v\n", message)
//			break
//		}
//		mc.Lock()
//		delete(mc.resultMap, message.Command.ID)
//		mc.Unlock()
//	}()
//}

// RemoveOldIds removes old application ID. We only keep the last MaxNumOldAppIDItems items
func (mc *Application) RemoveOldIds() {
	var idArrays []string

	mc.Lock()
	defer mc.Unlock()
	for k := range mc.crawledIDNos {
		idArrays = append(idArrays, k)
	}

	if len(idArrays) > MaxNumOldAppIDItems {
		i := 0
		for i < len(idArrays)-MaxNumOldAppIDItems {
			delete(mc.crawledIDNos, idArrays[i])
			i++
		}
		logger.Root.Infof("Removed %d old id elements", len(idArrays)-MaxNumOldAppIDItems)
	}
	idArrays = nil

	// idArrays = idArrays[len(idArrays)-MAX_NUMBER_ITEMS:]
}

func (mc *Application) UpdateAccountMessage(loginMessage string, username string, isActive bool, needToTerminate bool) {
	logger.Root.Infof("(Updating) account `%s` with message `%s` isActive:%v needToTerminate:%v\n", username, loginMessage, isActive, needToTerminate)

	param := models.FeedbackParam{
		Message:         loginMessage,
		Username:        username,
		NeedToTerminate: needToTerminate,
		IsActive:        isActive,
		ClientID:        mc.config.NsqConfig.MyNsqID,
	}
	if needToTerminate {
		param.Message += "(Stopped our application)"
	}

	cm := commandmanager.CreateMessageCommand(nil, param, commands.TypeRequest, commands.SubTypeUpdateRemoteAccount, mc.config.NsqConfig.MyNsqID, mc.config.NsqConfig.SystemNsqID, mc.config.NsqConfig.MainChannel)

	go func() {

		err, res := mc.NetworkManager.SendMessage(cm, 30)
		if err != nil {
			logger.Root.Infof("Sending updating account message error: %s\n", err)
			mc.handleTheError(err)
		} else {
			logger.Root.Infof("Sending updating account message: %v\n", res)

		}
	}()

	if param.NeedToTerminate {
		time.Sleep(3 * time.Second)
		logger.Root.Infof("Sleep 3 seconds before terminating...\n")
		mc.client.Close()
		if mc.onClosing != nil {
			mc.onClosing()
		}
	}
}

func (mc *Application) SetNeedToStopGettingData(val bool) {
	mc.Lock()
	defer mc.Unlock()
	mc.needToExit = val
}

// Dispose prepares the application before closing it
func (mc *Application) Dispose() {
	logger.Root.Infof("Set need to close=true from the Dispose")
	mc.SetNeedToStopGettingData(true)
	mc.Lock()
	mc.config.WorkerConfigs.Crawler.RequestedStatus = models.WorkerStopped
	mc.config.WorkerConfigs.Checker.RequestedStatus = models.WorkerStopped
	mc.Unlock()
	if mc.NetworkManager.isTunnelStarted() {
		mc.sendPingMessage()
		mc.client.Close()
		if mc.stopTunnelFunc != nil {
			mc.stopTunnelFunc()
		}
		if mc.onClosing != nil {
			mc.onClosing()
		}
		mc.signalInterruption <- true
	} else {
		if mc.client != nil {
			mc.client.Close()
		}
	}
}
