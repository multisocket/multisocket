package transport

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameMaxRecvMsgSize optionName = iota
	optionNameRecvRawMsgBufSize
	optionNameConnnRawMode
)

// Options
var (
	OptionMaxRecvMsgSize    = options.NewUint32Option(optionNameMaxRecvMsgSize)
	OptionRecvRawMsgBufSize = options.NewUint32Option(optionNameRecvRawMsgBufSize)
	OptionConnRawMode       = options.NewBoolOption(optionNameConnnRawMode)
)
