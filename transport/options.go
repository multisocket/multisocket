package transport

import (
	"github.com/webee/multisocket/options"
)

type (
	transportOptions struct {
		MaxRecvMsgSize options.Uint32Option
		RawRecvBufSize options.Uint32Option
		RawMode        options.BoolOption
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = []string{"transport"}
	// Options for transport
	Options = transportOptions{
		MaxRecvMsgSize: options.NewUint32Option(uint32(32 * 1024)), // 0 for no limit
		RawRecvBufSize: options.NewUint32Option(uint32(4 * 1024)),
		RawMode:        options.NewBoolOption(false),
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
