package remotewrite

import (
	"context"
	"testing"
)

func TestInit(t *testing.T) {
	oldAddr := *addr
	defer func() { *addr = oldAddr }()

	*addr = "http://localhost:8428"
	cl, err := Init(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Close(); err != nil {
		t.Fatal(err)
	}
}
