package skyenv

import (
	"time"
)

// Constants for default services.
const (
	DefaultTpDiscAddr      = "http://transport.discovery.skywire.skycoin.com"
	DefaultDmsgDiscAddr    = "http://dmsg.discovery.skywire.skycoin.com"
	DefaultRouteFinderAddr = "http://routefinder.skywire.skycoin.com"
	DefaultSetupPK         = "026c5a07de617c5c488195b76e8671bf9e7ee654d0633933e202af9e111ffa358d"
)

// Constants for testing deployment.
const (
	TestTpDiscAddr      = "http://transport.discovery.skywire.cc"
	TestDmsgDiscAddr    = "http://dmsg.discovery.skywire.cc"
	TestRouteFinderAddr = "http://routefinder.skywire.cc"
)

// Common app constants.
const (
	AppProtocolVersion = "0.0.1"
)

// Default dmsg ports.
const (
	DmsgSetupPort      = uint16(36)  // Listening port of a setup node.
	DmsgAwaitSetupPort = uint16(136) // Listening port of a visor node for setup operations.
	DmsgTransportPort  = uint16(45)  // Listening port of a visor node for incoming transports.
)

// Default dmsgpty constants.
const (
	DefaultDmsgPtyPort    = uint16(233)
	DefaultDmsgPtyCLINet  = "unix"
	DefaultDmsgPtyCLIAddr = "/tmp/dmsgpty.sock"
)

// Default skywire app constants.
const (
	SkychatName    = "skychat"
	SkychatVersion = "1.0"
	SkychatPort    = uint16(1)
	SkychatAddr    = ":8000"

	SkyproxyName    = "socksproxy"
	SkyproxyVersion = "1.0"
	SkyproxyPort    = uint16(3)

	SkyproxyClientName        = "socksproxy-client"
	SkyproxyClientVersion     = "1.0"
	SkyproxyClientPort        = uint16(13)
	SkyproxyClientAddr        = ":1080"
	SkyproxyClientTimeout     = 5 * time.Second
	SkyproxyReconnectInterval = 1 * time.Second
	SkyproxyRetryTimes        = 0
	SkyproxyRetryFactor       = 1
)
