package pb

// trace_fields.go contains field names when storing OTLP trace span data in VictoriaLogs.

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
	TraceIDField                = "trace_id"
	SpanIDField                 = "span_id"
	TraceStateField             = "trace_state"
	ParentSpanIDField           = "parent_span_id"
	FlagsField                  = "flags"
	NameField                   = "name"
	KindField                   = "kind"
	StartTimeUnixNanoField      = "start_time_unix_nano"
	EndTimeUnixNanoField        = "end_time_unix_nano"
	SpanAttrPrefixField         = "span_attr:"
	DroppedAttributesCountField = "dropped_attributes_count"
	// Span_Event Here
	DroppedEventsCountField = "dropped_events_count"
	// Span_Link Here
	DroppedLinksCountField = "dropped_links_count"
	// Status Here

	// DurationField field is calculated by end-start to allow duration filter on span.
	// It's not part of OTLP.
	DurationField = "duration"
)

// Span_Event
const (
	EventPrefix = "event:"

	EventTimeUnixNanoField           = "event_time_unix_nano"
	EventNameField                   = "event_name"
	EventAttrPrefix                  = "event_attr:"
	EventDroppedAttributesCountField = "event_dropped_attributes_count"
)

// Span_Link
const (
	LinkPrefix = "link:"

	LinkTraceIDField                = "link_trace_id"
	LinkSpanIDField                 = "link_span_id"
	LinkTraceStateField             = "link_trace_state"
	LinkAttrPrefix                  = "link_attr:"
	LinkDroppedAttributesCountField = "link_dropped_attributes_count"
	LinkFlagsField                  = "link_flags"
)

// Status
const (
	StatusMessageField = "status_message"
	StatusCodeField    = "status_code"
)
