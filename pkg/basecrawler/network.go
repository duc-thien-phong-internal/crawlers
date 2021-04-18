package basecrawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/duc-thien-phong/techsharedservices/commands"
	"github.com/duc-thien-phong/techsharedservices/constantcodes/httpcodes"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
	nsq_models "github.com/duc-thien-phong/techsharedservices/nsq/models"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"github.com/ductrung-nguyen/simplesshtun"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type PretendMsg struct {
	Msg     nsq_models.Message
	OnError func(error)
	OnOK    func(result models.HTTPResult)
}

type EventValue interface{}
type EventHandler func(EventValue)
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateDisconnecting
	StateConnecting
	StateConnected
)

const (
	EventFetchingAccountDone = "fetchingAccountDone"
	EventNetworkError        = "networkError"
	EventOnReceivingMessage  = "receivingMessage"
)

func (s ConnectionState) String() string {
	switch s {
	case StateConnected:
		return "Connected"
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateDisconnecting:
		return "Disconnecting"
	}
	return "Unknown"
}

// NetworkManager handles everything about network
// such as connection, tunnel, vpn...
type NetworkManager struct {
	sync.Mutex    // protect every attributes below
	eventHandlers map[string][]chan EventValue
	// A function that will be use to check if a connection is still available or not
	CheckConnectionFunc func() ConnectionState
	needToOpenTunnel    bool
	stateTunnel         ConnectionState
	OpenTunnelFunc      func() error
	CloseTunnelFunc     func() error
	// the connection to our server
	socketConn *websocket.Conn
	// a "queue" of messages that need to be sent
	needToSend chan PretendMsg
	// the map that contains the result of a command request
	resultMap map[commands.CommandID]chan string
	// the Channel that helps us to stop the connection on demand
	signalInterruption chan bool
	//ourServerConfig             OurServer
	//NsqConfig                   *NsqConfig
	stateConnectionToOurServer  ConnectionState
	fetchConfigFromServer       bool
	fetchedAccountsFromServer   bool
	cookieJarOurServer          *cookiejar.Jar
	AuthorizationToken          string
	numErrorWhenCommunicating   int
	workerSoftwareSource        customer.SoftwareSource
	lastTimeGetConfigFromServer time.Time
	LastTimeRefreshToken        time.Time
	application                 *Application

	sleepingTime SleepingTime
}

type SleepingTime struct {
	sleepTime int
	direction int
}

func CreateSleepingTime() SleepingTime {
	return SleepingTime{
		sleepTime: 0,
		direction: 1,
	}
}

func (s *SleepingTime) Update() {
	s.sleepTime += s.direction * 5
	if s.sleepTime >= 10*60 || s.sleepTime <= 0 {
		s.direction *= -1
		if s.sleepTime < 0 {
			s.sleepTime = 1
		}
	}
}

func CreateNetworkManager(
	a *Application,

	checkConnFunc func() ConnectionState,
	openTunnelFunc func() error,
	closeFunnelFunc func() error,
	workerSoftwareSource customer.SoftwareSource,
) *NetworkManager {
	n := NetworkManager{
		application:          a,
		CheckConnectionFunc:  checkConnFunc,
		OpenTunnelFunc:       openTunnelFunc,
		CloseTunnelFunc:      closeFunnelFunc,
		workerSoftwareSource: workerSoftwareSource,
		sleepingTime:         CreateSleepingTime(),
	}
	return &n
}

//func (n *NetworkManager) setOurServerConfig(c OurServer) {
//	n.ourServerConfig = c
//}

func (n *NetworkManager) AddEventHandler(eventName string, ch chan EventValue) {
	// if the event listener information hasn't been initialized yet ever
	// initialize it first
	if n.eventHandlers == nil {
		n.eventHandlers = make(map[string][]chan EventValue)
	}

	// if the list of listeners for the given event hasn't been initialized yet
	// create a list for that event, and vice versa
	if _, ok := n.eventHandlers[eventName]; !ok {
		n.eventHandlers[eventName] = []chan EventValue{ch}
	} else {
		n.eventHandlers[eventName] = append(n.eventHandlers[eventName], ch)
	}
}

