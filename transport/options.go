package transport

import (
	"github.com/multisocket/multisocket/options"
)

type (
	transportOptions struct {
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = []string{"transport"}
	// Options for transport
	Options = transportOptions{}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
