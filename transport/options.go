package transport

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameMaxRecvMsgSize optionName = iota
)

// Options
var (
	OptionMaxRecvMsgSize = options.NewUint32Option(optionNameMaxRecvMsgSize)
)
