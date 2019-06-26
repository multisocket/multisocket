package options

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

type (
	// OptionChangeHook is called when set an option value.
	OptionChangeHook func(opt Option, oldVal, newVal interface{})
	// Options is option set.
	Options interface {
		SetOption(opt Option, val interface{}) (err error)
		SetOptionIfNotExists(opt Option, val interface{}) (err error)
		GetOption(opt Option) (val interface{}, ok bool)
		GetOptionDefault(opt Option) interface{}
		OptionValues() OptionValues
		SetOptionChangeHook(hook OptionChangeHook) Options
	}

	// Option is an option item.
	Option interface {
		String() string
		DefaultValue() interface{}
		Validate(val interface{}) (newVal interface{}, err error)
		Parse(s string) (val interface{}, err error)
	}

	// OptionValues is option/value map
	OptionValues = map[Option]interface{}

	options struct {
		sync.RWMutex
		opts             map[Option]interface{}
		accepts          map[Option]bool
		downstream       Options
		optionChangeHook OptionChangeHook
	}

	baseOption struct {
		defaultValue interface{}
	}

	// BoolOption is option with bool value.
	BoolOption interface {
		Option
		Value(val interface{}) bool
		ValueFrom(optss ...Options) bool
	}

	boolOption struct {
		baseOption
	}

	// StringOption is option with string value.
	StringOption interface {
		Option
		Value(val interface{}) string
		ValueFrom(optss ...Options) string
	}

	stringOption struct {
		baseOption
	}

	// TimeDurationOption is option with time duration value.
	TimeDurationOption interface {
		Option
		Value(val interface{}) time.Duration
		ValueFrom(optss ...Options) time.Duration
	}

	timeDurationOption struct {
		baseOption
	}

	// IntOption is option with int value.
	IntOption interface {
		Option
		Value(val interface{}) int
		ValueFrom(optss ...Options) int
	}

	intOption struct {
		baseOption
	}

	// Uint8Option is option with uint8 value.
	Uint8Option interface {
		Option
		Value(val interface{}) uint8
		ValueFrom(optss ...Options) uint8
	}

	uint8Option struct {
		baseOption
	}

	// Uint16Option is option with uint16 value.
	Uint16Option interface {
		Option
		Value(val interface{}) uint16
		ValueFrom(optss ...Options) uint16
	}

	uint16Option struct {
		baseOption
	}

	// Uint32Option is option with uint32 value.
	Uint32Option interface {
		Option
		Value(val interface{}) uint32
		ValueFrom(optss ...Options) uint32
	}

	uint32Option struct {
		baseOption
	}

	// Int32Option is option with int32 value.
	Int32Option interface {
		Option
		Value(val interface{}) int32
		ValueFrom(optss ...Options) int32
	}

	int32Option struct {
		baseOption
	}
)

// errors
var (
	ErrInvalidOptionValue = errors.New("invalid option value")
	ErrUnsupportedOption  = errors.New("unsupported option")
	ErrOptionNotFound     = errors.New("option not found")
)

var (
	lock              sync.RWMutex
	registeredOptions = map[string]interface{}{}
	optionFullNames   = map[Option]string{}
)

// RegisterOption register option
func RegisterOption(opt Option, name string, domains []string) {
	lock.Lock()
	cur := registeredOptions
	for _, d := range domains {
		d = strings.ToLower(d)
		m := cur[d]
		if m == nil {
			m = make(map[string]interface{})
			cur[d] = m
		}
		cur = m.(map[string]interface{})
	}
	cur[strings.ToLower(name)] = opt
	optionFullNames[opt] = strings.Join(append(domains, name), ".")
	lock.Unlock()
}

// RegisterStructuredOptions register structured options
func RegisterStructuredOptions(opts interface{}, domains []string) {
	v := reflect.ValueOf(opts)
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		fv := v.Field(i).Interface()
		if opt, ok := fv.(Option); ok {
			// option
			RegisterOption(opt, f.Name, domains)
		} else {
			// structured options
			RegisterStructuredOptions(fv, append(domains, f.Name))
		}
	}
}

