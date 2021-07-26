package main

import (
	"encoding/gob"
	"flag"
	"github.com/duc-thien-phong-internal/crawlers/libs/cicblib"
	"github.com/duc-thien-phong-internal/crawlers/pkg/basecrawler"
	"github.com/duc-thien-phong-internal/crawlers/pkg/osinfo"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"github.com/joho/godotenv"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	defer os.Exit(0)

	logger.InitLogger(nil)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	cwd, _ := os.Executable()
	err := godotenv.Load(path.Join(filepath.Dir(cwd), ".env"))

	flag.Parse()
	args := flag.Args()

	if err != nil {
		logger.Root.Warnf("Error when loading .env file %s\n", err)
	}

	adapter := cicblib.CreateAdapter()
	if len(args) > 0 {
		if v, err := strconv.Atoi(args[0]); err == nil {
			logger.Root.Infof("Start crawling from ID %d", v)
			adapter.StartingID = v
		}
	}
	if len(args) > 1 {
		if v, err := strconv.Atoi(args[1]); err == nil {
			logger.Root.Infof("Start crawling from ID %d", v)
			adapter.StopID = v
		}
	}
	//if adapter.StartingID == 0 {
	//	file, _ := os.Open("lastid")
	//
	//	defer file.Close()
	//
	//	decoder := gob.NewDecoder(file)
	//
	//	decoder.Decode(&(adapter.StartingID))
	//}
	//if adapter.StartingID == 0 {
	//	logger.Root.Errorf("Could not start application because the starting id  = 0")
	//	return
	//}

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
		logger.Root.Infof("Ctrl+C is pressed. Terminating....")
		app.Dispose()
		if osinfo.CurrentOS == osinfo.OSWindows {
			utils.ExecuteBashCommand(strings.Split("taskkill /F /IM geckodriver.exe", " "), nil)
			utils.ExecuteBashCommand(strings.Split("taskkill /F /IM firefox.exe", " "), nil)
		}

		file, _ := os.Create("lastid")

		defer file.Close()

		encoder := gob.NewEncoder(file)

		encoder.Encode(adapter.CurrentID)
		os.Exit(1)
	}()
	defer func() {
		app.Dispose()
	}()

	app.Start(adapter, "cicb.ids", false, func() {

	})

}
