package main

import (
	"github.com/duc-thien-phong-internal/crawlers/libs/pega7blib"
	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong-internal/crawlers/pkg/osinfo"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"github.com/golang/glog"
	"github.com/joho/godotenv"
	"github.com/zserge/lorca"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	defer os.Exit(0)

	logger.InitLogger(nil)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	cwd, _ := os.Executable()
	err := godotenv.Load(path.Join(filepath.Dir(cwd), ".env"))

	if err != nil {
		logger.Root.Warnf("Error when loading .env file %s\n", err)
	}

	adapter := pega7lib.CreateAdapter()

	_ = func() basecrawler.ConnectionState {
		// return connection status
		return basecrawler.StateConnected
	}

	app := basecrawler.NewApplication(nil, func() error {
		// on open tunnel function
		return nil
	}, func() error {
		// on close tunnel function
		return nil
	}, adapter.GetWorkerSoftwareSource())

	go func() {
		<-c
		glog.Infof("Ctrl+C is pressed. Terminating....")
		app.Dispose()
		if osinfo.CurrentOS == osinfo.OSWindows {
			utils.ExecuteBashCommand(strings.Split("taskkill /F /IM geckodriver.exe", " "), nil)
			utils.ExecuteBashCommand(strings.Split("taskkill /F /IM firefox.exe", " "), nil)
		}
		os.Exit(1)
	}()
	defer func() {
		app.Dispose()
	}()

	ui, _ := lorca.New("", "", 480, 320)
	defer ui.Close()

	go app.Start(adapter, "pega7.ids", false, func() {

	})

	<-ui.Done()

}
