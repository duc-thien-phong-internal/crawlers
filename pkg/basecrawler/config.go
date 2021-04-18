package basecrawler

import (
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models"
	"os"
	"path/filepath"
)

type TunnelConfig struct {
	Host       string `yaml:"host"`
	Username   string `yaml:"username"`
	SSHPort    int    `yaml:"port"`
	Password   string `yaml:"password"`
	PrivateKey string `yaml:"privateKey"`
	LocalPort  int    `yaml:"localPort"`
	RemotePort int    `yaml:"remotePort"`
}

type App struct {
	Command string `yaml:"command"`

	Host            string   `yaml:"host"`
	Port            uint64   `yaml:"port"`
	EnvironmentVars []string `yaml:"envVars"`
}

type NsqConfig struct {
	MyNsqID     string
	SystemNsqID string
	MainChannel string
}

type OurServer struct {
	Scheme   string `yaml:"scheme"`
	Host     string `yaml:"host"`
	Port     int64  `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Config struct {
	WorkerConfigs models.DataClientConfig `yaml:"dataConfig"`

	// CurentStartingCrawlingTime time.Time `yaml:"currentStartingCrawlingTime"`
	CurrentCrawlingPage int `yaml:"currentPage"`

	// Our server
	OServer OurServer `yaml:"oServer"`

	// Their server
	App App `yaml:"app"`

	Tunnel TunnelConfig `yaml:"tunnel"`

	NsqConfig NsqConfig `yaml:"nsq"`
}

func getCurrentPath() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return ""
	}
	logger.Root.Infof(" Current path:%s\n", dir)

	return dir
}
