package basecrawler

import (
	"encoding/json"
	"github.com/duc-thien-phong/techsharedservices/commands"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models"
	nsq_models "github.com/duc-thien-phong/techsharedservices/nsq/models"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"strings"
)

func (mc *Application) handleRequestChangingAppStatus(msg nsq_models.Message) {
	changeAppStatusReq := commands.CommandReqChangeAppStatus{}

	body := msg.GetRawDataFromBody()
	if err := json.Unmarshal([]byte(body), &changeAppStatusReq); err == nil {

		mc.config.WorkerConfigs.Crawler.RequestedStatus = changeAppStatusReq.CrawlerStatus
		mc.config.WorkerConfigs.Checker.RequestedStatus = changeAppStatusReq.CheckerStatus
		mc.compareRunningModesAndStartCrawlerAndChecker()

		logger.Root.Infof("Set new app status to %s\n", changeAppStatusReq.CrawlerStatus)
		var responseObj commands.CommandResponseChangeAppStatus
		if err := mc.writeCurrentConfig(); err != nil {
			logger.Root.Errorf("Could not write the new app status\n")
			responseObj = commands.CommandResponseChangeAppStatus{
				Success:      false,
				ErrorMessage: err.Error(),
			}
		} else {
			responseObj = commands.CommandResponseChangeAppStatus{
				Success:      true,
				ErrorMessage: "",
			}
		}
		responseBytes, _ := json.Marshal(responseObj)
		mc.SendResponseCommand(msg.Command.ID, msg.Command.SubType, string(responseBytes), msg.Channel)
		mc.sendPingMessage()
	} else {
		logger.Root.Errorf("The request changing application status failed: %s\n", err)
	}
}

// compare the requestedStatus and the current status
// then give decision to stop or start all crawlers/checkers
func (mc *Application) compareRunningModesAndStartCrawlerAndChecker() {
	newConfig := &mc.config.WorkerConfigs

	if (newConfig.Crawler.Status == models.WorkerStopping || newConfig.Crawler.Status == models.WorkerStopped) &&
		(newConfig.Crawler.RequestedStatus == models.WorkerStarting || newConfig.Crawler.RequestedStatus == models.WorkerRunning) {
		go func() {
			mc.client.StartOrStopAllCrawlers(true)
			newConfig.Crawler.Status = models.WorkerRunning
			mc.writeCurrentConfig()
			//mc.sendPingMessage()
		}()
		// newConfig.Crawler.RunningMode = models.RunningModeManually
		// logger.Root.Infof("Change runnning mode of crawler to manually")
	}
	if (newConfig.Checker.Status == models.WorkerStopping || newConfig.Checker.Status == models.WorkerStopped) &&
		(newConfig.Checker.RequestedStatus == models.WorkerStarting || newConfig.Checker.RequestedStatus == models.WorkerRunning) {
		go func() {
			mc.client.StartOrStopAllCheckers(true)
			newConfig.Checker.Status = models.WorkerRunning
			mc.writeCurrentConfig()
			//mc.sendPingMessage()
		}()
		// newConfig.Checker.RunningMode = models.RunningModeManually
		// logger.Root.Infof("Change runnning mode of checker to manually")
	}

	if (newConfig.Crawler.Status == models.WorkerStarting || newConfig.Crawler.Status == models.WorkerRunning) &&
		(newConfig.Crawler.RequestedStatus == models.WorkerStopping || newConfig.Crawler.RequestedStatus == models.WorkerStopped) {
		go func() {
			mc.client.StartOrStopAllCrawlers(false)
			newConfig.Crawler.Status = models.WorkerStopped
			mc.writeCurrentConfig()
			//mc.sendPingMessage()
		}()
		// newConfig.Crawler.RunningMode = models.RunningModeManually
		// logger.Root.Infof("Change runnning mode of crawler to manually")
	}
	if (newConfig.Checker.Status == models.WorkerStarting || newConfig.Checker.Status == models.WorkerRunning) &&
		(newConfig.Checker.RequestedStatus == models.WorkerStopping || newConfig.Checker.RequestedStatus == models.WorkerStopped) {
		go func() {
			mc.client.StartOrStopAllCheckers(false)
			newConfig.Checker.Status = models.WorkerStopped
			mc.writeCurrentConfig()
			//mc.sendPingMessage()
		}()
		// newConfig.Checker.RunningMode = models.RunningModeManually
		// logger.Root.Infof("Change runnning mode of checker to manually")
	}

}

func (mc *Application) handleRequestChangingAppConfig(msg nsq_models.Message) {
	req := commands.CommandReqChangeAppConfig{}

	body := msg.GetRawDataFromBody()
	if err := json.Unmarshal([]byte(body), &req); err == nil {
		cf := req.Config
		mc.config.WorkerConfigs = cf
		logger.Root.Infof("Change app config to %v\n", mc.config)
		mc.compareRunningModesAndStartCrawlerAndChecker()

		var responseObj commands.CommandResponseChangeAppStatus
		if err := mc.writeCurrentConfig(); err != nil {
			logger.Root.Errorf("Could not write the new app status\n")
			responseObj = commands.CommandResponseChangeAppStatus{
				Success:      false,
				ErrorMessage: err.Error(),
			}
		} else {
			responseObj = commands.CommandResponseChangeAppStatus{
				Success:      true,
				ErrorMessage: "",
			}
		}
		responseBytes, _ := json.Marshal(responseObj)
		go mc.SendResponseCommand(msg.Command.ID, msg.Command.SubType, string(responseBytes), msg.Channel)
	} else {
		logger.Root.Errorf("The request changing application status failed: %s\n", err)
	}
}

func (mc *Application) handleRequestRunCommand(msg nsq_models.Message) {
	req := commands.CommandReqExecuteCommand{}

	body := msg.GetRawDataFromBody()
	if err := json.Unmarshal([]byte(body), &req); err == nil {
		executableCommand := req.ExecutableCommand

		logger.Root.Infof("Execute command :%s\n", executableCommand)
		var responseObj commands.CommandRespExecuteCommand
		if _, _, err := utils.ExecuteBashCommand(strings.Fields(executableCommand), map[string]string{}); err != nil {
			logger.Root.Errorf("Could not execute command\n")
			responseObj = commands.CommandRespExecuteCommand{
				Success:      false,
				ErrorMessage: err.Error(),
			}
		} else {
			responseObj = commands.CommandRespExecuteCommand{
				Success:      true,
				ErrorMessage: "",
			}
		}
		responseBytes, _ := json.Marshal(responseObj)
		mc.SendResponseCommand(msg.Command.ID, msg.Command.SubType, string(responseBytes), msg.Channel)
	} else {
		logger.Root.Errorf("The request changing application status failed: %s\n", err)
	}
}
