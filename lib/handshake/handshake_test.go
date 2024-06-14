package handshake

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestVMInsertHandshake(t *testing.T) {
	testHandshake(t, VMInsertClient, VMInsertServer)
}

func TestVMSelectHandshake(t *testing.T) {
	testHandshake(t, VMSelectClient, VMSelectServer)
}

func testHandshake(t *testing.T, clientFunc ClientFunc, serverFunc ServerFunc) {
	t.Helper()

	c, s := net.Pipe()
	ch := make(chan error, 1)
	go func() {
		bcs, err := serverFunc(s, 3, 1)
		if err != nil {
			ch <- fmt.Errorf("error on outer handshake: %w", err)
			return
		}
		bcc, id, err := clientFunc(bcs, 3)
		if err != nil {
			ch <- fmt.Errorf("error on inner handshake: %w", err)
			return
		}
		if id != 1 {
			ch <- fmt.Errorf("unexpected id; got %d; want 1", id)
			return
		}
		if bcc == nil {
			ch <- fmt.Errorf("expecting non-nil conn")
			return
		}
		ch <- nil
	}()

	bcc, id, err := clientFunc(c, 0)
	if err != nil {
		t.Fatalf("error on outer handshake: %s", err)
	}
	if id != 1 {
		t.Fatalf("unexpected id; got %d; want 2", id)
	}

	bcs, err := serverFunc(bcc, 0, 1)
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
