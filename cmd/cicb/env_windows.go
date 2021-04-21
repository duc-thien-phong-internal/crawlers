// +build windows

package main

import (
	"dtp/pkg/osinfo"

	"github.com/tebeka/selenium"
)

const GECKOFILE = "geckodriver.exe"
const CONTROLKEY = selenium.ControlKey
const OS = "windows"

func init() {
	osinfo.CurrentOS = OS
}
