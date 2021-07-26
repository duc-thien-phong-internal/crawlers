package basecrawler

import (
	"github.com/duc-thien-phong/techsharedservices/models"
	"github.com/duc-thien-phong/techsharedservices/models/softwareclient"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"github.com/tebeka/selenium"
	"time"
)

func Filter(arr []models.RemoteAccount, cond func(models.RemoteAccount) bool) []models.RemoteAccount {
	result := []models.RemoteAccount{}
	for i := range arr {
		if cond(arr[i]) {
			result = append(result, arr[i])
		}
	}

	return result
}

func ContainsTag(arr []string, tag string) bool {
	for _, t := range arr {
		if t == tag {
			return true
		}
	}
	return false
}

func GenerateAppNoFromTimeStamp(currentTimeInMicro uint64, softwareClientNo softwareclient.AppNoType) uint64 {
	return currentTimeInMicro | uint64(int64(softwareClientNo)<<52)
}

func WaitUntilElementDisplay(d selenium.WebDriver, xpath string, timeoutInSec int, intervalInSec int, timeSleep int, numRetries int) error {
	return utils.RetryOperation(func() error {
		// wait the dialog SelectValue appear
		return d.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
			errElement, err := wd.FindElement(selenium.ByXPATH, xpath)
			if err != nil {
				return false, err
			}

			displayed, _ := errElement.IsDisplayed()
			return displayed, nil
		}, time.Duration(timeoutInSec)*time.Second, time.Duration(intervalInSec)*time.Second)
	}, numRetries, time.Duration(timeSleep)*time.Second)
}

func WaitUntilElementDisappear(d selenium.WebDriver, xpath string, timeoutInSec int, intervalInSec int, timeSleep int, numRetries int) error {
	return utils.RetryOperation(func() error {
		// wait the dialog SelectValue appear
		return d.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
			errElement, err := wd.FindElement(selenium.ByXPATH, xpath)
			if err != nil {
				return true, nil
			}

			displayed, _ := errElement.IsDisplayed()
			return !displayed, nil
		}, time.Duration(timeoutInSec)*time.Second, time.Duration(intervalInSec)*time.Second)
	}, numRetries, time.Duration(timeSleep)*time.Second)
}
