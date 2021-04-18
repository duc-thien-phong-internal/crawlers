package basecrawler

import (
	"github.com/duc-thien-phong/techsharedservices/models"
	"net/http/cookiejar"
	"time"
)

type UsefulAccountInfo struct {
	Token               string
	CookieJar           *cookiejar.Jar
	Account             *models.RemoteAccount
	UserAgent           string
	HasAccessedHomepage bool
	LoginAtleastOnce    bool
	LastTimeLogin       time.Time
	Type                AccountType

	// this data can be cookie, or token, or any other useful data
	OtherData map[string]interface{}
}
