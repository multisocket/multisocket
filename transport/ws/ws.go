package ws

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
)

type (
	wsTran struct {
		scheme string
		isSr   bool
		isRw   bool
	}

	dialer struct {
		t    *wsTran
		addr string
		url  *url.URL
	}

	// Listener websocket listener, exported for add handler or self serving
	Listener struct {
		t        *wsTran
		addr     string
		URL      *url.URL
		upgrader websocket.Upgrader
		*http.ServeMux
		externalListen bool
		htsvr          *http.Server
		listener       net.Listener
		pending        chan net.Conn
		sync.Mutex
		closedq chan struct{}
	}

	wsConn struct {
		*websocket.Conn
		url   *url.URL
		laddr net.Addr
		raddr net.Addr
		r     io.Reader
		dtype int
	}

	// SendReceiver
	srWsConn struct {
		*wsConn
	}

	// CheckOriginFunc check request origin
	CheckOriginFunc func(r *http.Request) bool
)

var (
	// RwTransport is a transport.Transport for Websocket, using ReadWriter.
	RwTransport = &wsTran{scheme: "ws.rw", isRw: true}
	// SrTransport is a transport.Transport for Websocket, using SendReceiver+ReadWriter.
	SrTransport = &wsTran{scheme: "ws.sr", isSr: true}
)

var (
	subprotocols = []string{"multisocket.binary", "multisocket.text"}
	dataTypes    = map[string]int{
		"multisocket.binary": websocket.BinaryMessage,
		"multisocket.text":   websocket.TextMessage,
	}
)

func init() {
	transport.RegisterTransport(RwTransport)
	transport.RegisterTransport(SrTransport)
	// default transport
	transport.RegisterTransportWithScheme(SrTransport, "ws")
}

func noCheckOrigin(r *http.Request) bool {
	return true
}

// ws
func (c *wsConn) LocalAddr() net.Addr {
	return c.laddr
}

func (c *wsConn) RemoteAddr() net.Addr {
	return c.raddr
}

// SendReceiver

func (c *srWsConn) Send(b []byte) (err error) {
	return c.Conn.WriteMessage(c.dtype, b)
}

func (c *srWsConn) Recv() (b []byte, err error) {
	_, b, err = c.Conn.ReadMessage()
	return
}

// ReadWriter

func (c *wsConn) Read(b []byte) (n int, err error) {
	if c.r == nil {
		if _, c.r, err = c.Conn.NextReader(); err != nil {
			return
		}
	}
	n, err = c.r.Read(b)
	if err == io.EOF {
		c.r = nil
		if n == 0 {
			return c.Read(b)
		}
		err = nil
	}
	return
}

func (c *wsConn) Write(b []byte) (n int, err error) {
	err = c.Conn.WriteMessage(c.dtype, b)
	n = len(b)
	return
}

func (c *wsConn) SetDeadline(t time.Time) (err error) {
	if err = c.Conn.SetReadDeadline(t); err != nil {
		return
	}
	return c.Conn.SetWriteDeadline(t)
}

// dialer

func (d *dialer) Dial(opts options.Options) (_ transport.Connection, err error) {
	var (
		ws *websocket.Conn
	)

	wd := &websocket.Dialer{
		WriteBufferPool: &sync.Pool{},
		Subprotocols:    subprotocols,
	}
	// config
	if val, ok := opts.GetOption(Options.ReadBufferSize); ok {
		wd.ReadBufferSize = Options.ReadBufferSize.Value(val)
	}
	if val, ok := opts.GetOption(Options.WriteBufferSize); ok {
		wd.WriteBufferSize = Options.ReadBufferSize.Value(val)
	}

	if ws, _, err = wd.Dial(d.url.String(), nil); err != nil {
		return nil, err
	}

	dtype, ok := dataTypes[ws.Subprotocol()]
	if !ok {
		ws.Close()
		err = errs.ErrBadProtocol
		return
	}

	c := &wsConn{
		Conn:  ws,
		url:   d.url,
		laddr: ws.LocalAddr(),
		raddr: transport.NewAddress(d.t.scheme, d.addr),
		dtype: dtype,
	}

	var conn net.Conn = c
	if d.t.isSr {
		conn = &srWsConn{wsConn: c}
	}

	return transport.NewConnection(RwTransport, conn, false)
}

// listener

