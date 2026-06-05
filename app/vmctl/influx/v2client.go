package influx

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

// influxMinTime is time.Unix(0, math.MinInt64).UTC() formatted as RFC3339Nano.
// It is the earliest nanosecond timestamp InfluxDB can represent and is used
// as the default "fetch all historical data" start when no time filter is set.
const influxMinTime = "1677-09-21T00:12:43.145224194Z"

// V2Config holds the connection and migration settings for InfluxDB v2.
type V2Config struct {
	Addr      string
	Token     string
	Org       string
	Bucket    string
	ChunkSize int
	Filter    Filter
	TLSConfig *tls.Config
}

// V2Client talks to InfluxDB v2 via Flux queries and implements Source.
type V2Client struct {
	queryAPI  api.QueryAPI
	bucket    string
	chunkSize int
	filter    Filter
}

// NewV2Client connects to InfluxDB v2, runs a health check, and returns a ready client.
func NewV2Client(cfg V2Config) (*V2Client, error) {
	opts := influxdb2.DefaultOptions()
	if cfg.TLSConfig != nil {
		opts = opts.SetTLSConfig(cfg.TLSConfig)
	}
	c := influxdb2.NewClientWithOptions(cfg.Addr, cfg.Token, opts)
	health, err := c.Health(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to reach InfluxDB v2 at %s: %w", cfg.Addr, err)
	}
	log.Printf("connected to InfluxDB v2 at %s (status: %s)", cfg.Addr, health.Status)

	chunkSize := cfg.ChunkSize
	if chunkSize < 1 {
		chunkSize = 10_000
	}
	return &V2Client{
		queryAPI:  c.QueryAPI(cfg.Org),
		bucket:    cfg.Bucket,
		chunkSize: chunkSize,
		filter:    cfg.Filter,
	}, nil
}

// Bucket returns the bucket name.
func (c *V2Client) Bucket() string {
	return c.bucket
}

// Label implements Source: attaches a "bucket" label so you can tell which
// InfluxDB bucket the data came from after migration.
func (c *V2Client) Label() (name, value string) {
	return "bucket", c.bucket
}

// Explore discovers all unique time series in the bucket.
func (c *V2Client) Explore() ([]*Series, error) {
	log.Printf("exploring schema for bucket %q", c.bucket)

	mFields, err := c.fieldsByMeasurement()
	if err != nil {
		return nil, fmt.Errorf("failed to get field keys: %w", err)
	}
	if len(mFields) == 0 {
		return nil, fmt.Errorf("no fields found in bucket %q", c.bucket)
	}

	var result []*Series
	for measurement, fields := range mFields {
		for _, field := range fields {
			tagSets, err := c.getTagSets(measurement, field)
			if err != nil {
				return nil, fmt.Errorf("failed to get tag sets for %q.%q: %w", measurement, field, err)
			}
			for _, tagSet := range tagSets {
				result = append(result, &Series{
					Measurement: measurement,
					Field:       field,
					LabelPairs:  tagSet,
				})
			}
		}
	}

	log.Printf("found %d time series in bucket %q", len(result), c.bucket)
	return result, nil
}

// fieldsByMeasurement returns a map of measurement → list of field names.
// Uses keep()+distinct() rather than schema.fieldKeys() because some InfluxDB v2
// versions omit the _measurement column from schema.fieldKeys() output.
func (c *V2Client) fieldsByMeasurement() (map[string][]string, error) {
	start, _ := c.timeRange()
	query := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: time(v: "%s"))
  |> keep(columns: ["_measurement", "_field"])
  |> distinct(column: "_field")
`, escapeFlux(c.bucket), start)
	result, err := c.queryAPI.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("fieldKeys query error: %w", err)
	}
	defer result.Close()

	fields := make(map[string][]string)
	for result.Next() {
		r := result.Record()
		measurement, _ := r.ValueByKey("_measurement").(string)
		fieldName, _ := r.ValueByKey("_field").(string)
		if measurement == "" || fieldName == "" {
			continue
		}
		fields[measurement] = append(fields[measurement], fieldName)
	}
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("error reading fieldKeys: %w", err)
	}
	log.Printf("found fields in %d measurements", len(fields))
	return fields, nil
}

// getTagSets finds all unique tag combinations for one (measurement, field) pair.
// first() returns one row per Flux table, and one table = one unique tag set.
func (c *V2Client) getTagSets(measurement, field string) ([][]LabelPair, error) {
	start, stop := c.timeRange()
	query := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: time(v: "%s"), stop: %s)
  |> filter(fn: (r) => r._measurement == "%s" and r._field == "%s")
  |> first()
`, escapeFlux(c.bucket), start, fluxStopExpr(stop), escapeFlux(measurement), escapeFlux(field))
	result, err := c.queryAPI.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("tag set query failed for %q.%q: %w", measurement, field, err)
	}
	defer result.Close()

	var tagSets [][]LabelPair
	for result.Next() {
		tags := extractTags(result.Record().Values())
		tagSets = append(tagSets, tags)
	}
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("error reading tag sets for %q.%q: %w", measurement, field, err)
	}
	if len(tagSets) == 0 {
		tagSets = append(tagSets, nil)
	}
	return tagSets, nil
}

