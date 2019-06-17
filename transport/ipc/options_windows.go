// +build windows

package ipc

type optionName int

const (
	listenerOptionNameSecurityDescriptor optionName = iota
	listenerOptionNameInputBufferSize
	listenerOptionNameOutputBufferSize
)

// Options
var (
	// OptionSecurityDescriptor represents a Windows security
	// descriptor in SDDL format (string).  This can only be set on
	// a Listener, and must be set before the Listen routine
	// is called.
	ListenerOptionSecurityDescriptor = options.NewStringOption(listenerOptionNameSecurityDescriptor)

	// OptionInputBufferSize represents the Windows Named Pipe
	// input buffer size in bytes (type int32).  Default is 4096.
	// This is only for Listeners, and must be set before the
	// Listener is started.
	ListenerOptionInputBufferSize = options.NewInt32Option(listenerOptionNameInputBufferSize)

	// OptionOutputBufferSize represents the Windows Named Pipe
	// output buffer size in bytes (type int32).  Default is 4096.
	// This is only for Listeners, and must be set before the
	// Listener is started.
	ListenerOptionOutputBufferSize = options.NewInt32Option(listenerOptionNameOutputBufferSize)
)
