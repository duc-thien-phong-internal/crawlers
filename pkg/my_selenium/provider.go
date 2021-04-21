package my_selenium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/duc-thien-phong-internal/crawlers/pkg/osinfo"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kardianos/osext"
	"github.com/mitchellh/go-homedir"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/firefox"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const SeleniumDockerImage = "selenium/standalone-firefox:3.141.59-20201119"

type ChannelInfo struct {
	Username        string `json:"username"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	ConfirmationURL string `json:"confirmation_url"`
	Password        string `json:"password"`
	ID              int    `json:"id"`
}

type DriverInfo struct {
	URL               string              `json:"session_url"`
	Session           string              `json:"session_id"`
	ChannelInfo       ChannelInfo         `json:"channel_info"`
	Service           *selenium.Service   `json:"-"`
	Driver            *selenium.WebDriver `json:"-"`
	DockerContainerID string              `json:"containerID"`
}

var drivers map[int]DriverInfo

// const (
// 	// These paths will be different on your system.
// 	seleniumPath    = "drivers/selenium-server.jar"
// 	geckoDriverPath = "drivers/geckodriver"
// )

// Close tries to quit the driver and stop the service
func (driver DriverInfo) Close() {

	if driver.Driver != nil {
		logger.Root.Infof("Quit selenium driver\n")
		// if err := (*driver.Driver).Close(); err != nil {
		// 	logger.Root.Errorf("Could not close selenium driver: %v", err)
		// }
		if err := (*driver.Driver).Quit(); err != nil {
			logger.Root.Errorf("Could not quit selenium driver %v", err)
		}
	}
	if driver.Service != nil {
		logger.Root.Infof("Stop selenium services\n")
		if err := driver.Service.Stop(); err != nil {
			logger.Root.Errorf("Could not close selenium service %v", err)
		}
	}
	if driver.DockerContainerID != "" {
		closeDockerContainer(driver.DockerContainerID)
	}
	driver.Driver = nil
	driver.Service = nil
}

func closeDockerContainer(id string) {
	logger.Root.Infof("Closing docker container ID: `%s`", id)
	if cli, err := client.NewEnvClient(); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		if err = cli.ContainerStop(ctx, id, nil); err != nil {
			logger.Root.Infof("Could not stop container id: `%s`. Error: %s\n", id, err)
		}
		if err = cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true}); err != nil {
			logger.Root.Infof("Could not delete container id: `%s`. Error: %s\n", id, err)
		}
	} else {
		logger.Root.Infof("Could not create docker client instance to stop container id `%s`\n", id)
	}
}

func pickUnusedPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

// GetSeleniumService returns a driver instance
func GetSeleniumService(withDocker bool, geckoPath string, args map[string]interface{}) (driver DriverInfo, err error) {

	port, err := pickUnusedPort()
	if err != nil {
		return DriverInfo{}, err
	}

	var service *selenium.Service
	var wd *selenium.WebDriver
	var containerID string
	var containerName string = "selenium-"
	if args != nil {
		if name, ok := args["containerName"]; ok && name != "" {
			containerName += fmt.Sprint(name)
		}
	}
	if containerName == "selenium-" {
		containerName += fmt.Sprint(time.Now().Unix())
	}

	if !withDocker {
		curDir, _ := osext.ExecutableFolder()

		if geckoPath == "" {
			geckoPath = filepath.Join(curDir, "drivers", "geckodriver")
			if osinfo.CurrentOS == "windows" {
				geckoPath += ".exe"
			}
		}

		logger.Root.Infof("Geckopath: '%s'\n", geckoPath)
		if !utils.FileExists(geckoPath) {
			logger.Root.Errorf("The file `%s`  doesn't exist", geckoPath)
			return DriverInfo{}, errors.New("Gecko driver file doesnt exist")
		}

		opts := []selenium.ServiceOption{
			// selenium.StartFrameBuffer(),           // Start an X frame buffer for the browser to run in.
			selenium.GeckoDriver(geckoPath), // Specify the path to GeckoDriver in order to use Firefox.
		}

		logger.Root.Infof("Selenium server: %s port: %d", filepath.Join(curDir, "drivers", "selenium-server.jar"), port)
		// selenium.SetDebug(true)
		seleniumFile := filepath.Join(curDir, "drivers", "selenium-server.jar")
		if !utils.FileExists(seleniumFile) {
			logger.Root.Errorf("The file `%s`  doesn't exist", seleniumFile)
			return DriverInfo{}, errors.New("Selenium Jar file doesnt exist")
		}
		service, err = selenium.NewSeleniumService(seleniumFile, port, opts...)

	} else {

		cli, err := client.NewEnvClient()
		if err != nil {
			return DriverInfo{}, err
		}
		ctx := context.Background()

		imageExist := false
		summary, err := cli.ImageList(ctx, types.ImageListOptions{All: true})
		if err == nil {
			for _, s := range summary {
				// logger.Root.Infof("%#v", s)
				if len(s.RepoTags) > 0 && s.RepoTags[0] == SeleniumDockerImage {
					imageExist = true
				}
			}
		}
		if !imageExist {
			logger.Root.Infof("Need to pull image %s", SeleniumDockerImage)

			if reader, err := cli.ImagePull(ctx, SeleniumDockerImage, types.ImagePullOptions{}); err != nil {
				panic(err)
			} else {
				io.Copy(os.Stdout, reader)
			}
		}
		// io.Copy(os.Stdout, reader)
		config := &container.Config{
			Image: SeleniumDockerImage,
			ExposedPorts: nat.PortSet{
				"4444/tcp": struct{}{},
			},
			// Volumes: map[string]struct{}{
			// 	"/dev/shm": "/dev/shm",
			// },
		}

		hostConfig := &container.HostConfig{
			AutoRemove: true,
			PortBindings: nat.PortMap{
				"4444/tcp": []nat.PortBinding{
					{
						HostIP:   "localhost",
						HostPort: fmt.Sprint(port),
					},
				},
			},
		}

		resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
		if err != nil {
			logger.Root.Errorf("Could not create container: %v", err)
			return DriverInfo{}, nil
		}

		time.Sleep(500 * time.Millisecond)

		containerID = resp.ID
		if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			logger.Root.Errorf("Error! Could not start the container: %s\n", err)
			cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
			return DriverInfo{}, nil
		}
		// wait to start the selenium server in the container
		time.Sleep(20 * time.Second)
	}

	retries := 0
	sessionURL := fmt.Sprintf("http://localhost:%d/wd/hub", port)
	logger.Root.Infof("Session URL:%s\n", sessionURL)
	for retries < 3 {
		wd, err = AttachToSession(sessionURL, args)
		if err == nil && wd != nil {
			logger.Root.Infof("Using this session\n")
			break
		}

		retries++
		logger.Root.Infof("Sleep 5 seconds (retries=%d)\n", retries)
		time.Sleep(5 * time.Second)
	}

	if err != nil {
		logger.Root.Infof("Error when trying to attach to Selenium session %v", err)
		if withDocker && containerID != "" {
			closeDockerContainer(containerID)
		} else {
			if wd != nil {
				logger.Root.Infof("Quit selenium driver\n")
				if err := (*wd).Quit(); err != nil {
					logger.Root.Errorf("Could not quit selenium driver %v", err)
				}
			}
			if service != nil {
				logger.Root.Infof("Stop selenium services\n")
				if err := service.Stop(); err != nil {
					logger.Root.Errorf("Could not close selenium service %v", err)
				}
			}
		}
		return DriverInfo{}, err
	}

	if wd == nil {
		logger.Root.Errorf("Could not create selenium web driver")
	}

	return DriverInfo{Driver: wd, Service: service, URL: sessionURL, Session: (*wd).SessionID(), DockerContainerID: containerID}, err
}

func startDockerContainerUsingCmd(port int) error {
	o1, e1, err := utils.ExecuteBashCommand(strings.Fields("docker ps"), nil)
	logger.Root.Infof("output when starting selenium instance:%s\n", o1)
	logger.Root.Infof("stderr when starting selenium instance:%s\n", e1)

	command := fmt.Sprintf("/usr/local/bin/docker run -d -p %d:4444 -v /dev/shm:/dev/shm selenium/standalone-firefox:3.141.59-20201119", port)
	logger.Root.Infof("Running command: `%s` to start selenium docker instance\n", command)
	o, e, err := utils.ExecuteBashCommand(strings.Fields(command), nil)
	logger.Root.Infof("output when starting selenium instance:%s\n", o)
	logger.Root.Infof("stderr when starting selenium instance:%s\n", e)

	if err != nil {
		logger.Root.Errorf("Cannot create selenium service. Error: %s", err)
		return err
	}
	return nil
}

// AttachToSession attach to the current web driver
func AttachToSession(sessionURL string, args map[string]interface{}) (wd *selenium.WebDriver, err error) {
	// Connect to the WebDriver instance running locally.
	caps := selenium.Capabilities{
		"browserName": "firefox",
	}

	f := firefox.Capabilities{}
	f.Args = append(f.Args, "--disable-infobars")
	f.Args = append(f.Args, "--disable-extensions")
	f.Args = append(f.Args, "--disable-gpu")
	showGUI := false
	if args != nil {
		if v, ok := args["showGUIBrowser"]; ok {
			if show, ok := v.(bool); ok && show {
				showGUI = true
			}
		}
	}
	if !showGUI {
		f.Args = append(f.Args, "--headless")
	}
	f.Args = append(f.Args, "--disable-popup-blocking")
	f.Args = append(f.Args, "--profile-directory=\""+os.TempDir()+"\"")

	f.Args = append(f.Args, "--ignore-certificate-errors")
	f.Args = append(f.Args, "--disable-plugins-discovery")
	f.Args = append(f.Args, "--incognito")
	f.Args = append(f.Args, "--width=1080")
	f.Args = append(f.Args, "--height=720")
	// f.Args = append(f.Args, "user_agent = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10.11; rv:46.0) Gecko/20100101 Firefox/46.0'")
	if f.Prefs == nil {
		f.Prefs = make(map[string]interface{})
	}

	f.Prefs["geo.enabled"] = false
	if args != nil {
		if host, ok := args["proxyHost"]; ok && host != "" {
			if port, ok := args["proxyPort"]; ok && port != "" {
				f.Prefs["network.proxy.type"] = 1
				f.Prefs["network.proxy.socks_version"] = 5
				f.Prefs["network.proxy.socks"] = fmt.Sprintf("%v", host)
				f.Prefs["network.proxy.socks_port"], _ = strconv.Atoi(fmt.Sprint(port))
			}
		}
		//if proxyHost+proxyPort != "" {
		//	caps.AddProxy(selenium.Proxy{
		//		Type:         selenium.Manual,
		//		SOCKS:        fmt.Sprintf("%v:%v", proxyHost, proxyPort),
		//		SOCKSVersion: 5,
		//	})
		//}
	}
	// f.Prefs["browser.cache.disk.enable"] = false
	// f.Prefs["browser.cache.memory.enable"] = false
	// f.Prefs["browser.cache.memory.enable"] = false
	// f.Prefs["browser.cache.memory.enable"] = false

	caps.AddFirefox(f)

	wdriver, err := selenium.NewRemote(caps, sessionURL)
	if err != nil {
		logger.Root.Errorf("Error when create new selenium remote: %s\n", err)
		return nil, err
	}
	logger.Root.Infof("Creating selenium session OK\n")
	return &wdriver, nil
}

// SaveDrivers stores the drivers to file
func SaveDrivers() {
	file, _ := json.MarshalIndent(drivers, "", " ")
	homeDir, err := homedir.Dir()
	if err != nil {
		return
	}
	_ = ioutil.WriteFile(filepath.Join(homeDir, ".selenium.dat"), file, 0644)
}

// DriverIsAlive checks if a driver is alive or not
func (driver DriverInfo) DriverIsAlive() (isAlive bool) {
	if status, err := (*driver.Driver).Status(); err == nil {
		if status.Ready {
			return true
		}
	}
	return false
}

// DriverIsAlive checks if a driver is alive or not
func (driver DriverInfo) HasWindowHanle() (hasWindow bool) {
	if driver.Driver == nil {
		return false
	}

	handles, err := (*driver.Driver).WindowHandles()
	if err != nil {
		return false
	}

	return len(handles) > 0
}
