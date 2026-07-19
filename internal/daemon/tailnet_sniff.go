package daemon

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

// sniffPeekTimeout bounds the first-byte peek that classifies a fresh conn as
// TLS or plaintext. A real client speaks immediately; a conn with nothing to
// say by the deadline is closed.
const sniffPeekTimeout = 3 * time.Second

// tlsRecordHandshake is the first byte of every TLS ClientHello record.
const tlsRecordHandshake = 0x16

// sniffFactories wraps each tailnet leg factory so every accepted conn is
// classified TLS-or-plain; boot legs and reconcile-added legs both inherit it.
func sniffFactories(factories []func(context.Context) (net.Listener, error), mgr *certManager) []func(context.Context) (net.Listener, error) {
	wrapped := make([]func(context.Context) (net.Listener, error), len(factories))
	for i, factory := range factories {
		wrapped[i] = func(ctx context.Context) (net.Listener, error) {
			ln, err := factory(ctx)
			if err != nil {
				return nil, err
			}
			slog.Info("tailnet: leg bound", "addr", ln.Addr().String(), "tls_armed", mgr.get() != nil)
			return newSniffListener(ln, mgr.tlsConfig()), nil
		}
	}
	return wrapped
}

// sniffListener serves TLS and plaintext HTTP on one port. The inner accept
// loop hands each raw conn to its own goroutine for the first-byte peek, so a
// silent client never stalls Accept for anyone else; a conn whose peek times
// out or errors is closed, never handed through. Addr delegates to the inner
// listener via embedding.
type sniffListener struct {
	net.Listener
	conf  *tls.Config
	conns chan net.Conn
	errs  chan error
	done  chan struct{}
	once  sync.Once
}

func newSniffListener(ln net.Listener, conf *tls.Config) *sniffListener {
	l := &sniffListener{
		Listener: ln,
		conf:     conf,
		conns:    make(chan net.Conn),
		errs:     make(chan error, 1),
		done:     make(chan struct{}),
	}
	go l.acceptLoop()
	return l
}

func (l *sniffListener) acceptLoop() {
	// Temporary errors retry with backoff like net/http.Server.Serve; returning
	// here would strand Serve's own retried Accept on a producerless channel.
	var delay time.Duration
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Temporary() { //nolint:staticcheck // SA1019: same Temporary() retry net/http.Server.Serve uses.
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay = min(delay*2, time.Second)
				}
				select {
				case <-time.After(delay):
					continue
				case <-l.done:
					return
				}
			}
			select {
			case l.errs <- err:
			case <-l.done:
			}
			return
		}
		delay = 0
		go l.classify(conn)
	}
}

func (l *sniffListener) classify(conn net.Conn) {
	c, err := sniff(conn, l.conf)
	if err != nil {
		_ = conn.Close()
		return
	}
	select {
	case l.conns <- c:
	case <-l.done:
		_ = c.Close()
	}
}

func (l *sniffListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case err := <-l.errs:
		return nil, err
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *sniffListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return l.Listener.Close()
}

// sniff classifies conn by its first byte under sniffPeekTimeout. A peek that
// times out or errors is reported for the caller to close the conn — a real
// HTTP client sends bytes immediately, so a silent conn fails fast instead of
// occupying the server.
func sniff(conn net.Conn, conf *tls.Config) (net.Conn, error) {
	_ = conn.SetReadDeadline(time.Now().Add(sniffPeekTimeout))
	var first [1]byte
	if _, err := io.ReadFull(conn, first[:]); err != nil {
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Time{})
	buffered := &bufferedConn{Conn: conn, buf: first[:]}
	if first[0] == tlsRecordHandshake {
		return tls.Server(buffered, conf), nil
	}
	return buffered, nil
}

// bufferedConn replays the peeked bytes before reading from the wrapped conn.
type bufferedConn struct {
	net.Conn
	buf []byte
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	if len(c.buf) > 0 {
		n := copy(p, c.buf)
		c.buf = c.buf[n:]
		return n, nil
	}
	return c.Conn.Read(p)
}
