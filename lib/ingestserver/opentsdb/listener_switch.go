package opentsdb

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// listenerSwitch listens for incoming connections and multiplexes them to OpenTSDB http or telnet listeners
// depending on the first byte in the accepted connection.
//
// It is expected that both listeners - http and telnet consume incoming connections as soon as possible.
type listenerSwitch struct {
	ln net.Listener
	wg sync.WaitGroup

	telnetConnsCh chan net.Conn
	httpConnsCh   chan net.Conn

	closeLock sync.Mutex
	closed    bool
	acceptErr error
	closeErr  error
}

func newListenerSwitch(ln net.Listener) *listenerSwitch {
	ls := &listenerSwitch{
		ln: ln,
	}
	ls.telnetConnsCh = make(chan net.Conn)
	ls.httpConnsCh = make(chan net.Conn)
	ls.wg.Add(1)
	go func() {
		ls.worker()
		close(ls.telnetConnsCh)
		close(ls.httpConnsCh)
		ls.wg.Done()
	}()
	return ls
}

func (ls *listenerSwitch) stop() error {
	var err error
	ls.closeLock.Lock()
	if !ls.closed {
		err = ls.ln.Close()
		ls.closeErr = err
		ls.closed = true
	}
	ls.closeLock.Unlock()

	if err == nil {
		// Wait until worker detects the closed ls.ln and exits.
		ls.wg.Wait()
	}
	return err
}

func (ls *listenerSwitch) worker() {
	var buf [1]byte
	for {
		c, err := ls.ln.Accept()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Temporary() {
				logger.Infof("listenerSwitch: temporary error at %q: %s; sleeping for a second...", ls.ln.Addr(), err)
				time.Sleep(time.Second)
				continue
			}
			ls.closeLock.Lock()
			ls.acceptErr = err
			ls.closeLock.Unlock()
			return
		}
		if _, err := io.ReadFull(c, buf[:]); err != nil {
			logger.Errorf("listenerSwitch: cannot read one byte from the underlying connection for %q: %s", ls.ln.Addr(), err)
			_ = c.Close()
			continue
		}

		// It is expected that both listeners - http and telnet consume incoming connections as soon as possible,
		// so the below code shouldn't block for extended periods of time.
		pc := &peekedConn{
			Conn:      c,
			firstChar: buf[0],
		}
		if buf[0] == 'p' {
			// Assume the request starts with `put`.
			ls.telnetConnsCh <- pc
		} else {
			// Assume the request starts with `POST`.
			ls.httpConnsCh <- pc
		}
	}
}

type peekedConn struct {
	net.Conn
	firstChar     byte
	firstCharRead bool
}

func (pc *peekedConn) Read(p []byte) (int, error) {
	// It is assumed that the pc cannot be read from concurrent goroutines.
	if pc.firstCharRead {
		// Fast path - first char already read.
		return pc.Conn.Read(p)
	}

	// Slow path - read the first char.
	if len(p) == 0 {
		return 0, nil
	}
	p[0] = pc.firstChar
	pc.firstCharRead = true
	n, err := pc.Conn.Read(p[1:])
	return n + 1, err
}

func (ls *listenerSwitch) newTelnetListener() *chanListener {
	return &chanListener{
		ls: ls,
		ch: ls.telnetConnsCh,
	}
}

func (ls *listenerSwitch) newHTTPListener() *chanListener {
	return &chanListener{
		ls: ls,
		ch: ls.httpConnsCh,
	}
}

type chanListener struct {
	ls *listenerSwitch
	ch chan net.Conn
}

func (cl *chanListener) Accept() (net.Conn, error) {
	c, ok := <-cl.ch
	if ok {
		return c, nil
	}

	cl.ls.closeLock.Lock()
	err := cl.ls.acceptErr
	cl.ls.closeLock.Unlock()
	return nil, err
}

func (cl *chanListener) Close() error {
	return cl.ls.stop()
}

func (cl *chanListener) Addr() net.Addr {
	return cl.ls.ln.Addr()
}
