package tracer_test

import (
	"fmt"
	"net/http/httptest"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tracer"
)

func ExampleContext() {
	r := httptest.NewRequest("GET", "/", nil)
	q := r.URL.Query()
	q.Add("trace_query", "true")
	q.Add("trace_query_id", "foo")
	r.URL.RawQuery = q.Encode()

	ctx := tracer.NewContext(r)

	factorial(ctx, 5)

	ctx.Done(func() string {
		return fmt.Sprintf("execution trace %q", ctx.ID())
	})
	fmt.Println(ctx.Print())
	// Output:
	//0ms: execution trace "foo"
	//-- 0ms: calculating factorial for 5
	//---- 0ms: calculating factorial for 4
	//------ 0ms: calculating factorial for 3
	//-------- 0ms: calculating factorial for 2
	//---------- 0ms: calculating factorial for 1
	//------------ 0ms: calculating factorial for 0
}

func factorial(ctx *tracer.Context, number int) uint64 {
	// create a sub-context for each call
	subCtx := ctx.Add()

	var res uint64
	if number >= 1 {
		res = uint64(number) * factorial(subCtx, number-1)
	} else {
		res = 1
	}

	// finish sub-context
	subCtx.Done(func() string {
		return fmt.Sprintf("calculating factorial for %d", number)
	})

	return res
}
