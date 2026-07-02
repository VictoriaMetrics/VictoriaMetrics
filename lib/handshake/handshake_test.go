package handshake

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestVMInsertHandshake(t *testing.T) {
	testHandshake(t, vminsertClient, VMInsertServer)
}

func TestVMSelectHandshake(t *testing.T) {
	testHandshake(t, VMSelectClient, VMSelectServer)
}

func TestVMSelectServerTCPHealthcheck(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot start listener: %s", err)
	}

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("cannot dial: %s", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("cannot close client conn: %s", err)
	}
	s, err := ln.Accept()
	if err != nil {
		t.Fatalf("cannot accept conn: %s", err)
	}
	if _, err := VMSelectServer(s, 0); !IsTCPHealthcheck(err) {
		t.Fatalf("unexpected error; got %v; want TCP healthcheck error", err)
	}
}

func testHandshake(t *testing.T, clientFunc, serverFunc Func) {
	t.Helper()

	c, s := net.Pipe()
	ch := make(chan error, 1)
	go func() {
		bcs, err := serverFunc(s, 3)
		if err != nil {
			ch <- fmt.Errorf("error on outer handshake: %w", err)
			return
		}
		bcc, err := clientFunc(bcs, 3)
		if err != nil {
			ch <- fmt.Errorf("error on inner handshake: %w", err)
			return
		}
		if bcc == nil {
			ch <- fmt.Errorf("expecting non-nil conn")
			return
		}
		ch <- nil
	}()

	bcc, err := clientFunc(c, 0)
	if err != nil {
		t.Fatalf("error on outer handshake: %s", err)
	}
	bcs, err := serverFunc(bcc, 0)
	if err != nil {
		t.Fatalf("error on inner handshake: %s", err)
	}
	if bcs == nil {
		t.Fatalf("expecting non-nil conn")
	}

	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	case err := <-ch:
		if err != nil {
			t.Fatalf("unexpected error on the server side: %s", err)
		}
	}
}