// FetchDataPoints fetches all data points for a single Series.
func (c *V2Client) FetchDataPoints(s *Series) (ChunkedResponse, error) {
	start, stop := c.timeRange()

	// Tag keys and values cannot go through QueryWithParams because the number
	// of conditions is variable. We escape both with escapeFlux() so a tag like
	// host=`a"b` doesn't break the string literal.
	tagFilter := ""
	for _, lp := range s.LabelPairs {
		tagFilter += fmt.Sprintf(` and r["%s"] == "%s"`, escapeFlux(lp.Name), escapeFlux(lp.Value))
	}

	query := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: time(v: "%s"), stop: %s)
  |> filter(fn: (r) => r._measurement == "%s" and r._field == "%s"%s)
  |> sort(columns: ["_time"])
`, escapeFlux(c.bucket), start, fluxStopExpr(stop), escapeFlux(s.Measurement), escapeFlux(s.Field), tagFilter)

	result, err := c.queryAPI.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("data fetch failed for %q.%q: %w", s.Measurement, s.Field, err)
	}
	return &v2ChunkedResponse{result: result}, nil
}

// timeRange returns the Flux start and stop strings derived from the Filter.
// Both callers (getTagSets and FetchDataPoints) use both values, so returning
// the pair keeps the logic in one place and avoids referencing filter fields directly.
func (c *V2Client) timeRange() (start, stop string) {
	start = influxMinTime
	if c.filter.TimeStart != "" {
		start = c.filter.TimeStart
	}
	// Empty stop means "now()" — callers embed this in a Flux conditional
	// `if params.Stop == "" then now() else time(v: params.Stop)` so they
	// never need to special-case the empty string themselves.
	stop = ""
	if c.filter.TimeEnd != "" {
		stop = c.filter.TimeEnd
	}
	return start, stop
}

// extractTags picks the user-defined tag columns out of a row's value map,
// skipping Flux system columns like _time, _value, _field, etc.
func extractTags(values map[string]interface{}) []LabelPair {
	systemCols := map[string]bool{
		"_time": true, "_value": true, "_field": true,
		"_measurement": true, "_start": true, "_stop": true,
		"result": true, "table": true,
	}
	var tags []LabelPair
	for k, v := range values {
		if systemCols[k] {
			continue
		}
		tags = append(tags, LabelPair{Name: k, Value: fmt.Sprintf("%v", v)})
	}
	return tags
}

// v2ChunkedResponse streams a Flux query result one Flux-table worth of rows
// at a time. Each table corresponds to one unique tag set.
//
// The pending fields handle the fact that api.QueryTableResult.Next() consumes
// the first row of a new table when it detects a table change — we buffer that
// row so it is not lost and emit it at the start of the subsequent Next() call.
type v2ChunkedResponse struct {
	result     *api.QueryTableResult
	done       bool
	hasPending bool
	pendingTS  int64
	pendingVal float64
}

// Close releases the underlying HTTP response body.
func (cr *v2ChunkedResponse) Close() error {
	cr.result.Close()
	return nil
}

// Next reads rows until either the Flux table changes (different tag set) or
// the stream ends. Returns empty slices and nil error when fully consumed.
func (cr *v2ChunkedResponse) Next() ([]int64, []float64, error) {
	if cr.done {
		return nil, nil, nil
	}

	var timestamps []int64
	var values []float64

	// Emit the buffered first row of the previous table boundary, if any.
	if cr.hasPending {
		timestamps = append(timestamps, cr.pendingTS)
		values = append(values, cr.pendingVal)
		cr.hasPending = false
	}

	for cr.result.Next() {
		r := cr.result.Record()

		if cr.result.TableChanged() && len(timestamps) > 0 {
			// We are on the first row of a new table and already have data for
			// the current table. Buffer this row so it is returned on the next
			// call — without this, result.Next() has already consumed the row
			// and calling Next() again would skip it entirely.
			ts := fluxTimeToMillis(r.Time())
			v, err := toFloat64(r.Value())
			if err != nil {
				return nil, nil, fmt.Errorf("cannot convert value to float64: %w", err)
			}
			cr.hasPending = true
			cr.pendingTS = ts
			cr.pendingVal = v
			break
		}

		ts := fluxTimeToMillis(r.Time())
		timestamps = append(timestamps, ts)

		v, err := toFloat64(r.Value())
		if err != nil {
			return nil, nil, fmt.Errorf("cannot convert value to float64: %w", err)
		}
		values = append(values, v)
	}

	if len(timestamps) == 0 {
		cr.done = true
		return nil, nil, nil
	}

	if err := cr.result.Err(); err != nil {
		return nil, nil, fmt.Errorf("stream error: %w", err)
	}

	return timestamps, values, nil
}

// fluxTimeToMillis converts a time value to milliseconds since the Unix epoch.
// VictoriaMetrics stores millisecond timestamps; sub-millisecond precision is lost.
func fluxTimeToMillis(t interface{ UnixNano() int64 }) int64 {
	return t.UnixNano() / 1e6
}

// fluxStopExpr returns a Flux expression for the range stop.
// An empty stop means "no upper bound" so we use now(); a non-empty stop is an
// RFC3339 timestamp which is safe to embed directly (only digits/dashes/colons/dots/Z).
func fluxStopExpr(stop string) string {
	if stop == "" {
		return "now()"
	}
	return fmt.Sprintf(`time(v: "%s")`, stop)
}

// escapeFlux escapes a string for embedding inside a Flux double-quoted string
// literal. Backslash is escaped before quote to avoid double-escaping.
func escapeFlux(s string) string {
	out := make([]byte, 0, len(s)+4)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			out = append(out, '\\', '\\')
		case '"':
			out = append(out, '\\', '"')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
