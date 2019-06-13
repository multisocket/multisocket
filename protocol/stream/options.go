package stream

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameStreamQueueSize optionName = iota
	optionNameConnRecvQueueSize
	optionNameConnKeepAliveIdle
	optionNameConnKeepAliveInteval
	optionNameConnKeepAliveProbes
)

// Options
var (
	OptionStreamQueueSize       = options.NewIntOption(optionNameStreamQueueSize)
	OptionConnRecvQueueSize     = options.NewIntOption(optionNameConnRecvQueueSize)
	OptionConnKeepAliveIdle     = options.NewTimeDurationOption(optionNameConnKeepAliveIdle)
	OptionConnKeepAliveInterval = options.NewTimeDurationOption(optionNameConnKeepAliveInteval)
	OptionConnKeepAliveProbes   = options.NewIntOption(optionNameConnKeepAliveProbes)
)
