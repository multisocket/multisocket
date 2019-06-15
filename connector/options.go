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
	pipeOptionSendTimeout
	pipeOptionRecvTimeout
)

// Options
var (
	OptionConnLimit              = options.NewIntOption(optionNameConnLimit)
	DialerOptionMinReconnectTime = options.NewTimeDurationOption(dialerOptionNameMinReconnectTime)
	DialerOptionMaxReconnectTime = options.NewTimeDurationOption(dialerOptionNameMaxReconnectTime)
	DialerOptionDialAsync        = options.NewBoolOption(dialerOptionNameDialAsync)
	PipeOptionSendTimeout        = options.NewTimeDurationOption(pipeOptionSendTimeout)
	PipeOptionRecvTimeout        = options.NewTimeDurationOption(pipeOptionRecvTimeout)
)
