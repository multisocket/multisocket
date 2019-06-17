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
	// Connector
	OptionConnLimit       = options.NewIntOption(optionNameConnLimit)
	PipeOptionSendTimeout = options.NewTimeDurationOption(pipeOptionSendTimeout)
	PipeOptionRecvTimeout = options.NewTimeDurationOption(pipeOptionRecvTimeout)
	// Dialer
	DialerOptionMinReconnectTime = options.NewTimeDurationOption(dialerOptionNameMinReconnectTime)
	DialerOptionMaxReconnectTime = options.NewTimeDurationOption(dialerOptionNameMaxReconnectTime)
	DialerOptionDialAsync        = options.NewBoolOption(dialerOptionNameDialAsync)
)
