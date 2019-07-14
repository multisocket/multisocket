package inproc

import (
	"github.com/multisocket/multisocket/transport"
	"github.com/multisocket/multisocket/options"
)

type (
	inprocOptions struct {
		ReadBuffer options.IntOption
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = append(transport.OptionDomains, "inproc")
	// Options for inproc
	Options = inprocOptions{
		ReadBuffer: options.NewIntOption(8 * 1024),
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
