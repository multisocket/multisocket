package connector

import (
	"time"

	"github.com/multisocket/multisocket/options"
)

type (
	dialerOptions struct {
		// reconnect when pipe closed
		Reconnect        options.BoolOption
		MinReconnectTime options.TimeDurationOption
		MaxReconnectTime options.TimeDurationOption
		DialAsync        options.BoolOption
	}

	pipeOptions struct {
		Raw            options.BoolOption
		RawRecvBufSize options.IntOption
		// close pipe when peer shutdown write(half-close, cause EOF)
		CloseOnEOF           options.BoolOption
		MaxRecvContentLength options.Uint32Option
	}

	connectorOptions struct {
		PipeLimit options.IntOption
		Dialer    dialerOptions
		Pipe      pipeOptions
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = []string{"Connector"}
	// Options for connector
	Options = connectorOptions{
		PipeLimit: options.NewIntOption(-1), // -1: no limit
		Dialer: dialerOptions{
			Reconnect:        options.NewBoolOption(true),
			MinReconnectTime: options.NewTimeDurationOption(100 * time.Millisecond),
			MaxReconnectTime: options.NewTimeDurationOption(8 * time.Second),
			DialAsync:        options.NewBoolOption(false),
		},
		Pipe: pipeOptions{
			Raw:                  options.NewBoolOption(false),
			RawRecvBufSize:       options.NewIntOption(4 * 1024),
			CloseOnEOF:           options.NewBoolOption(true),
			MaxRecvContentLength: options.NewUint32Option(128 * 1024), // 0 for no limit
		},
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
