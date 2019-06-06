package tcp

import (
	"github.com/webee/multisocket/options"
)

type optionName int

const (
	optionNameNoDelay optionName = iota
	optionNameKeeyAlive
	optionNameKeepAliveTime
)

// Options
var (
	OptionNoDelay       = options.NewBoolOption(optionNameNoDelay)
	OptionKeepAlive     = options.NewBoolOption(optionNameKeeyAlive)
	OptionKeepAliveTime = options.NewTimeDurationOption(optionNameKeepAliveTime)
)
