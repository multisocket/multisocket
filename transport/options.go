package transport

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameMaxRecvSize optionName = iota
)

// Options
var (
	OptionMaxRecvSize = options.NewIntOption(optionNameMaxRecvSize)
)
