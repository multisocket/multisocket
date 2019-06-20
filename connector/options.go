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
	dialerOptionNameReconnect
	pipeOptionSendTimeout
	pipeOptionRecvTimeout
	pipeOptionCloseOnEOF
)

// Options
var (
	// Connector
	OptionConnLimit       = options.NewIntOption(optionNameConnLimit)
	PipeOptionSendTimeout = options.NewTimeDurationOption(pipeOptionSendTimeout)
	PipeOptionRecvTimeout = options.NewTimeDurationOption(pipeOptionRecvTimeout)
	PipeOptionCloseOnEOF  = options.NewBoolOption(pipeOptionCloseOnEOF)
	// Dialer
	DialerOptionMinReconnectTime = options.NewTimeDurationOption(dialerOptionNameMinReconnectTime)
	DialerOptionMaxReconnectTime = options.NewTimeDurationOption(dialerOptionNameMaxReconnectTime)
	DialerOptionDialAsync        = options.NewBoolOption(dialerOptionNameDialAsync)
	DialerOptionReconnect        = options.NewBoolOption(dialerOptionNameReconnect)
)