// Listen start listen
func (l *Listener) Listen(opts options.Options) (err error) {
	select {
	case <-l.closedq:
		return errs.ErrClosed
	default:
	}

	l.pending = make(chan net.Conn, Options.Listener.PendingSize.ValueFrom(opts))
	// config
	if val, ok := opts.GetOption(Options.ReadBufferSize); ok {
		l.upgrader.ReadBufferSize = Options.ReadBufferSize.Value(val)
	}
	if val, ok := opts.GetOption(Options.WriteBufferSize); ok {
		l.upgrader.WriteBufferSize = Options.ReadBufferSize.Value(val)
	}
	if val, ok := opts.GetOption(Options.Listener.CheckOrigin); ok {
		checkOrigin := Options.Listener.CheckOrigin.Value(val)
		if checkOrigin {
			l.upgrader.CheckOrigin = nil
			if val, ok := opts.GetOption(Options.Listener.OriginChecker); ok {
				l.upgrader.CheckOrigin = val.(CheckOriginFunc)
			}
		} else {
			l.upgrader.CheckOrigin = noCheckOrigin
		}
	} else if val, ok := opts.GetOption(Options.Listener.OriginChecker); ok {
		l.upgrader.CheckOrigin = val.(CheckOriginFunc)
	}

	if Options.Listener.ExternalListen.ValueFrom(opts) {
		l.externalListen = true
		return nil
	}

	// internal listen
	var taddr *net.TCPAddr
	if taddr, err = transport.ResolveTCPAddr(l.URL.Host); err != nil {
		return err
	}

	if l.listener, err = net.ListenTCP("tcp", taddr); err != nil {
		return
	}
	l.htsvr = &http.Server{Handler: l.ServeMux}
	go l.htsvr.Serve(l.listener)
	return nil
}

// Accept start accept
func (l *Listener) Accept(opts options.Options) (transport.Connection, error) {
	if !l.externalListen && l.listener == nil {
		return nil, errs.ErrBadOperateState
	}

	select {
	case c := <-l.pending:
		return transport.NewConnection(RwTransport, c, true)
	case <-l.closedq:
		return nil, errs.ErrClosed
	}
}

// Close stop listen
func (l *Listener) Close() error {
	l.Lock()
	select {
	case <-l.closedq:
		l.Unlock()
		return errs.ErrClosed
	default:
		close(l.closedq)
	}
	l.Unlock()

	if l.listener != nil {
		l.listener.Close()
	}

CLOSING:
	for {
		select {
		case c := <-l.pending:
			c.Close()
		default:
			break CLOSING
		}
	}
	return nil
}

func (l *Listener) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	ws, err := l.upgrader.Upgrade(resp, req, nil)
	if err != nil {
		return
	}

	select {
	case <-l.closedq:
		ws.Close()
		return
	default:
	}

	dtype, ok := dataTypes[ws.Subprotocol()]
	if !ok {
		ws.Close()
		return
	}

	c := &wsConn{
		Conn:  ws,
		url:   l.URL,
		laddr: transport.NewAddress(l.t.scheme, l.addr),
		raddr: ws.RemoteAddr(),
		dtype: dtype,
	}

	if l.t.isSr {
		l.pending <- &srWsConn{wsConn: c}
	} else {
		l.pending <- c
	}
}

func (t *wsTran) Scheme() string {
	return t.scheme
}

func (t *wsTran) NewDialer(address string) (transport.Dialer, error) {
	var (
		err  error
		url  *url.URL
		addr string
	)
	if url, addr, err = parseAddressToURL(t, address); err != nil {
		return nil, err
	}
	// NOTE:
	if t.isSr {
		url.Scheme = strings.TrimSuffix(url.Scheme, ".sr")
	} else if t.isRw {
		url.Scheme = strings.TrimSuffix(url.Scheme, ".rw")
	}

	d := &dialer{
		t:    t,
		addr: addr,
		url:  url,
	}
	return d, nil
}

func (t *wsTran) NewListener(address string) (transport.Listener, error) {
	var (
		err  error
		url  *url.URL
		addr string
	)
	if url, addr, err = parseAddressToURL(t, address); err != nil {
		return nil, err
	}
	if url.Path == "" {
		url.Path = "/"
	}
	// NOTE:
	if t.isSr {
		url.Scheme = strings.TrimSuffix(url.Scheme, ".sr")
	} else if t.isRw {
		url.Scheme = strings.TrimSuffix(url.Scheme, ".rw")
	}

	l := &Listener{
		t:    t,
		addr: addr,
		URL:  url,
		upgrader: websocket.Upgrader{
			WriteBufferPool: &sync.Pool{},
			Subprotocols:    subprotocols,
		},
		closedq: make(chan struct{}),
	}
	l.ServeMux = http.NewServeMux()
	l.ServeMux.Handle(l.URL.Path, l)

	return l, nil
}

func parseAddressToURL(t transport.Transport, address string) (url *url.URL, addr string, err error) {
	if addr, err = transport.StripScheme(t, address); err != nil {
		return
	}
	url, err = url.Parse(address)
	return
}
