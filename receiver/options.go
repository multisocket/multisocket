package receiver

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameRecvQueueSize optionName = iota
	optionNameNoRecv
)

// Options
var (
	OptionRecvQueueSize = options.NewUint16Option(optionNameRecvQueueSize)
	OptionNoRecv        = options.NewBoolOption(optionNameNoRecv)
)
