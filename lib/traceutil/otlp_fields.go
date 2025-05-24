package traceutil

// Resource
const (
	ResourceAttrPrefix      = "resource_attr:"
	ResourceAttrServiceName = "resource_attr:service.name" // ResourceAttrServiceName service name is a special resource attribute
)

// ScopeSpans - InstrumentationScope
const (
	InstrumentationScopeName       = "scope_name"
	InstrumentationScopeVersion    = "scope_version"
	InstrumentationScopeAttrPrefix = "scope_attr:"
)

// Span
const (
	TraceId                = "trace_id"
	SpanId                 = "span_id"
	TraceState             = "trace_state"
	ParentSpanId           = "parent_span_id"
	Flags                  = "flags"
	Name                   = "name"
	Kind                   = "kind"
	StartTimeUnixNano      = "start_time_unix_nano"
	EndTimeUnixNano        = "end_time_unix_nano"
	SpanAttrPrefix         = "span_attr:"
	DroppedAttributesCount = "dropped_attributes_count"
	// Span_Event Here
	DroppedEventsCount = "dropped_events_count"
	// Span_Link Here
	DroppedLinksCount = "dropped_links_count"
	// Status Here

	// Duration field is calculated by end-start to allow duration filter on span.
	// It's not part of OTLP.
	Duration = "duration"
)

// Span_Event
const (
	EventPrefix = "event:"

	EventTimeUnixNano           = "event_time_unix_nano"
	EventName                   = "event_name"
	EventAttrPrefix             = "event_attr:"
	EventDroppedAttributesCount = "event_dropped_attributes_count"
)

// Span_Link
const (
	LinkPrefix = "link:"

	LinkTraceId                = "link_trace_id"
	LinkSpanId                 = "link_span_id"
	LinkTraceState             = "link_trace_state"
	LinkAttrPrefix             = "link_attr:"
	LinkDroppedAttributesCount = "link_dropped_attributes_count"
	LinkFlags                  = "link_flags"
)

// Status
const (
	StatusMessage = "status_message"
	StatusCode    = "status_code"
)
