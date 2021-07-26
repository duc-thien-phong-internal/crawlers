// +build linux

package main

import "github.com/tebeka/selenium"
import "github.com/duc-thien-phong-internal/crawlers/pkg/osinfo"

const GECKOFILE = "geckodriver"
const CONTROLKEY = selenium.ControlKey
const OS = "linux"

func init() {
	osinfo.CurrentOS = OS
}
