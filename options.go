package multisocket

import (
	"time"

	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
)

type (
	socketOptions struct {
		NoRecv           options.BoolOption // silently drop received messages
		RecvQueueSize    options.Uint16Option
		NoSend           options.BoolOption // silently drop sended messages
		SendQueueSize    options.Uint16Option
		SendTTL          options.Uint8Option
		SendBestEffort   options.BoolOption
		SendCloseTimeout options.TimeDurationOption
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = []string{"Socket"}
	// Options for receiver
	Options = socketOptions{
		NoRecv:           options.NewBoolOption(false),
		RecvQueueSize:    options.NewUint16Option(64),
		NoSend:           options.NewBoolOption(false),
		SendQueueSize:    options.NewUint16Option(64),
		SendTTL:          options.NewUint8Option(message.DefaultMsgTTL),
		SendBestEffort:   options.NewBoolOption(false),
		SendCloseTimeout: options.NewTimeDurationOption(5 * time.Second),
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}
