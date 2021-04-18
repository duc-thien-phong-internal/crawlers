package osinfo

type OSType string

var (
	OSWindows OSType = "windows"
	OSLinux   OSType = "linux"
	OSMac     OSType = "macos"
)

var CurrentOS OSType = OSLinux
