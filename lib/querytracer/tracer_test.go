package querytracer

import (
	"fmt"
	"regexp"
	"sync"
	"testing"
)

func TestTracerDisabled(t *testing.T) {
	qt := New(false, "test")
	if qt.Enabled() {
		t.Fatalf("query tracer must be disabled")
	}
	qtChild := qt.NewChild("child done %d", 456)
	if qtChild.Enabled() {
		t.Fatalf("query tracer must be disabled")
	}
	qtChild.Printf("foo %d", 123)
	qtChild.Done()
	qt.Printf("parent %d", 789)
	if err := qt.AddJSON([]byte("foobar")); err != nil {
		t.Fatalf("unexpected error in AddJSON: %s", err)
	}
	qt.Done()
	s := qt.String()
	if s != "" {
		t.Fatalf("unexpected trace; got %s; want empty", s)
	}
	s = qt.ToJSON()
	if s != "" {
		t.Fatalf("unexpected json trace; got %s; want empty", s)
	}
}

func TestTracerEnabled(t *testing.T) {
	qt := New(true, "test")
	if !qt.Enabled() {
		t.Fatalf("query tracer must be enabled")
	}
	qtChild := qt.NewChild("child done %d", 456)
	if !qtChild.Enabled() {
		t.Fatalf("child query tracer must be enabled")
	}
	qtChild.Printf("foo %d", 123)
	qtChild.Done()
	qt.Printf("parent %d", 789)
	qt.Donef("foo %d", 33)
	s := qt.String()
	sExpected := `- 0ms: : test: foo 33
| - 0ms: child done 456
| | - 0ms: foo 123
| - 0ms: parent 789
`
	if !areEqualTracesSkipDuration(s, sExpected) {
		t.Fatalf("unexpected trace\ngot\n%s\nwant\n%s", s, sExpected)
	}
}

func TestTracerMultiline(t *testing.T) {
	qt := New(true, "line1\nline2")
	qt.Printf("line3\nline4\n")
	qt.Done()
	s := qt.String()
	sExpected := `- 0ms: : line1
| line2
| - 0ms: line3
| | line4
`
	if !areEqualTracesSkipDuration(s, sExpected) {
		t.Fatalf("unexpected trace\ngot\n%s\nwant\n%s", s, sExpected)
	}
}

func TestTracerToJSON(t *testing.T) {
	qt := New(true, "test")
	if !qt.Enabled() {
		t.Fatalf("query tracer must be enabled")
	}
	qtChild := qt.NewChild("child done %d", 456)
	if !qtChild.Enabled() {
		t.Fatalf("child query tracer must be enabled")
	}
	qtChild.Printf("foo %d", 123)
	qtChild.Done()
	qt.Printf("parent %d", 789)
	qt.Done()
	s := qt.ToJSON()
	sExpected := `{"duration_msec":0,"message":": test","children":[` +
		`{"duration_msec":0,"message":"child done 456","children":[` +
		`{"duration_msec":0,"message":"foo 123"}]},` +
		`{"duration_msec":0,"message":"parent 789"}]}`
	if !areEqualJSONTracesSkipDuration(s, sExpected) {
		t.Fatalf("unexpected trace\ngot\n%s\nwant\n%s", s, sExpected)
	}
}

func TestTraceAddJSON(t *testing.T) {
	qtChild := New(true, "child")
	qtChild.Printf("foo")
	qtChild.Done()
	jsonTrace := qtChild.ToJSON()
	qt := New(true, "parent")
	qt.Printf("first_line")
	if err := qt.AddJSON([]byte(jsonTrace)); err != nil {
		t.Fatalf("unexpected error in AddJSON: %s", err)
	}
	qt.Printf("last_line")
	if err := qt.AddJSON(nil); err != nil {
		t.Fatalf("unexpected error in AddJSON(nil): %s", err)
	}
	qt.Done()
	s := qt.String()
	sExpected := `- 0ms: : parent
| - 0ms: first_line
| - 0ms: : child
| | - 0ms: foo
| - 0ms: last_line
`
	if !areEqualTracesSkipDuration(s, sExpected) {
		t.Fatalf("unexpected trace\ngot\n%s\nwant\n%s", s, sExpected)
	}

	jsonS := qt.ToJSON()
	jsonSExpected := `{"duration_msec":0,"message":": parent","children":[` +
		`{"duration_msec":0,"message":"first_line"},` +
		`{"duration_msec":0,"message":": child","children":[` +
		`{"duration_msec":0,"message":"foo"}]},` +
		`{"duration_msec":0,"message":"last_line"}]}`
	if !areEqualJSONTracesSkipDuration(jsonS, jsonSExpected) {
		t.Fatalf("unexpected trace\ngot\n%s\nwant\n%s", jsonS, jsonSExpected)
	}
}

