package pega7lib

const (
	BaseURL       = "https://fecrdt-los-prod1-internal.pegacloud.io/prweb/PRWebLDAP2/@id@/"
	LoginURL      = BaseURL + "!STANDARD"
	PageReportURL = BaseURL + "!TABTHREAD1?pyActivity=Rule-Shortcut.pxRunShortcut&InsKey=RULE-SHORTCUT%20FECREDITSUPERVISORREPORTS%20S!ALL!NUMBEROFCURRENTAPPLICATIONSBYSTAGE%20%2320170124T034251.673%20GMT&pzHarnessID=HID11B09BA5B7D952B0EA333D6CD70261BF"
)

const (
	TimeFilterToday         = "Today"
	TimeFilterYesterday     = "Yesterday"
	TimeFilterTomorrow      = "Tomorrow"
	TimeFilterLast90Days    = "Last%2090%20Days"
	TimeFilterLast30Days    = "Last%2030%20Days"
	TimeFilterLast120Days   = "Last%20120%20Days"
	TimeFilterPreviousWeek  = "Previous%20Week"
	TimeFilterCurrentWeek   = "Current%20Week"
	TimeFilterPreviousMonth = "Current%20Week"
)

type DataType int

const (
	TypeString DataType = iota
	TypeStringPointer
	TypeFloat64
	TypeInt64
	TypeInt32
	TypeInt8
	TypeUint8
	TypeUint64
	TypeShortDate
	TypeDateWithTime
	TypeUint
	TypeInt
	TypeBoolean
	TypeMarriage
)

type AdditionalURLData struct {
	URL     string
	Options map[string]interface{}
}
