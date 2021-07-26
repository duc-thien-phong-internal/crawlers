// +build darwin

package main

import (
	"fmt"
	"github.com/duc-thien-phong-internal/crawlers/pkg/osinfo"
	"github.com/duc-thien-phong/techsharedservices/utils"
	"path/filepath"
	"strings"

	"github.com/kardianos/osext"
)

// GECKOFILE is the file name contains the gecko dirve
const GECKOFILE = "geckodriver"

const OS = "macos"

// CONTROLKEY is the control key on macos
const CONTROLKEY = "\ue03d"

func init() {
	curDir, _ := osext.ExecutableFolder()

	cmd := []string{"xattr", "-r", "-d", "com.apple.quarantine", filepath.Join(curDir, "drivers", "geckodriver")}
	utils.ExecuteBashCommand(cmd, nil)
	fmt.Printf("Executed command: '%s'\n", strings.Join(cmd, " "))

	cmd = []string{"xattr", "-r", "-d", "com.apple.quarantine", filepath.Join(curDir, "drivers", "selenium-server.jar")}
	utils.ExecuteBashCommand(cmd, nil)
	fmt.Printf("Executed command: '%s'\n", strings.Join(cmd, " "))

	osinfo.CurrentOS = OS
}
