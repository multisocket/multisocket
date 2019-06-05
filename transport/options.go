package transport

import (
	"github.com/webee/multisocket"
)

type optionName int

const (
	optionNameMaxRecvSize optionName = iota
)

// Options
var (
	OptionMaxRecvSize = multisocket.NewIntOption(optionNameMaxRecvSize)
)
