package socket

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameTTL = iota
)

// Options
var (
	OptionTTL = options.NewIntOption(optionNameTTL)
)
