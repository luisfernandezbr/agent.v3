package rpcdef

import "github.com/hashicorp/go-plugin"

// handshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "PLUGIN",
	MagicCookieValue: "pinpoint-agent-plugin",
}

var PluginMap = map[string]plugin.Plugin{
	"integration": &IntegrationPlugin{},
}
