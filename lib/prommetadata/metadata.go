package prommetadata

import "flag"

var enableMetadata = flag.Bool("enableMetadata", false, "Whether to enable metadata processing for metrics scraped from targets, received via VictoriaMetrics remote write, Prometheus remote write v1 or OpenTelemetry protocol. "+
	"See also remoteWrite.maxMetadataPerBlock")

// IsEnabled reports whether metadata processing is enabled.
func IsEnabled() bool {
	return *enableMetadata
}

// SetEnabled sets enableMetadata to v and returns the previous value of enableMetadata.
// This function is intended for promscrape tests.
func SetEnabled(v bool) bool {
	prev := *enableMetadata
	*enableMetadata = v
	return prev
}
