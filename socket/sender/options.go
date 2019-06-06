package sender

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameTTL = iota
)

// Options
var (
	OptionTTL = options.NewUint8Option(optionNameTTL)
)
