package transport

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameMaxRecvMsgSize optionName = iota
	optionNameConnnRawMode
)

// Options
var (
	OptionMaxRecvMsgSize = options.NewUint32Option(optionNameMaxRecvMsgSize)
	OptionConnRawMode    = options.NewBoolOption(optionNameConnnRawMode)
)
