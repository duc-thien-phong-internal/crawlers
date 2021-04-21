package basecrawler

import "errors"

const (
	// ServerLoginURL the url to login to our server
	ServerLoginURL = "%s://%s:%d/software/login"

	// ServerRefreshURL the url to refresh token from our server
	ServerRefreshURL = "%s://%s:%d/software/refresh-token"

	// BaseServerURL the base url
	BaseServerURL = "%s:%d"

	// ChannelSocketPath the path in the URL to connect to the socket
	ChannelSocketPath = "/software/chat/channel/%v"

	ServerFetchAccountURL = "%s://%s:%d/software/list-accs?loginTo=%v"
	ServerFetchConfigURL  = "%s://%s:%d/software/get-config?loginTo=%v"
	ServerProfilesURL     = "%s://%s:%d/software/new"

	// config Config
)

// MaxNumOldAppIDItems is the maximum number of historical application ID that we should keep track
// to check if an application is extracted or not
const MaxNumOldAppIDItems = 80000

var (
	// ErrNoUsableAccount means there is no usable account
	ErrNoUsableAccount = errors.New("There is no usable account")

	// ErrInvalidUsernamePassword means the wrong username or password
	ErrInvalidUsernamePassword = errors.New("Invalid username or password")
	// ErrAccountIsInUsed means the account is in used
	ErrAccountIsInUsed = errors.New("Account is in used")

	// ErrorMaximumNumActiveSessionReached means the maximum sessions is eached
	ErrorMaximumNumActiveSessionReached = errors.New("Maximum number of active session is reached")

	// ErrCouldNotCreateService means the service cannot be created
	ErrCouldNotCreateService = errors.New("Could not create service")

	// ErrMaintain means the website is in the maintenance mode
	ErrMaintain = errors.New("Is in maintenance mode")
)

var configFileName = "config.yml"

var ColorNames = []string{
	"black",
	"white",
	"gray",
	"brown",
	"red",
	"pink",
	"crimson",
	"carnelian",
	"orange",
	"yellow",
	"ivory",
	"cream",
	"green",
	"viridian",
	"aquamarine",
	"cyan",
	"blue",
	"cerulean",
	"azure",
	"indigo",
	"navy",
	"violet",
	"purple",
	"lavender",
	"magenta",
	"rainbow",
	"iridescent",
	"spectrum",
	"prism",
	"bold",
	"vivid",
	"pale",
	"clear",
	"glass",
	"translucent",
	"misty",
	"dark",
	"light",
	"gold",
	"silver",
	"copper",
	"bronze",
	"steel",
	"iron",
	"brass",
	"mercury",
	"zinc",
	"chrome",
	"platinum",
	"titanium",
	"nickel",
	"lead",
	"pewter",
	"rust",
	"metal",
	"stone",
	"quartz",
	"granite",
	"marble",
	"alabaster",
	"agate",
	"jasper",
	"pebble",
	"pyrite",
	"crystal",
	"geode",
	"obsidian",
	"mica",
	"flint",
	"sand",
	"gravel",
	"boulder",
	"basalt",
	"ruby",
	"beryl",
	"scarlet",
	"citrine",
	"sulpher",
	"topaz",
	"amber",
	"emerald",
	"malachite",
	"jade",
	"abalone",
	"lapis",
	"sapphire",
	"diamond",
	"peridot",
	"gem",
	"jewel",
	"bevel",
	"coral",
	"jet",
	"ebony",
	"wood",
	"tree",
	"cherry",
	"maple",
	"cedar",
	"branch",
	"bramble",
	"rowan",
	"ash",
	"fir",
	"pine",
	"cactus",
	"alder",
	"grove",
	"forest",
	"jungle",
	"palm",
	"bush",
	"mulberry",
	"juniper",
	"vine",
	"ivy",
	"rose",
	"lily",
	"tulip",
	"daffodil",
	"honeysuckle",
	"fuschia",
	"hazel",
	"walnut",
	"almond",
	"lime",
	"lemon",
	"apple",
	"blossom",
}

const (
	EventConnectionToOurServerChanged = "eventConnectionToOurServerChanged"

	EventTunnelStateChanged = "eventTunnelStateChanged"
	Event
	//EventConnectedToOurServer    = "connected"
	//EventDisconnectedToOurServer = "disconnected"
)

var (
	ErrMissingField    = errors.New("Missing field")
	ErrEmptyContent    = errors.New("Empty content")
	ErrInvalidSession  = errors.New("Invalid session")
	ErrTimeout         = errors.New("Timeout")
	ErrInvalidProducer = errors.New("invalid producer")
)
