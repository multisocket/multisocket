package options

import (
	"errors"
	"sync"
	"time"
)

type (
	// Options is option set.
	Options interface {
		SetOption(opt Option, val interface{}) (err error)
		WithOption(opt Option, val interface{}) Options
		SetOptionIfNotExists(opt Option, val interface{}) (err error)
		GetOption(opt Option) (val interface{}, ok bool)
		GetOptionDefault(opt Option, def interface{}) (val interface{})
		OptionValues() []*OptionValue
	}

	// Option is an option item.
	Option interface {
		Name() interface{}
		Validate(val interface{}) error
	}

	// OptionValue option value pair
	OptionValue struct {
		Option Option
		Value  interface{}
	}

	options struct {
		sync.RWMutex
		opts       map[Option]interface{}
		accepts    map[Option]bool
		upstream   Options
		downstream Options
	}

	baseOption struct {
		name interface{}
	}

	// BoolOption is option with bool value.
	BoolOption interface {
		Option
		Value(val interface{}) bool
	}

	boolOption struct {
		baseOption
	}

	// TimeDurationOption is option with time duration value.
	TimeDurationOption interface {
		Option
		Value(val interface{}) time.Duration
	}

	timeDurationOption struct {
		baseOption
	}

	// IntOption is option with int value.
	IntOption interface {
		Option
		Value(val interface{}) int
	}

	intOption struct {
		baseOption
	}

	// UInt8Option is option with uint8 value.
	UInt8Option interface {
		Option
		Value(val interface{}) uint8
	}

	uint8Option struct {
		baseOption
	}
)

// errors
var (
	ErrInvalidOptionValue = errors.New("invalid option value")
	ErrUnsupportedOption  = errors.New("unsupported option")
)

// NewOptions create an option set.
func NewOptions() Options {
	return &options{
		opts: make(map[Option]interface{}),
	}
}

// NewOptionsWithUpDownStreamsAndAccepts create an option set with up/down streams and accepts.
func NewOptionsWithUpDownStreamsAndAccepts(upstream, downstream Options, accepts ...Option) Options {
	options := &options{
		opts:       make(map[Option]interface{}),
		upstream:   upstream,
		downstream: downstream,
		accepts:    make(map[Option]bool),
	}

	for _, opt := range accepts {
		options.accepts[opt] = true
	}

	return options
}

// NewOptionsWithAccepts create an option set with accepts.
func NewOptionsWithAccepts(accepts ...Option) Options {
	return NewOptionsWithUpDownStreamsAndAccepts(nil, nil, accepts...)
}

func (opts *options) acceptOption(opt Option) bool {
	return opts.accepts == nil || opts.accepts[opt]
}

// SetOption add an option value.
func (opts *options) SetOption(opt Option, val interface{}) (err error) {
	if err = opt.Validate(val); err != nil {
		return
	}

	if !opts.acceptOption(opt) {
		if opts.downstream == nil {
			err = ErrUnsupportedOption
			return
		}
		// pass to downstream
		return opts.downstream.SetOption(opt, val)
	}

	opts.Lock()
	defer opts.Unlock()

	opts.opts[opt] = val
	return
}

// WithOption set an option value.
func (opts *options) WithOption(opt Option, val interface{}) Options {
	opts.SetOption(opt, val)
	return opts
}

func (opts *options) SetOptionIfNotExists(opt Option, val interface{}) (err error) {
	if err = opt.Validate(val); err != nil {
		return
	}

	if !opts.acceptOption(opt) {
		if opts.downstream == nil {
			err = ErrUnsupportedOption
			return
		}
		// pass to downstream
		return opts.downstream.SetOptionIfNotExists(opt, val)
	}

	opts.Lock()
	defer opts.Unlock()

	if _, ok := opts.opts[opt]; !ok {
		opts.opts[opt] = val
	}
	return
}

// GetOption get an option value.
func (opts *options) GetOption(opt Option) (val interface{}, ok bool) {
	if opts.acceptOption(opt) {
		opts.RLock()
		defer opts.RUnlock()
		val, ok = opts.opts[opt]
		return
	} else if opts.upstream != nil {
		// pass to upstream
		return opts.upstream.GetOption(opt)
	}

	return
}

// GetOptionDefault get an option value with default.
func (opts *options) GetOptionDefault(opt Option, def interface{}) (val interface{}) {
	var ok bool
	if val, ok = opts.GetOption(opt); !ok {
		val = def
	}
	return
}

func (opts *options) OptionValues() (res []*OptionValue) {
	opts.RLock()
	defer opts.RUnlock()

	res = make([]*OptionValue, len(opts.opts))
	idx := 0
	for opt, val := range opts.opts {
		res[idx] = &OptionValue{opt, val}
		idx++
	}
	return
}

func (o *baseOption) Name() interface{} {
	return o.name
}

// NewBoolOption create a bool option
func NewBoolOption(name interface{}) BoolOption {
	return &boolOption{baseOption{name}}
}

// Validate validate the option value
func (o *boolOption) Validate(val interface{}) error {
	if _, ok := val.(bool); !ok {
		return ErrInvalidOptionValue
	}
	return nil
}

// Value get option's value, must ensure option value is not empty
func (o *boolOption) Value(val interface{}) bool {
	return val.(bool)
}

// NewTimeDurationOption create a time duration option
func NewTimeDurationOption(name interface{}) TimeDurationOption {
	return &timeDurationOption{baseOption{name}}
}

// Validate validate the option value
func (o *timeDurationOption) Validate(val interface{}) error {
	if _, ok := val.(time.Duration); !ok {
		return ErrInvalidOptionValue
	}
	return nil
}

// Value get option's value, must ensure option value is not empty
func (o *timeDurationOption) Value(val interface{}) time.Duration {
	return val.(time.Duration)
}

// NewIntOption create an int option
func NewIntOption(name interface{}) IntOption {
	return &intOption{baseOption{name}}
}

// Validate validate the option value
func (o *intOption) Validate(val interface{}) error {
	if _, ok := val.(int); !ok {
		return ErrInvalidOptionValue
	}
	return nil
}

// Value get option's value, must ensure option value is not empty
func (o *intOption) Value(val interface{}) int {
	return val.(int)
}

// NewUInt8Option create an uint8 option
func NewUInt8Option(name interface{}) UInt8Option {
	return &uint8Option{baseOption{name}}
}

// Validate validate the option value
func (o *uint8Option) Validate(val interface{}) error {
	if _, ok := val.(uint8); !ok {
		return ErrInvalidOptionValue
	}
	return nil
}

// Value get option's value, must ensure option value is not empty
func (o *uint8Option) Value(val interface{}) uint8 {
	return val.(uint8)
}
