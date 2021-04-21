// +build linux

package main

import "github.com/tebeka/selenium"

const GECKOFILE = "geckodriver"
const CONTROLKEY = selenium.ControlKey
const OS = "linux"

func init() {
	osinfo.CurrentOS = OS
}
