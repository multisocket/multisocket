package receiver

import (
	"github.com/webee/multisocket/options"
)

type (
	receiverOptions struct {
		NoRecv        options.BoolOption
		RecvQueueSize options.Uint16Option
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = []string{"receiver"}
	// Options for receiver
	Options = receiverOptions{
		NoRecv:        options.NewBoolOption(false),
		RecvQueueSize: options.NewUint16Option(64),
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
