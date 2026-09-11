package querytracer

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// OTLPTrace is an intermediate representation of a finished tracer tree
// suitable for conversion to OTLP protobuf spans.
type OTLPTrace struct {
	TraceID string
	Spans   []OTLPSpan
}

// OTLPSpan represents a single span ready for OTLP export.
type OTLPSpan struct {
	TraceID      string
	SpanID       string
	ParentSpanID string // empty for root
	Name         string
	StartNano    uint64
	EndNano      uint64
	Events       []OTLPEvent
}

// OTLPEvent is a timestamped annotation within a span (from Printf leaf nodes).
type OTLPEvent struct {
	TimeNano uint64
	Name     string
}

// ToOTLPTrace converts the finished tracer tree into a flat list of OTLP spans.
// It must be called after Done/Donef.
func (t *Tracer) ToOTLPTrace() *OTLPTrace {
	if t == nil {
		return nil
	}
	traceID := newTraceID()
	tr := &OTLPTrace{TraceID: traceID}
	collectSpans(tr, t, traceID, "", t.startTime)
	return tr
}

// collectSpans recursively walks the tracer tree and populates tr.Spans.
// parentSpanID is empty for the root.
// prevTime is used only when approximating timestamps for JSON-embedded spans.
func collectSpans(tr *OTLPTrace, t *Tracer, traceID, parentSpanID string, prevTime time.Time) {
	if t.span != nil {
		collectJSONSpans(tr, t.span, traceID, parentSpanID, prevTime)
		return
	}

	isLeaf := t.doneTime.Equal(t.startTime)
	if isLeaf {
		// Printf leaf: no span to emit here; the caller handles attaching it.
		return
	}

	// Regular span.
	spanID := newSpanID()
	spanIdx := len(tr.Spans)
	tr.Spans = append(tr.Spans, OTLPSpan{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		Name:         t.message,
		StartNano:    uint64(t.startTime.UnixNano()),
		EndNano:      uint64(t.doneTime.UnixNano()),
	})

	// Process children. Leaf children become events on this span; non-leaf
	// children recurse and produce their own spans.
	childPrev := t.startTime
	for _, child := range t.children {
		if child.span == nil && child.doneTime.Equal(child.startTime) {
			// Printf leaf: attach as span event.
			tr.Spans[spanIdx].Events = append(tr.Spans[spanIdx].Events, OTLPEvent{
				TimeNano: uint64(child.startTime.UnixNano()),
				Name:     child.message,
			})
			continue
		}
		collectSpans(tr, child, traceID, spanID, childPrev)
		if !child.doneTime.IsZero() && !child.doneTime.Equal(child.startTime) {
			childPrev = child.doneTime
		}
	}
}

// collectJSONSpans converts a span tree that was deserialized via AddJSON.
// Timestamps are approximated from prevTime since JSON spans only carry duration_msec.
func collectJSONSpans(tr *OTLPTrace, s *span, traceID, parentSpanID string, prevTime time.Time) {
	startNano := uint64(prevTime.UnixNano())
	durationNano := uint64(s.DurationMsec * float64(time.Millisecond))
	spanID := newSpanID()
	tr.Spans = append(tr.Spans, OTLPSpan{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		Name:         s.Message,
		StartNano:    startNano,
		EndNano:      startNano + durationNano,
	})
	childPrev := prevTime
	for _, child := range s.Children {
		collectJSONSpans(tr, child, traceID, spanID, childPrev)
		childPrev = childPrev.Add(time.Duration(child.DurationMsec * float64(time.Millisecond)))
	}
}

func newTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func newSpanID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
