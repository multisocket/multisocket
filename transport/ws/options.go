package ws

import (
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
)

type (
	listenerOptions struct {
		CheckOrigin    options.BoolOption
		OriginChecker  options.AnyOption
		ExternalListen options.BoolOption
		PendingSize    options.IntOption
	}

	wsOptions struct {
		ReadBufferSize  options.IntOption
		WriteBufferSize options.IntOption
		Listener        listenerOptions
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = append(transport.OptionDomains, "ws")
	// Options for websocket
	Options = wsOptions{
		ReadBufferSize:  options.NewIntOption(4 * 1024),
		WriteBufferSize: options.NewIntOption(4 * 1024),
		Listener: listenerOptions{
			CheckOrigin:    options.NewBoolOption(false),
			OriginChecker:  options.NewAnyOption(noCheckOrigin),
			ExternalListen: options.NewBoolOption(false),
			PendingSize:    options.NewIntOption(16),
		},
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
