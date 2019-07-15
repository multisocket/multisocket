package channel

import (
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport/inproc"
)

type (
	channelOptions struct {
		BufferSize options.IntOption
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = append(inproc.OptionDomains, "channel")
	// Options for inproc
	Options = channelOptions{
		BufferSize: options.NewIntOption(16),
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
