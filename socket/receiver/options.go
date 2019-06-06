package receiver

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameRecvQueueSize optionName = iota
)

// Options
var (
	OptionRecvQueueSize = options.NewUint16Option(optionNameRecvQueueSize)
)
