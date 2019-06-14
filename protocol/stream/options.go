package stream

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameStreamQueueSize optionName = iota
	optionNameConnRecvQueueSize
	optionNameAcceptable
	optionNameConnKeepAliveIdle
	optionNameConnKeepAliveInteval
	optionNameConnKeepAliveProbes
)

// Options
var (
	OptionStreamQueueSize       = options.NewIntOption(optionNameStreamQueueSize)
	OptionConnRecvQueueSize     = options.NewIntOption(optionNameConnRecvQueueSize)
	OptionAcceptable            = options.NewBoolOption(optionNameAcceptable)
	OptionConnKeepAliveIdle     = options.NewTimeDurationOption(optionNameConnKeepAliveIdle)
	OptionConnKeepAliveInterval = options.NewTimeDurationOption(optionNameConnKeepAliveInteval)
	OptionConnKeepAliveProbes   = options.NewIntOption(optionNameConnKeepAliveProbes)
)
