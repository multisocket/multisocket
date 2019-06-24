// +build windows

package ipc

import (
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
)

type (
	listenerOptions struct {
		SecurityDescriptor options.StringOption
		InputBufferSize options.Int32Option
		OutputBufferSize options.Int32Option
	}

	ipcOptions struct {
		Listener listenerOptions
	}
)

var (
	// OptionDomains is option's domain
	OptionDomains = append(transport.OptionDomains, "ipc")
	// Options for windows named pipe
	Options = ipcOptions {
		Listener: listenerOptions {
			// OptionSecurityDescriptor represents a Windows security
			// descriptor in SDDL format (string).  This can only be set on
			// a Listener, and must be set before the Listen routine
			// is called.
			SecurityDescriptor: options.NewStringOption(""),
			// OptionInputBufferSize represents the Windows Named Pipe
			// input buffer size in bytes (type int32).  Default is 4096.
			// This is only for Listeners, and must be set before the
			// Listener is started.
			InputBufferSize: options.NewInt32Option(4096),
			// OptionOutputBufferSize represents the Windows Named Pipe
			// output buffer size in bytes (type int32).  Default is 4096.
			// This is only for Listeners, and must be set before the
			// Listener is started.
			OutputBufferSize: options.NewInt32Option(4096),
		}
	}
)

func init() {
	options.RegisterStructuredOptions(Options, OptionDomains)
}