func TestTraceMissingDonef(t *testing.T) {
	qt := New(true, "parent")
	qt.Printf("parent printf")
	qtChild := qt.NewChild("child")
	qtChild.Printf("child printf")
	qt.Printf("another parent printf")
	s := qt.String()
	sExpected := `- 0.000ms: missing Tracer.Done() call for the trace with message=: parent
`
	if !areEqualTracesSkipDuration(s, sExpected) {
		t.Fatalf("unexpected trace\ngot\n%s\nwant\n%s", s, sExpected)
	}
}

func TestTraceConcurrent(t *testing.T) {
	qt := New(true, "parent")
	childLocal := qt.NewChild("local")
	childLocal.Printf("abc")
	childLocal.Done()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		child := qt.NewChild(fmt.Sprintf("child %d", i))
		wg.Add(1)
		go func() {
			for j := 0; j < 100; j++ {
				child.Printf(fmt.Sprintf("message %d", j))
			}
			wg.Done()
		}()
	}
	qt.Done()
	// Verify that it is safe to call qt.String() when child traces aren't done yet
	s := qt.String()
	wg.Wait()
	sExpected := `- 0.008ms: : parent
| - 0.002ms: local
| | - 0.000ms: abc
| - 0.000ms: missing Tracer.Done() call for the trace with message=child 0
| - 0.000ms: missing Tracer.Done() call for the trace with message=child 1
| - 0.000ms: missing Tracer.Done() call for the trace with message=child 2
`
	if !areEqualTracesSkipDuration(s, sExpected) {
		t.Fatalf("unexpected trace\ngot\n%s\nwant\n%s", s, sExpected)
	}
}

func TestZeroDurationInTrace(t *testing.T) {
	s := `- 123ms: missing Tracer.Donef() call
| - 0ms: parent printf
| - 54ms: missing Tracer.Donef() call
| | - 45ms: child printf
| - 0ms: another parent printf
`
	result := zeroDurationsInTrace(s)
	resultExpected := `- 0ms: missing Tracer.Donef() call
| - 0ms: parent printf
| - 0ms: missing Tracer.Donef() call
| | - 0ms: child printf
| - 0ms: another parent printf
`
	if result != resultExpected {
		t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
	}
}

func TestZeroJSONDurationInTrace(t *testing.T) {
	s := `{"duration_msec":123,"message":"parent","children":[` +
		`{"duration_msec":0,"message":"first_line"},` +
		`{"duration_msec":434,"message":"child","children":[` +
		`{"duration_msec":343,"message":"foo"}]},` +
		`{"duration_msec":0,"message":"last_line"}]}`
	result := zeroJSONDurationsInTrace(s)
	resultExpected := `{"duration_msec":0,"message":"parent","children":[` +
		`{"duration_msec":0,"message":"first_line"},` +
		`{"duration_msec":0,"message":"child","children":[` +
		`{"duration_msec":0,"message":"foo"}]},` +
		`{"duration_msec":0,"message":"last_line"}]}`
	if result != resultExpected {
		t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
	}
}

func areEqualTracesSkipDuration(s1, s2 string) bool {
	s1 = zeroDurationsInTrace(s1)
	s2 = zeroDurationsInTrace(s2)
	return s1 == s2
}

func zeroDurationsInTrace(s string) string {
	return skipDurationRe.ReplaceAllString(s, " 0ms: ")
}

var skipDurationRe = regexp.MustCompile(" [0-9.]+ms: ")

func areEqualJSONTracesSkipDuration(s1, s2 string) bool {
	s1 = zeroJSONDurationsInTrace(s1)
	s2 = zeroJSONDurationsInTrace(s2)
	return s1 == s2
}

func zeroJSONDurationsInTrace(s string) string {
	return skipJSONDurationRe.ReplaceAllString(s, `"duration_msec":0`)
}

var skipJSONDurationRe = regexp.MustCompile(`"duration_msec":[0-9.]+`)
