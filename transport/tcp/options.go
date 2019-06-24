package tcp

import (
	"time"

	"github.com/webee/multisocket/transport"

	"github.com/webee/multisocket/options"
)

type (
	tcpOptions struct {
		NoDelay         options.BoolOption
		KeepAlive       options.BoolOption
		KeepAlivePeriod options.TimeDurationOption
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
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
