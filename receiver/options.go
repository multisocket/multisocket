package receiver

import (
	"github.com/webee/multisocket/options"
)

type (
	receiverOptions struct {
		NoRecv               options.BoolOption
		RecvQueueSize        options.Uint16Option
		MaxRecvContentLength options.Uint32Option
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = []string{"receiver"}
	// Options for receiver
	Options = receiverOptions{
		NoRecv:               options.NewBoolOption(false),
		RecvQueueSize:        options.NewUint16Option(64),
		MaxRecvContentLength: options.NewUint32Option(128 * 1024), // 0 for no limit
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
