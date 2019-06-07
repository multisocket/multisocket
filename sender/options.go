package sender

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameTTL optionName = iota
	optionNameSendQueueSize
	optionNameSendDeadline
)

// Options
var (
	OptionTTL           = options.NewUint8Option(optionNameTTL)
	OptionSendQueueSize = options.NewUint16Option(optionNameSendQueueSize)
	OptionSendDeadline  = options.NewTimeDurationOption(optionNameSendDeadline)
)
