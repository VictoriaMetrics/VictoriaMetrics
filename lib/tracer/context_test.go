package tracer

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext(nil)
	if ctx.enabled {
		t.Fatalf("expected context to be disabled for nil request")
	}

	ctx = NewContext(newReq(t))
	if ctx.enabled {
		t.Fatalf("expected context to be disabled for empty request")
	}

	ctx = NewContext(newReq(t, traceEnableColorParam, "true"))
	if ctx.enabled {
		t.Fatalf("expected context to be disabled for request with only color set")
	}

	ctx = NewContext(newReq(t, traceEnableParam, "false"))
	if ctx.enabled {
		t.Fatalf("expected context to be disabled for request with enable=false")
	}

	ctx = NewContext(newReq(t, traceEnableParam, "true"))
	if !ctx.enabled {
		t.Fatalf("expected context to be enabled ")
	}
	if ctx.ID() == "" {
		t.Fatalf("expected context to have ID; got empty value instead")
	}
	if ctx.color {
		t.Fatalf("expected context to have color disabled by default")
	}

	ctx = NewContext(newReq(t, traceEnableParam, "true", traceEnableColorParam, "true"))
	if !ctx.enabled {
		t.Fatalf("expected context to be enabled ")
	}
	if !ctx.color {
		t.Fatalf("expected context to have color enabled")
	}

	testID := "foo"
	ctx = NewContext(newReq(t, traceEnableParam, "true", traceIDParam, testID))
	if !ctx.enabled {
		t.Fatalf("expected context to be enabled ")
	}
	if ctx.ID() != testID {
		t.Fatalf("expected context to have ID %q; got %q instead", testID, ctx.ID())
	}
}

func newReq(t *testing.T, params ...string) *http.Request {
	t.Helper()
	if len(params)%2 != 0 {
		t.Fatalf("expected to get even number of params")
	}

	r := httptest.NewRequest("GET", "/", nil)
	q := r.URL.Query()
	for i := 0; i < len(params); i += 2 {
		q.Add(params[i], params[i+1])
	}
	r.URL.RawQuery = q.Encode()
	return r
}
