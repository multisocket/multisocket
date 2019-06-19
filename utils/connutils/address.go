package connutils

import (
	"strings"

	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
)

type (
	// SmartAddress group dial/listen, async, raw and address together
	SmartAddress interface {
		ForDial() bool
		ForListen() bool
		IsAsync() bool
		NoReconn() bool
		IsRaw() bool
		Address() string
		Connect(ctr connector.ConnectorCoreAction, ovs options.OptionValues) error
	}

	smartAddress struct {
		dial     bool
		async    bool
		noreconn bool
		listen   bool
		raw      bool
		addr     string
	}
)

// ParseSmartAddress parse s to a SmartAddress
func ParseSmartAddress(s string) SmartAddress {
	parts := strings.Split(s, ",")
	n := len(parts)
	addr := parts[n-1]
	sa := &smartAddress{
		addr: addr,
	}

	for _, p := range parts[:n-1] {
		switch p {
		case "dial", "d":
			sa.dial = true
		case "async", "a":
			sa.async = true
		case "noreconn":
			sa.noreconn = true
		case "listen", "l":
			sa.listen = true
		case "raw", "r":
			sa.raw = true
		}
	}

	return sa
}

func (sa *smartAddress) ForDial() bool {
	return sa.dial
}

func (sa *smartAddress) ForListen() bool {
	return sa.listen
}

func (sa *smartAddress) IsAsync() bool {
	return sa.async
}

func (sa *smartAddress) NoReconn() bool {
	return sa.noreconn
}

func (sa *smartAddress) IsRaw() bool {
	return sa.raw
}

func (sa *smartAddress) Address() string {
	return sa.addr
}

func (sa *smartAddress) Connect(ctr connector.ConnectorCoreAction, ovs options.OptionValues) error {
	xovs := options.OptionValues{}
	if sa.async {
		xovs[connector.DialerOptionDialAsync] = true
	}
	if sa.noreconn {
		xovs[connector.DialerOptionReconnect] = false
	}
	if sa.raw {
		xovs[transport.OptionConnRawMode] = true
	}

	for opt, val := range ovs {
		xovs[opt] = val
	}

	if sa.listen {
		if listener, err := ctr.NewListener(sa.addr, xovs); err != nil {
			return err
		} else if err := listener.Listen(); err != nil {
			return err
		}
	}
	if sa.dial {
		if dialer, err := ctr.NewDialer(sa.addr, xovs); err != nil {
			return err
		} else if err := dialer.Dial(); err != nil {
			return err
		}
	}
	return nil
}