// ParseOption parse Option from string.
func ParseOption(s string) (opt Option, err error) {
	domains := strings.Split(s, ".")
	l := len(domains)
	name := strings.ToLower(domains[l-1])
	domains = domains[:l-1]

	lock.Lock()
	var ok bool
	cur := registeredOptions
	for _, d := range domains {
		d = strings.ToLower(d)
		m := cur[d]
		if m == nil {
			return nil, fmt.Errorf("%s: %s", ErrOptionNotFound, s)
		}
		if cur, ok = m.(map[string]interface{}); !ok {
			return nil, fmt.Errorf("%s: %s", ErrOptionNotFound, s)
		}
	}

	if opt, ok = cur[name].(Option); !ok {
		return nil, fmt.Errorf("%s: %s", ErrOptionNotFound, s)
	}
	lock.Unlock()
	return
}

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

// NewOptionsWithDownStreamsAndAccepts create an option set with down stream and accepts.
func NewOptionsWithDownStreamsAndAccepts(downstream Options, accepts ...Option) Options {
	options := &options{
		opts:       make(map[Option]interface{}),
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
	return NewOptionsWithDownStreamsAndAccepts(nil, accepts...)
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
		val, ok = opts.opts[opt]
		opts.RUnlock()
		return
	} else if opts.downstream != nil {
		// pass to downstream
		return opts.downstream.GetOption(opt)
	}

	return
}

// GetOptionDefault get an option value or option default value.
func (opts *options) GetOptionDefault(opt Option) interface{} {
	if val, ok := opts.GetOption(opt); ok {
		return val
	}
	return opt.DefaultValue()
}

func (opts *options) OptionValues() (res OptionValues) {
	opts.RLock()

	res = make(map[Option]interface{})
	for opt, val := range opts.opts {
		res[opt] = val
	}
	opts.RUnlock()

	return
}

func (o *baseOption) String() string {
	return fmt.Sprintf("<%T:%v>", o.defaultValue, o.defaultValue)
}

func (o *baseOption) DefaultValue() interface{} {
	return o.defaultValue
}

// valueFrom get opt from optss else return default value
func valueFrom(opt Option, optss ...Options) interface{} {
	for _, opts := range optss {
		if val, ok := opts.GetOption(opt); ok {
			return val
		}
	}
	return opt.DefaultValue()
}

