package connector

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameConnLimit optionName = iota
	dialerOptionNameMinReconnectTime
	dialerOptionNameMaxReconnectTime
	dialerOptionNameDialAsync
)

// Options
var (
	OptionConnLimit              = options.NewIntOption(optionNameConnLimit)
	DialerOptionMinReconnectTime = options.NewTimeDurationOption(dialerOptionNameMinReconnectTime)
	DialerOptionMaxReconnectTime = options.NewTimeDurationOption(dialerOptionNameMaxReconnectTime)
	DialerOptionDialAsync        = options.NewBoolOption(dialerOptionNameDialAsync)
)
