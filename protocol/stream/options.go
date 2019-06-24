package stream

import (
	"time"

	"github.com/webee/multisocket/options"
)

type (
	streamOptions struct {
		StreamQueueSize       options.IntOption
		ConnRecvQueueSize     options.IntOption
		Acceptable            options.BoolOption
		ConnKeepAliveIdle     options.TimeDurationOption
		ConnKeepAliveInterval options.TimeDurationOption
		ConnKeepAliveProbes   options.IntOption
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = []string{"stream"}
	// Options from stream
	Options = streamOptions{
		StreamQueueSize:       options.NewIntOption(16),
		ConnRecvQueueSize:     options.NewIntOption(64),
		Acceptable:            options.NewBoolOption(true),
		ConnKeepAliveIdle:     options.NewTimeDurationOption(30 * time.Second),
		ConnKeepAliveInterval: options.NewTimeDurationOption(1 * time.Second),
		ConnKeepAliveProbes:   options.NewIntOption(7),
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
