package remotewrite

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type roundrobin struct {
	idx     uint64
	servers []string
}

func (rr *roundrobin) Next() string {
	idx := atomic.AddUint64(&rr.idx, 1) % uint64(len(rr.servers))
	return rr.servers[idx]
}

func newRoundRobin(servers []string) (*roundrobin, error) {
	if len(servers) == 0 {
		return nil, fmt.Errorf("servers is empty")
	}
	return &roundrobin{
		servers: servers,
		idx:     rand.Uint64() % uint64(len(servers)),
	}, nil
}

type transport struct {
	rr         atomic.Value
	host       string
	port       string
	rawAddr    string
	retryTimes int
	tr         http.RoundTripper
	stopCh     chan struct{}
}

func newTransport(tr http.RoundTripper, addr string, stopCh chan struct{}) (*transport, error) {
	var (
		host string
		port string
		err  error
	)
	host = addr
	if strings.Index(addr, ":") >= 0 {
		host, port, err = net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
	}

	t := &transport{
		host:       host,
		port:       port,
		rawAddr:    addr,
		tr:         tr,
		retryTimes: 3,
		stopCh:     stopCh,
	}
	if err = t.resolveHost(); err != nil {
		return nil, err
	}

	go t.loop()

	return t, nil
}

func (tr *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	if tr.rawAddr != r.URL.Host {
		return tr.tr.RoundTrip(r)
	}

	rr := tr.rr.Load().(*roundrobin)
	for i := 0; i < tr.retryTimes; i++ {
		addr := rr.Next()
		r.URL.Host = addr
		resp, err := tr.tr.RoundTrip(r)
		if err == nil {
			return resp, nil
		}
		if opErr, ok := err.(*net.OpError); ok && opErr.Op == "dial" {
			continue
		}
		return resp, err
	}
	return nil, fmt.Errorf("not found avaliable backend")
}

func (tr *transport) resolveHost() error {
	servers, err := net.LookupHost(tr.host)
	if err != nil {
		return err
	}

	if len(tr.port) > 0 {
		for i := range servers {
			servers[i] = net.JoinHostPort(servers[i], tr.port)
		}
	}

	rr, err := newRoundRobin(servers)
	if err != nil {
		return err
	}

	tr.rr.Store(rr)
	return nil
}

func (tr *transport) loop() {
	dur := 30 * time.Second
	tk := time.NewTimer(dur)
	defer tk.Stop()

	for {
		select {
		case <-tk.C:
			if err := tr.resolveHost(); err != nil {
				logger.Errorf("resolve host failed, err: %v", err)
			}
			tk.Reset(dur)
		case <-tr.stopCh:
			return
		}
	}
}
