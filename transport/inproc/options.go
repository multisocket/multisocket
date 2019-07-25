package inproc

import (
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
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
