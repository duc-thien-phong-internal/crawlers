package basecrawler

import "github.com/duc-thien-phong/techsharedservices/models"

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