// RemoveEventHandler removes a listener from an event queue
func (n *NetworkManager) RemoveEventHandler(eventName string, ch chan EventValue) {
	if _, ok := n.eventHandlers[eventName]; ok {
		for i := range n.eventHandlers[eventName] {
			if n.eventHandlers[eventName][i] == ch {
				n.eventHandlers[eventName] = append(n.eventHandlers[eventName][:i], n.eventHandlers[eventName][i+1:]...)
				break
			}
		}
	}
}

func (n *NetworkManager) AddNetworkErrorHandler(ch chan EventValue) {
	n.AddEventHandler(EventNetworkError, ch)
}

func (n *NetworkManager) RemoveNetworkErrorHandler(ch chan EventValue) {
	n.RemoveEventHandler(EventNetworkError, ch)
}

func (n *NetworkManager) RegisterEventHandler(eventName string, handler EventHandler) (unregisterEventFunc func()) {
	c := make(chan EventValue)
	cancelChan := make(chan interface{})
	n.AddEventHandler(eventName, c)
	go func() {
	forLoop:
		for {
			select {
			case v := <-c:
				handler(v)

			case <-cancelChan:
				break forLoop
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	return func() {
		cancelChan <- struct{}{}
	}
}

// EmitEvent raises an event, and invokes the handlers
func (n *NetworkManager) EmitEvent(eventName string, response EventValue) {
	if _, ok := n.eventHandlers[eventName]; ok {
		for _, handler := range n.eventHandlers[eventName] {
			go func(handler chan EventValue) {
				handler <- response
			}(handler)
		}
	}
}

func (n *NetworkManager) SetNeedToOpenTunnel(value bool) {
	n.needToOpenTunnel = value
}

func (n *NetworkManager) onConnectionToOurServerChanged(v EventValue) {
	if v != nil {
		result := v.(ConnectionState)
		if result == StateConnected {
			logger.Root.Infof("Connected to our server")
			//if !n.fetchConfigFromServer {
			n.fetchConfigurationFromServer(true)
			//}

		} else if result == StateDisconnected {
			logger.Root.Infof("Disconnected from our server")
		}
	}
}

func (n *NetworkManager) onTunnelStateChange(v EventValue) {
	if v != nil {
		result := v.(ConnectionState)
		if result == StateConnected {
			logger.Root.Infof("Connected to our tunnel")
			if n.AuthorizationToken == "" && n.LoginToOurServer() {
				n.FetchAccounts()
			} else {
				if n.RefreshToken(false) != nil {
					n.sleepingTime.Update()
					logger.Root.Infof(" Sleeping %d seconds before trying to connect to server again...", n.sleepingTime.sleepTime)
					time.Sleep(time.Duration(n.sleepingTime.sleepTime) * time.Second)
					return
				}
			}

			// register to our server
			n.openSocketConnection()
		} else if result == StateDisconnected {
			logger.Root.Infof("Disconnected from our tunnel")
			if n.socketConn != nil {
				n.socketConn.Close()
			}
			n.setStateConnectionToServer(StateDisconnected)

		}
	}
}

func (n *NetworkManager) openSocketConnection() {

	logger.Root.Infof(" Registering myself to server %s", fmt.Sprintf(BaseServerURL, n.application.config.OServer.Host, n.application.config.OServer.Port))

	u := url.URL{
		Scheme: "ws",
		Host:   fmt.Sprintf(BaseServerURL, n.application.config.OServer.Host, n.application.config.OServer.Port),
		Path:   fmt.Sprintf(ChannelSocketPath, n.workerSoftwareSource),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var err error
	n.socketConn, _, err = websocket.DefaultDialer.DialContext(ctx, u.String(), http.Header{"Authorization": []string{n.AuthorizationToken}})
	if err != nil {
		logger.Root.Errorf(" Error when connect to socket: %s\n", err)
		n.setStateConnectionToServer(StateDisconnected)

		logger.Root.Infof(" Sleeping %d seconds before trying to connect to server again...", n.sleepingTime.sleepTime)
		time.Sleep(time.Duration(n.sleepingTime.sleepTime) * time.Second)
		n.sleepingTime.Update()
		if n.socketConn != nil {
			n.socketConn.Close()
		}
		return
	}

	regResult := registerResult{}
	err = n.socketConn.ReadJSON(&regResult)
	if err != nil {
		logger.Root.Errorf(" Error when reading json from server's message: %s\n", err)
		n.setStateConnectionToServer(StateDisconnected)

		logger.Root.Infof(" Sleeping %d seconds before trying to connect to server socket again...", n.sleepingTime.sleepTime)
		time.Sleep(time.Duration(n.sleepingTime.sleepTime) * time.Second)
		if n.socketConn != nil {
			n.socketConn.Close()

		}
		return
	} else {
		n.sleepingTime.sleepTime = 0
	}

	n.application.config.NsqConfig.MyNsqID = regResult.ToUser
	n.application.config.NsqConfig.SystemNsqID = regResult.FromUser
	n.application.config.NsqConfig.MainChannel = regResult.Channel

	n.setStateConnectionToServer(StateConnected)

	logger.Root.With("myID", n.application.config.NsqConfig.MyNsqID).
		With("systemID", n.application.config.NsqConfig.SystemNsqID).
		With("channel", n.application.config.NsqConfig.MainChannel).
		Infof("Registered to our server!")

	logger.Root.Infof("Waiting for receiving command from server....\n")
	go n.waitForCommandFromOurServer()
}

func (n *NetworkManager) ConnectToTunnel() {
	app := n.application
	if n.application.config.Tunnel.Host != "" {
		// doneTunnel := make(chan error)
		onOK := func() {
			n.setTunnelState(StateConnected)
			logger.Root.Info("Tunnel started  OK .....")
		}
		onError := func() {
			n.setTunnelState(StateDisconnected)
		}
		sshTun := simplesshtun.New(app.config.Tunnel.LocalPort, app.config.Tunnel.Host, app.config.Tunnel.RemotePort)
		sshTun.SetUser(app.config.Tunnel.Username)
		if app.config.Tunnel.PrivateKey != "" {
			sshTun.SetKeyFile(app.config.Tunnel.PrivateKey)
		} else {
			sshTun.SetPassword(app.config.Tunnel.Password)
		}
		defer sshTun.Stop()

		app.stopTunnelFunc = func() {
			logger.Root.Infof("Force stop the tunnel (maybe to restart it)")
			// mc.setTunnelState(false)
			sshTun.Stop()
		}

		go func() {
			for {
				if app.config.Tunnel.Host != "" && n.isTunnelStarted() && !app.canStartTunnel() {
					logger.Root.Infof("Stop the tunnel\n")
					sshTun.Stop()

					n.setTunnelState(StateDisconnected)
				}
				time.Sleep(5 * time.Second)
			}
		}()

		for {
			// mc.loadConfigFromDisk()
			logger.Root.Infof("Can start tunnel: %v Tunnel is started:%v\n", app.canStartTunnel(), app.NetworkManager.isTunnelStarted())
			if app.canStartTunnel() && !n.isTunnelStarted() {
				logger.Root.Info("Starting the tunnel....")
				// We want to connect to port 8080 on our machine to access port 80 on my.super.host.com

				// We enable debug messages to see what happens
				sshTun.SetDebug(true)
				sshTun.SetPort(app.config.Tunnel.SSHPort)

				// We set a callback to know when the tunnel is ready
				sshTun.SetConnState(func(tun *simplesshtun.SSHTun, state simplesshtun.ConnState) {
					switch state {
					case simplesshtun.StateStarting:
						log.Printf("Tunnel is Starting")
					case simplesshtun.StateStarted:
						log.Printf("Tunnel is Started")
						onOK()
					case simplesshtun.StateStopped:
						log.Printf("Tunnel is Stopped")
						onError()
					}
				})

				if err := sshTun.Start(); err != nil {
					log.Printf("SSH tunnel stopped: %s", err.Error())
				}
			}

			n.setTunnelState(StateDisconnected)
			sshTun.Stop()

			logger.Root.Info(" Sleeping 3 seconds before next try to restart tunnel...")
			time.Sleep(3 * time.Second)
		}
		return
	} else {
		logger.Root.Infof(" NO NEED THE TUNNEL\n")
		n.setTunnelState(StateConnected)
	}

}

func (n *NetworkManager) Run() {
	_ = n.RegisterEventHandler(EventConnectionToOurServerChanged, n.onConnectionToOurServerChanged)
	_ = n.RegisterEventHandler(EventTunnelStateChanged, n.onTunnelStateChange)

	if n.CheckConnectionFunc == nil {
		n.CheckConnectionFunc = func() ConnectionState {
			_, err := http.Get("https://google.com")
			if err != nil {
				logger.Root.Errorf("Could not connect to google %v", err)
				return StateDisconnected
			}
			return StateConnected
		}
	}
	go n.ConnectToTunnel()
	go n.writePump()

	// run a watcher to check the connection state
	for {
		currentState := n.CheckConnectionFunc()
		if currentState != n.stateConnectionToOurServer && currentState == StateDisconnected {
			n.EmitEvent(EventConnectionToOurServerChanged, currentState)
		}
		time.Sleep(10 * time.Second)
	}
}

func (n *NetworkManager) setStateConnectionToServer(state ConnectionState) {
	n.Lock()
	logger.Root.Infof("Connected to server(Disconnected/Disconnecting/Connecting/Connected): %s", state)
	n.stateConnectionToOurServer = state
	n.EmitEvent(EventConnectionToOurServerChanged, state)
	n.Unlock()
}

func (n *NetworkManager) isTunnelStarted() bool {
	return n.stateTunnel == StateConnected
}

func (n *NetworkManager) setTunnelState(v ConnectionState) {
	n.Lock()
	defer n.Unlock()
	n.stateTunnel = v
	n.EmitEvent(EventTunnelStateChanged, v)
	logger.Root.Infof("Set tunnelStarted to %v", v)
}

//func (n *NetworkManager) ConnectToOurServer() {
//	n.setStateConnectionToServer(StateDisconnected)
//
//	i := 0
//
//	networkErrorChan := make(chan EventValue)
//	n.AddNetworkErrorHandler(networkErrorChan)
//	for {
//
//		if n.isTunnelStarted() {
//			if n.RefreshToken(false) != nil {
//				n.sleepingTime.Update()
//				logger.Root.Infof(" Sleeping %d seconds before trying to connect to server again...", n.sleepingTime.sleepTime)
//				time.Sleep(time.Duration(n.sleepingTime.sleepTime) * time.Second)
//				continue
//			}
//			logger.Root.Infof(" Registering myself to server %s", fmt.Sprintf(BaseServerURL, n.application.config.OServer.Host, n.application.config.OServer.Port))
//
//			u := url.URL{
//				Scheme: "ws",
//				Host:   fmt.Sprintf(BaseServerURL, n.application.config.OServer.Host, n.application.config.OServer.Port),
//				Path:   fmt.Sprintf(ChannelSocketPath, n.workerSoftwareSource),
//			}
//			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
//			defer cancel()
//
//			var err error
//			n.socketConn, _, err = websocket.DefaultDialer.DialContext(ctx, u.String(), http.Header{"Authorization": []string{n.AuthorizationToken}})
//
//			if err != nil {
//				logger.Root.Errorf(" Error when connect to socket: %s\n", err)
//				if n.isTunnelStarted() {
//					n.LoginToOurServer()
//					n.FetchAccounts()
//				}
//
//				n.setStateConnectionToServer(StateConnected)
//
//				logger.Root.Infof(" Sleeping %d seconds before trying to connect to server again...", n.sleepingTime.sleepTime)
//				time.Sleep(time.Duration(n.sleepingTime.sleepTime) * time.Second)
//
//				i++
//				// mc.conn = nil
//				n.sleepingTime.Update()
//
//				if n.socketConn != nil {
//					n.socketConn.Close()
//				}
//				continue
//			}
//
//			//n.socketConn.SetReadDeadline(time.Time{})
//
//			regResult := registerResult{}
//			err = n.socketConn.ReadJSON(&regResult)
//			if err != nil {
//				logger.Root.Errorf(" Error when reading json from server's message: %s\n", err)
//				n.setStateConnectionToServer(StateDisconnected)
//				i++
//
//				logger.Root.Infof(" Sleeping %d seconds before trying to connect to server socket again...", n.sleepingTime.sleepTime)
//				time.Sleep(time.Duration(n.sleepingTime.sleepTime) * time.Second)
//				if n.socketConn != nil {
//					n.socketConn.Close()
//				}
//				continue
//			} else {
//				n.sleepingTime.sleepTime = 0
//			}
//
//			logger.Root.Infof("Register result: %#v", regResult)
//
//			n.application.config.NsqConfig.MyNsqID = regResult.ToUser
//			n.application.config.NsqConfig.SystemNsqID = regResult.FromUser
//			n.application.config.NsqConfig.MainChannel = regResult.Channel
//
//			n.setStateConnectionToServer(StateConnected)
//
//			logger.Root.Infof(" myid: %s sysId:%s Channel: %s\n", n.application.config.NsqConfig.MyNsqID,
//				n.application.config.NsqConfig.SystemNsqID, n.application.config.NsqConfig.MainChannel)
//
//			logger.Root.Infof("Waiting for receiving command from server....\n")
//			go n.waitForCommandFromOurServer()
//
//			<-networkErrorChan
//		forLoop:
//			for {
//				select {
//				case <-networkErrorChan:
//				default:
//					// no more error in the Channel
//					break forLoop
//				}
//			}
//
//			i++
//		} else {
//			logger.Root.Infof("No need to re-register. isTunnelStarted: %v socket is nil:%v", n.isTunnelStarted(), n.socketConn == nil)
//		}
//
//		if n.socketConn != nil {
//			n.socketConn.Close()
//		}
//
//		logger.Root.Infof(" Sleeping %d seconds before trying to connect to server again...", n.sleepingTime.sleepTime)
//		time.Sleep(time.Duration(n.sleepingTime.sleepTime) * time.Second)
//		n.sleepingTime.sleepTime += 2
//
//	}
//}

func (n *NetworkManager) CheckConnectionToOurServer() bool {
	return n.RefreshToken(true) == nil
}

func (n *NetworkManager) hasNullSocket() bool {
	n.Lock()
	defer n.Unlock()
	return n.socketConn == nil
}

func (n *NetworkManager) setSocket(conn *websocket.Conn) {
	n.Lock()
	n.socketConn = conn
	n.Unlock()
}

// SendMessage to our server
func (n *NetworkManager) SendMessage(message nsq_models.Message, timeout int) (err error, response models.HTTPResult) {

	if timeout == 0 {
		timeout = 120
	}

	// n.Lock()
	// defer n.Unlock()
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovering from panic in SendMessage error is: %v \n", r)
		}
	}()
	if n.hasNullSocket() {
		logger.Root.Warn("The socket is not initialized yet\n")
		return
	}

	n.needToSend <- PretendMsg{Msg: message}

	if message.Command.Type != commands.TypeRequest {
		logger.Root.Infof("Sent command id:%s...", message.Command.ID)
		return
	}

	n.Lock()
	if n.resultMap == nil {
		n.resultMap = make(map[commands.CommandID]chan string)
	}
	// waiting for the response
	n.resultMap[message.Command.ID] = make(chan string)
	n.Unlock()
	logger.Root.Infof("Sent! Wait for the response for the command id:%s...", message.Command.ID)

	select {
	case data, ok := <-n.resultMap[message.Command.ID]:
		if !ok {
			logger.Root.Errorf("The result Channel has been broken")
			break
		}
		// logger.Root.Infof(" Got data: %s\n", data)
		resp := models.HTTPResult{}
		err := json.Unmarshal([]byte(data), &resp)
		if err != nil {
			logger.Root.Errorf("Error when sending message: %s\n", err)
			return err, models.HTTPResult{}
		} else {
			logger.Root.Infof("Sending message OK. Get the response")
			return nil, resp
		}

		break
	case <-n.signalInterruption:
		break
	case <-time.After(time.Duration(timeout) * time.Second):
		logger.Root.Errorf("Timeout when sending message %#v\n", message)
		break
	}
	n.Lock()
	delete(n.resultMap, message.Command.ID)
	n.Unlock()
	return errors.New("force to exit (timout/on-demand)"), models.HTTPResult{}
}

func (n *NetworkManager) LoginToOurServer() bool {

	config := n.application.config.OServer
	// mc.loadConfigFromDisk()
	loginUrl := fmt.Sprintf(ServerLoginURL, config.Scheme, config.Host, config.Port)
	username := config.Username
	password := config.Password
	if os.Getenv("S_USERNAME") != "" {
		username = os.Getenv("S_USERNAME")
	}
	if os.Getenv("S_PASSWORD") != "" {
		password = os.Getenv("S_PASSWORD")
	}
	b, _ := json.Marshal(map[string]interface{}{
		"username": username,
		"password": password,
	})
	// logger.Root.Infoln(os.Getenv("S_USERNAME"), os.Getenv("S_PASSWORD"))

	if n.cookieJarOurServer == nil {
		n.cookieJarOurServer, _ = cookiejar.New(nil)
	}
	resultStr, err := utils.SendRequest(loginUrl, n.cookieJarOurServer, map[string]string{"Content-Type": "application/json"}, "POST", bytes.NewBuffer(b), nil)

	var result models.ServerLoginResult
	json.Unmarshal([]byte(resultStr), &result)

	if err != nil || !result.Ok {
		logger.Root.Errorf(" Error when login. Result: %v Error %v\n", result, err)
		time.Sleep(5 * time.Second)
		return false
	}
	logger.Root.Infof(" Login OK!\n")

	n.AuthorizationToken = "Bearer " + result.Data
	return true
}

// RefreshToken logins to our server
func (n *NetworkManager) RefreshToken(force bool) error {
	if n.LastTimeRefreshToken.Add(time.Duration(5)*time.Minute).Unix() > time.Now().Unix() && !force {
		return nil
	}
	logger.Root.Infof(" Refreshing token of our server:")

	// mc.loadConfigFromDisk()
	url := fmt.Sprintf(ServerRefreshURL, n.application.config.OServer.Scheme, n.application.config.OServer.Host, n.application.config.OServer.Port)

	// logger.Root.Infof("refresh token with the current token%s\n", mc.AuthorizationToken)

	resultStr, err := utils.SendRequest(url, n.cookieJarOurServer, map[string]string{"Authorization": n.AuthorizationToken}, "GET", nil, nil)
	if err != nil {
		logger.Root.Errorf(" Error when refresh token. Error %s\n", err)
		return err
	}

	var result models.ServerRefreshTokenResult
	json.Unmarshal([]byte(resultStr), &result)

	if err != nil {
		logger.Root.Errorf(" Error when refresh token. Error %s\n", err)
		return err
	}
	logger.Root.Info("OK!")
	n.LastTimeRefreshToken = time.Now()
	n.AuthorizationToken = "Bearer " + result.Token
	return nil
}

// fetchConfigurationFromServer tries to get the configuration from server
// if we recently got it, no need to fetch again
// if force = yes, fetch it anyway
func (n *NetworkManager) fetchConfigurationFromServer(force bool) error {
	if n.lastTimeGetConfigFromServer.Add(time.Duration(5)*time.Minute).Unix() > time.Now().Unix() && !force {
		logger.Root.Infof(" no need to fetch configuration from our server. Last time got config: %s", n.lastTimeGetConfigFromServer)
		return nil
	}

	// mc.loadConfigFromDisk()
	url := fmt.Sprintf(ServerFetchConfigURL, n.application.config.OServer.Scheme, n.application.config.OServer.Host, n.application.config.OServer.Port, n.workerSoftwareSource)
	logger.Root.Infof(" fetching configuration from our server: %s", url)

	// logger.Root.Infof("refresh token with the current token%s\n", mc.AuthorizationToken)

	resultStr, err := utils.SendRequest(url, n.cookieJarOurServer, map[string]string{"Authorization": n.AuthorizationToken}, "GET", nil, nil)
	if err != nil {
		logger.Root.Errorf(" Error when fetching configuration. Error %s\n", err)
		return err
	}

	type customHTTPResult struct {
		Error   string                  `json:"error"`
		Success bool                    `json:"success"`
		Data    models.DataClientConfig `json:"data,omitempty"`
		Code    httpcodes.CodeType      `json:"code"`
		Message string                  `json:"message"`
	}
	var result customHTTPResult
	json.Unmarshal([]byte(resultStr), &result)
	//logger.Root.Debugf("Result of getting config %v", result)

	if !result.Success {
		logger.Root.Errorf(" Error when fetching configuration. Error %+v\n", result)
		return err
	}
	n.application.config.WorkerConfigs = result.Data
	logger.Root.With("configuration", n.application.config.WorkerConfigs).Info(" fetch configuration from server OK!")
	n.lastTimeGetConfigFromServer = time.Now()
	n.fetchConfigFromServer = true
	return nil
}

// FetchAccounts returns account to login remote server
func (n *NetworkManager) FetchAccounts() {
	config := n.application.config.OServer

	url := fmt.Sprintf(ServerFetchAccountURL, config.Scheme, config.Host, config.Port, n.workerSoftwareSource)

	// logger.Root.Infoln(mc.AuthorizationToken)
	headers := map[string]string{
		"Authorization": n.AuthorizationToken,
	}
	resultStr, err := utils.SendRequest(url, n.cookieJarOurServer, headers, "GET", nil, nil)

	var result models.ServerListRemoteAccountsResult
	json.Unmarshal([]byte(resultStr), &result)

	if err != nil || !result.Ok {
		logger.Root.Errorf(" Error when fetching accounts. Error %s Result: %#v\n", err, result)
		n.handleTheError(err)
	} else {
		logger.Root.With("result", result).Infof(" Fetching accounts OK!\n")
	}

	n.EmitEvent(EventFetchingAccountDone, result.RemoteAccounts)
	//if len(result.RemoteAccounts) > 0 {
	//	logger.Root.Infof("emit event fetching accounts done. Remember to assign accounts")
	//	// numAccs := len(result.RemoteAccounts)
	//	// mc.CurrentAccount = result.RemoteAccounts[rand.Intn(numAccs)]
	//	//if err := n.client.AssignAccounts(result.RemoteAccounts); err != nil {
	//	//	logger.Root.Errorf("Error when assigning accounts: %s\n", err)
	//	//	logger.Root.Infof("Set need to close=true when assigning accounts")
	//	//
	//	//	n.SetNeedToStopGettingData(true)
	//	//}
	//	// callbackWithAccount(result.RemoteAccounts)
	//} else {
	//	n.EmitEvent(EventFetchingAccountDone, result.RemoteAccounts)
	//}
}

func (n *NetworkManager) handleTheError(err error) {
	if err == nil {
		return
	}
	if (strings.Contains(err.Error(), "An established connection was aborted by the software in your host machine") ||
		strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host")) && n.isTunnelStarted() {
		logger.Root.Infof("Connection corrupted")
		n.numErrorWhenCommunicating++

		if n.CloseTunnelFunc != nil && n.numErrorWhenCommunicating >= 3 {
			logger.Root.Infof("The tunnel seems to be broken. Set isTunnelStarted to false in order to restart it")
			if err := n.CloseTunnelFunc(); err != nil {
				logger.Root.Infof("Could not close the tunnel. Error: %v", err)
			}
			n.numErrorWhenCommunicating = 0
		}
		go func() {
			//mc.errorChan <- err
			n.EmitEvent(EventNetworkError, err)
		}()
		logger.Root.Infof("Notified the socket")
	}
}

func (n *NetworkManager) waitForCommandFromOurServer() {
	defer func() {
		logger.Root.Infof("Stop waiting for commands from our server")
		if r := recover(); r != nil {
			logger.Root.Infof("recover from function waitForCommandFromOurServer")
			logger.Root.Infof(string(debug.Stack()))
		}
	}()

	for {
		if n.hasNullSocket() {
			time.Sleep(500 * time.Millisecond)
			logger.Root.Infof("skip because the conn is nil")
			continue
		}

		logger.Root.Infof("wait to read message")
		msg := nsq_models.Message{}
		err := n.socketConn.ReadJSON(&msg)
		// logger.Root.Infof("Get message: %#v\n", msg)

		if err != nil {
			logger.Root.Errorf(" Error when reading from socket: %s\n", err)
			go func() {
				n.EmitEvent(EventNetworkError, err)
				n.EmitEvent(EventTunnelStateChanged, StateDisconnected)
			}()
			break
		}
		// logger.Root.Infof("got Message: %#v\n", msg)

		if !msg.HasEmptyBody() {
			n.EmitEvent(EventOnReceivingMessage, msg)
		} else {
			logger.Root.Infof(" Message with empty body\n")
		}

		// logger.Root.Infof("finsih Message: %#v\n", msg)
	}
}

//func (n *NetworkManager) sendPingMessage() {
//	needToGetAppConfig := false
//	lastTimeUpdate := utils.GetCurrentTime().Unix()
//
//	var currentCrawlerStatus models.WorkerStatusType = models.WorkerStopped
//	var currentCheckerStatus models.WorkerStatusType = models.WorkerStopped
//
//	if n.lastTimeGetConfigFromServer.Unix()+1*60*60 < utils.GetCurrentTime().Unix() {
//		needToGetAppConfig = true
//		lastTimeUpdate = n. config.WorkerConfigs.UpdatedAt.Unix()
//	}
//
//	// after fetching the status from server
//	// and that config is still "up to date"
//	if mc.fetchedConfigFromServer && mc.lastTimeGetConfigFromServer.Unix()+1*60*60 >= utils.GetCurrentTime().Unix() {
//		currentCrawlerStatus = mc.config.WorkerConfigs.Crawler.Cu
//		currentCheckerStatus = mc.config.WorkerConfigs.Checker.RequestedStatus
//		lastTimeUpdate = utils.GetCurrentTime().Unix()
//	}
//
//	logger.Root.Infof("Sending ping. Need to get config=%v....\n", needToGetAppConfig)
//	commandData := commands.CommandReqPingPong{
//		Time:                 lastTimeUpdate,
//		CurrentStatusCrawler: currentCrawlerStatus,
//		CurrentStatusChecker: currentCheckerStatus,
//		WorkerSource:         mc.client.GetWorkerSoftwareSource(),
//		GetApplicationConfig: needToGetAppConfig,
//	}
//
//	message := commandmanager.CreateMessageCommand(nil, commandData, commands.TypeRequest, commands.SubTypePingPong, mc.config.NsqConfig.MyNsqID, mc.config.NsqConfig.SystemNsqID, mc.config.NsqConfig.MainChannel)
//
//	// message.ConstructBody(string(appsInByte))
//	mc.SendMessage(message, func(res models.HTTPResult) {
//		if res.Data != nil {
//			dbByte, _ := json.Marshal(res.Data)
//			var pingPongData commands.CommandRespPingPong
//			if err := json.Unmarshal(dbByte, &pingPongData); err == nil && needToGetAppConfig {
//				if pingPongData.ApplicationConfig != nil {
//					logger.Root.Infof("Updating worker configuration....\n")
//					oldConfig := mc.config.WorkerConfigs
//					mc.config.WorkerConfigs = *pingPongData.ApplicationConfig
//					logger.Root.Infof("Last crawling date: %s", mc.config.WorkerConfigs.Crawler.LastCrawlingDate.Format(time.RFC1123))
//					mc.fetchedConfigFromServer = true
//					mc.compareRunningModesAndStartCrawlerAndChecker(oldConfig)
//					mc.lastTimeGetConfigFromServer = utils.GetCurrentTime()
//				}
//			}
//		}
//		logger.Root.Infof("Sending ping message OK\n")
//		//mc.updateLastTimeSendingPing()
//	}, func(err error) {
//		logger.Root.Infof("Sending ping message Fail: %s\n", err)
//	}, 20)
//	mc.updateLastTimeSendingPing()
//}

func (n *NetworkManager) writePump() {
	for {
		select {
		case pretendMsg, ok := <-n.needToSend:
			if !ok {
				break
			}
			for n.hasNullSocket() {
				logger.Root.Infof("Sleep and wait until conn is not null")
				time.Sleep(200 * time.Millisecond)
			}
			if err := n.socketConn.WriteJSON(pretendMsg.Msg); err != nil {
				logger.Root.Errorf("Error when writing message %#v", pretendMsg.Msg)
				if pretendMsg.Msg.Command != nil {
					logger.Root.Errorf("  With command %#v\n err:%s", pretendMsg.Msg.Command, err)
				}
				pretendMsg.OnError(err)
				n.handleTheError(err)
			} else {
				logger.Root.Infof("Sent command id:%s...", pretendMsg.Msg.Command.ID)
			}

		default:
			break
		}
	}
}
