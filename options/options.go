package options

import (
	"errors"
	"math"
	"sync"
	"time"
)

type (
	// OptionChangeHook is called when set an option value.
	OptionChangeHook func(opt Option, oldVal, newVal interface{})
	// Options is option set.
	Options interface {
		SetOption(opt Option, val interface{}) (err error)
		WithOption(opt Option, val interface{}) Options
		SetOptionIfNotExists(opt Option, val interface{}) (err error)
		GetOption(opt Option) (val interface{}, ok bool)
		GetOptionDefault(opt Option, def interface{}) (val interface{})
		OptionValues() OptionValues
		SetOptionChangeHook(hook OptionChangeHook) Options
	}

	// Option is an option item.
	Option interface {
		Name() interface{}
		Validate(val interface{}) (newVal interface{}, err error)
	}

	// OptionValues is option/value map
	OptionValues = map[Option]interface{}

	options struct {
		sync.RWMutex
		opts             map[Option]interface{}
		accepts          map[Option]bool
		upstream         Options
		downstream       Options
		optionChangeHook OptionChangeHook
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

	// StringOption is option with string value.
	StringOption interface {
		Option
		Value(val interface{}) string
	}

	stringOption struct {
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

	// Uint8Option is option with uint8 value.
	Uint8Option interface {
		Option
		Value(val interface{}) uint8
	}

	uint8Option struct {
		baseOption
	}

	// Uint16Option is option with uint16 value.
	Uint16Option interface {
		Option
		Value(val interface{}) uint16
	}

	uint16Option struct {
		baseOption
	}

	// Uint32Option is option with uint32 value.
	Uint32Option interface {
		Option
		Value(val interface{}) uint32
	}

	uint32Option struct {
		baseOption
	}

	// Int32Option is option with int32 value.
	Int32Option interface {
		Option
		Value(val interface{}) int32
	}

	int32Option struct {
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
	return NewOptionsWithValues(nil)
}

// NewOptionsWithValues create an option set with option values.
func NewOptionsWithValues(ovs OptionValues) Options {
	opts := &options{
		opts: make(map[Option]interface{}),
	}
	for opt, val := range ovs {
		opts.SetOption(opt, val)
	}
	return opts
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

func (opts *options) SetOptionChangeHook(hook OptionChangeHook) Options {
	opts.Lock()
	opts.optionChangeHook = hook
	opts.Unlock()
	return opts
}

// SetOption add an option value.
func (opts *options) SetOption(opt Option, val interface{}) (err error) {
	if val, err = opt.Validate(val); err != nil {
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

	opts.doSetOption(opt, val)
	return
}

// doSetOption used by other setting functions.
func (opts *options) doSetOption(opt Option, val interface{}) {
	oldVal := opts.opts[opt]
	opts.opts[opt] = val
	if opts.optionChangeHook != nil {
		opts.Unlock()
		opts.optionChangeHook(opt, oldVal, val)
		opts.Lock()
	}
}

// WithOption set an option value.
func (opts *options) WithOption(opt Option, val interface{}) Options {
	opts.SetOption(opt, val)
	return opts
}

func (opts *options) SetOptionIfNotExists(opt Option, val interface{}) (err error) {
	if val, err = opt.Validate(val); err != nil {
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
		opts.doSetOption(opt, val)
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

func (opts *options) OptionValues() (res OptionValues) {
	opts.RLock()
	defer opts.RUnlock()

	res = make(map[Option]interface{})
	for opt, val := range opts.opts {
		res[opt] = val
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
func (o *boolOption) Validate(val interface{}) (newVal interface{}, err error) {
	if _, ok := val.(bool); !ok {
		err = ErrInvalidOptionValue
		return
	}
	newVal = val
	return
}

// Value get option's value, must ensure option value is not empty
func (o *boolOption) Value(val interface{}) bool {
	return val.(bool)
}

// NewStringOption create a string option
func NewStringOption(name interface{}) StringOption {
	return &stringOption{baseOption{name}}
}

// Validate validate the option value
func (o *stringOption) Validate(val interface{}) (newVal interface{}, err error) {
	if _, ok := val.(string); !ok {
		err = ErrInvalidOptionValue
		return
	}
	newVal = val
	return
}

// Value get option's value, must ensure option value is not empty
func (o *stringOption) Value(val interface{}) string {
	return val.(string)
}

// NewTimeDurationOption create a time duration option
func NewTimeDurationOption(name interface{}) TimeDurationOption {
	return &timeDurationOption{baseOption{name}}
}

// Validate validate the option value
func (o *timeDurationOption) Validate(val interface{}) (newVal interface{}, err error) {
	if _, ok := val.(time.Duration); !ok {
		err = ErrInvalidOptionValue
		return
	}
	newVal = val
	return
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
func (o *intOption) Validate(val interface{}) (newVal interface{}, err error) {
	if _, ok := val.(int); !ok {
		err = ErrInvalidOptionValue
		return
	}
	newVal = val
	return
}

// Value get option's value, must ensure option value is not empty
func (o *intOption) Value(val interface{}) int {
	return val.(int)
}

// NewUint8Option create an uint8 option
func NewUint8Option(name interface{}) Uint8Option {
	return &uint8Option{baseOption{name}}
}

// Validate validate the option value
func (o *uint8Option) Validate(val interface{}) (newVal interface{}, err error) {
	switch x := val.(type) {
	case uint8:
		newVal = x
	case int:
		if x >= 0 && x <= math.MaxUint8 {
			newVal = uint8(x)
			break
		}
		err = ErrInvalidOptionValue
	default:
		err = ErrInvalidOptionValue
	}
	return
}

// Value get option's value, must ensure option value is not empty
func (o *uint8Option) Value(val interface{}) uint8 {
	return val.(uint8)
}

// NewUint16Option create an uint16 option
func NewUint16Option(name interface{}) Uint16Option {
	return &uint16Option{baseOption{name}}
}

// Validate validate the option value
func (o *uint16Option) Validate(val interface{}) (newVal interface{}, err error) {
	switch x := val.(type) {
	case uint16:
		newVal = x
	case int:
		if x >= 0 && x <= math.MaxUint16 {
			newVal = uint16(x)
			break
		}
		err = ErrInvalidOptionValue
	default:
		err = ErrInvalidOptionValue
	}
	return
}

// Value get option's value, must ensure option value is not empty
func (o *uint16Option) Value(val interface{}) uint16 {
	return val.(uint16)
}

// NewUint32Option create an uint32 option
func NewUint32Option(name interface{}) Uint32Option {
	return &uint32Option{baseOption{name}}
}

// Validate validate the option value
func (o *uint32Option) Validate(val interface{}) (newVal interface{}, err error) {
	switch x := val.(type) {
	case uint32:
		newVal = x
	case int:
		if x >= 0 && x <= math.MaxUint32 {
			newVal = uint32(x)
			break
		}
		err = ErrInvalidOptionValue
	default:
		err = ErrInvalidOptionValue
	}
	return
}

// Value get option's value, must ensure option value is not empty
func (o *uint32Option) Value(val interface{}) uint32 {
	return val.(uint32)
}

// NewInt32Option create an int32 option
func NewInt32Option(name interface{}) Int32Option {
	return &int32Option{baseOption{name}}
}

// Validate validate the option value
func (o *int32Option) Validate(val interface{}) (newVal interface{}, err error) {
	switch x := val.(type) {
	case int32:
		newVal = x
	case int:
		if x >= math.MinInt32 && x <= math.MaxInt32 {
			newVal = int32(x)
			break
		}
		err = ErrInvalidOptionValue
	default:
		err = ErrInvalidOptionValue
	}
	return
}

// Value get option's value, must ensure option value is not empty
func (o *int32Option) Value(val interface{}) int32 {
	return val.(int32)
}
