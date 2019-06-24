package sender

import (
	"github.com/webee/multisocket/message"
	"github.com/webee/multisocket/options"
)

type (
	senderOptions struct {
		TTL            options.Uint8Option
		SendQueueSize  options.Uint16Option
		SendBestEffort options.BoolOption
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = []string{"sender"}
	// Options for sender
	Options = senderOptions{
		TTL:            options.NewUint8Option(message.DefaultMsgTTL),
		SendQueueSize:  options.NewUint16Option(64),
		SendBestEffort: options.NewBoolOption(false),
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