// NewBoolOption create a bool option
func NewBoolOption(val bool) BoolOption {
	return &boolOption{baseOption{val}}
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

func (o *boolOption) Parse(s string) (val interface{}, err error) {
	s = strings.ToLower(s)
	switch s {
	case "", "true", "t", "1":
		return true, nil
	case "false", "f", "0":
		return false, nil
	default:
		return nil, fmt.Errorf("%s: %s=>%s", ErrInvalidOptionValue, optionFullNames[o], s)
	}
}

// Value get option's value, must ensure option value is not empty
func (o *boolOption) Value(val interface{}) bool {
	return val.(bool)
}

func (o *boolOption) ValueFrom(optss ...Options) bool {
	return valueFrom(o, optss...).(bool)
}

// NewStringOption create a string option
func NewStringOption(val string) StringOption {
	return &stringOption{baseOption{val}}
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

func (o *stringOption) Parse(s string) (val interface{}, err error) {
	return s, nil
}

// Value get option's value, must ensure option value is not empty
func (o *stringOption) Value(val interface{}) string {
	return val.(string)
}

func (o *stringOption) ValueFrom(optss ...Options) string {
	return valueFrom(o, optss...).(string)
}

// NewTimeDurationOption create a time duration option
func NewTimeDurationOption(name time.Duration) TimeDurationOption {
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

func (o *timeDurationOption) Parse(s string) (val interface{}, err error) {
	if val, err = time.ParseDuration(s); err != nil {
		err = fmt.Errorf("%s: %s=>%s", ErrInvalidOptionValue, optionFullNames[o], s)
	}
	return
}

// Value get option's value, must ensure option value is not empty
func (o *timeDurationOption) Value(val interface{}) time.Duration {
	return val.(time.Duration)
}

func (o *timeDurationOption) ValueFrom(optss ...Options) time.Duration {
	return valueFrom(o, optss...).(time.Duration)
}

// NewIntOption create an int option
func NewIntOption(val int) IntOption {
	return &intOption{baseOption{val}}
}

// Validate validate the option value
func (o *intOption) Validate(val interface{}) (newVal interface{}, err error) {
	switch x := val.(type) {
	case int:
		newVal = x
	case int64:
		newVal = int(x)
	default:
		err = ErrInvalidOptionValue
	}
	return
}

func (o *intOption) Parse(s string) (val interface{}, err error) {
	if val, err = strconv.ParseInt(s, 10, 0); err != nil {
		err = fmt.Errorf("%s: %s=>%s", ErrInvalidOptionValue, optionFullNames[o], s)
	}
	return
}

// Value get option's value, must ensure option value is not empty
func (o *intOption) Value(val interface{}) int {
	return val.(int)
}

func (o *intOption) ValueFrom(optss ...Options) int {
	return valueFrom(o, optss...).(int)
}

// NewUint8Option create an uint8 option
func NewUint8Option(val uint8) Uint8Option {
	return &uint8Option{baseOption{val}}
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
	case uint64:
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

func (o *uint8Option) Parse(s string) (val interface{}, err error) {
	if val, err = strconv.ParseUint(s, 10, 8); err != nil {
		err = fmt.Errorf("%s: %s=>%s", ErrInvalidOptionValue, optionFullNames[o], s)
	}
	return
}

// Value get option's value, must ensure option value is not empty
func (o *uint8Option) Value(val interface{}) uint8 {
	return val.(uint8)
}

func (o *uint8Option) ValueFrom(optss ...Options) uint8 {
	return valueFrom(o, optss...).(uint8)
}

// NewUint16Option create an uint16 option
func NewUint16Option(val uint16) Uint16Option {
	return &uint16Option{baseOption{val}}
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
	case uint64:
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

func (o *uint16Option) Parse(s string) (val interface{}, err error) {
	if val, err = strconv.ParseUint(s, 10, 16); err != nil {
		err = fmt.Errorf("%s: %s=>%s", ErrInvalidOptionValue, optionFullNames[o], s)
	}
	return
}

// Value get option's value, must ensure option value is not empty
func (o *uint16Option) Value(val interface{}) uint16 {
	return val.(uint16)
}

func (o *uint16Option) ValueFrom(optss ...Options) uint16 {
	return valueFrom(o, optss...).(uint16)
}

// NewUint32Option create an uint32 option
func NewUint32Option(val uint32) Uint32Option {
	return &uint32Option{baseOption{val}}
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
	case uint64:
		if x >= 0 && x <= math.MaxUint64 {
			newVal = uint32(x)
			break
		}
		err = ErrInvalidOptionValue
	default:
		err = ErrInvalidOptionValue
	}
	return
}

func (o *uint32Option) Parse(s string) (val interface{}, err error) {
	if val, err = strconv.ParseUint(s, 10, 32); err != nil {
		err = fmt.Errorf("%s: %s=>%s", ErrInvalidOptionValue, optionFullNames[o], s)
	}
	return
}

// Value get option's value, must ensure option value is not empty
func (o *uint32Option) Value(val interface{}) uint32 {
	return val.(uint32)
}

func (o *uint32Option) ValueFrom(optss ...Options) uint32 {
	return valueFrom(o, optss...).(uint32)
}

// NewInt32Option create an int32 option
func NewInt32Option(val int32) Int32Option {
	return &int32Option{baseOption{val}}
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
	case int64:
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

func (o *int32Option) Parse(s string) (val interface{}, err error) {
	if val, err = strconv.ParseInt(s, 10, 32); err != nil {
		err = fmt.Errorf("%s: %s=>%s", ErrInvalidOptionValue, optionFullNames[o], s)
	}
	return
}

// Value get option's value, must ensure option value is not empty
func (o *int32Option) Value(val interface{}) int32 {
	return val.(int32)
}

func (o *int32Option) ValueFrom(optss ...Options) int32 {
	return valueFrom(o, optss...).(int32)
}
