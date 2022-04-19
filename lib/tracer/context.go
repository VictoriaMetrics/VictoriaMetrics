package tracer

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// whether enable tracing
	traceEnableParam = "trace_query"
	// whether to print tracing with colors in stdout
	traceEnableColorParam = "trace_query_color"
	// specific trace ID. If omitted a random ID will be generated
	traceIDParam = "trace_query_id"
)

// NewContext creates a new instance of the Context.
// If enabled=false, all function calls to the returned
// object will be no-op.
func NewContext(r *http.Request) *Context {
	defaultCtx := &Context{enabled: false}
	if r == nil {
		return defaultCtx
	}

	enabled := getBool(r, traceEnableParam)
	id := r.FormValue(traceIDParam)

	if !enabled && id == "" {
		return defaultCtx
	}

	if id == "" {
		id = newID()
	}

	enableColor := getBool(r, traceEnableColorParam)
	now := time.Now()
	return &Context{
		id:      id,
		color:   enableColor,
		enabled: enabled,
		updated: now,
		start:   now,
	}
}

var nextID = uint64(time.Now().UnixNano())

func newID() string {
	id := atomic.AddUint64(&nextID, 1)
	return fmt.Sprintf("%08X", id)
}

// Context represents a tracing context.
// Context must be created via NewContext func.
// Each created context must be finalized via Done func.
//
// Context might contain sub-contexts (branches)
// in order to build tree-like execution order.
// To add a sub-context use Add func.
type Context struct {
	// id is an optional param and used only for the root context
	id string
	// msg is the message/title of the current context
	msg string
	// start is the time when Context was created
	start time.Time
	// enabled makes all function calls no-op if set to false
	enabled bool
	// color defines whether to print tracing with colors in stdout
	color bool

	mu sync.Mutex
	// updated is the time when Context was updated last time
	updated time.Time
	// subContexts is a list of related Context
	subContexts []*Context
}

// ID returns a context ID
func (t *Context) ID() string {
	return t.id
}

// Add adds a new Context (sub-context) object to the caller
// Use Add in case if you want to "branch" current context.
func (t *Context) Add() *Context {
	if !t.enabled {
		return t
	}

	now := time.Now()
	newT := &Context{
		updated: now,
		start:   now,
		enabled: t.enabled,
		color:   t.color,
	}

	t.mu.Lock()
	t.subContexts = append(t.subContexts, newT)
	t.updated = now
	t.mu.Unlock()

	return newT
}

// Done finishes the current context by setting
// a message (will be used as a branch name)
// and updating the execution time.
func (t *Context) Done(fn func() string) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	t.updated = time.Now()
	t.msg = fn()
	t.mu.Unlock()
}

// Print returns a tree-like structure of all
// sub-contexts respecting the order of their adding.
func (t *Context) Print() string {
	return t.print(0)
}

func (t *Context) print(indentation int) string {
	if !t.enabled {
		return ""
	}
	var ind string
	for i := 0; i < indentation; i++ {
		ind += "--"
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.msg == "" {
		t.msg = "error: context wasn't finalized properly"
	}
	elapsed := t.updated.Sub(t.start)

	b := strings.Builder{}
	if t.color {
		b.WriteString(getColor(indentation))
	}
	b.WriteString(fmt.Sprintf("%s %dms: %s", ind, elapsed.Milliseconds(), t.msg))
	if t.color {
		b.WriteString(colorReset)
	}

	for _, sub := range t.subContexts {
		b.WriteString("\n")
		b.WriteString(sub.print(indentation + 1))
	}
	return b.String()
}

var colors = []string{
	"\033[34m", // blue
	"\033[32m", // green
	"\033[33m", // yellow
	"\033[35m", // purple
	"\033[36m", // cyan
	"\033[31m", // red
	"\033[37m", // gray
	"\033[38m", // white
}

var colorReset = "\033[0m"

func getColor(i int) string {
	return colors[i%len(colors)]
}

// PrintJSON returns a JSON representation
// of the context and all sub-contexts respecting
// the order of their adding.
func (t *Context) PrintJSON() string {
	if !t.enabled {
		return ""
	}
	t.mu.Lock()
	res := ctxToJson(t)
	t.mu.Unlock()
	return res
}

// getBool returns boolean value from the given argKey query arg.
func getBool(r *http.Request, argKey string) bool {
	argValue := r.FormValue(argKey)
	switch strings.ToLower(argValue) {
	case "", "0", "f", "false", "no":
		return false
	default:
		return true
	}
}
