package connector

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	dialerOptionNameMinReconnectTime = iota
	dialerOptionNameMaxReconnectTime
	dialerOptionNameDialAsync
)

// Options
var (
	DialerOptionMinReconnectTime = options.NewTimeDurationOption(dialerOptionNameMinReconnectTime)
	DialerOptionMaxReconnectTime = options.NewTimeDurationOption(dialerOptionNameMaxReconnectTime)
	DialerOptionDialAsync        = options.NewBoolOption(dialerOptionNameDialAsync)
)
