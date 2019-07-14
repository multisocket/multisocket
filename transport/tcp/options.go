package tcp

import (
	"time"

	"github.com/multisocket/multisocket/transport"

	"github.com/multisocket/multisocket/options"
)

type (
	tcpOptions struct {
		NoDelay         options.BoolOption
		KeepAlive       options.BoolOption
		KeepAlivePeriod options.TimeDurationOption
		ReadBuffer      options.IntOption
		WriteBuffer     options.IntOption
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = append(transport.OptionDomains, "tcp")
	// Options for tcp
	Options = tcpOptions{
		NoDelay:         options.NewBoolOption(true),
		KeepAlive:       options.NewBoolOption(true),
		KeepAlivePeriod: options.NewTimeDurationOption(time.Duration(0)),
		ReadBuffer:      options.NewIntOption(0),
		WriteBuffer:     options.NewIntOption(0),
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
