package tcp

import (
	"github.com/webee/multisocket"
)

type optionName int

const (
	optionNameNoDelay optionName = iota
	optionNameKeeyAlive
	optionNameKeepAliveTime
)

// Options
var (
	OptionNoDelay       = multisocket.NewBoolOption(optionNameNoDelay)
	OptionKeepAlive     = multisocket.NewBoolOption(optionNameKeeyAlive)
	OptionKeepAliveTime = multisocket.NewTimeDurationOption(optionNameKeepAliveTime)
)
