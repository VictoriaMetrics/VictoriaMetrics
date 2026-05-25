package influx2

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

// Config holds the connection and migration settings for InfluxDB v2.
// It intentionally drops v1 concepts like Username, Password, Database,
// and RetentionPolicy — v2 replaced all of those with Token, Org, and Bucket.
type Config struct {
	// Addr is the full URL of the InfluxDB v2 server, e.g. "http://localhost:8086".
	Addr string

	// Token is the API token used for auth. Every request sends:
	// Authorization: Token <token>
	Token string

	// Org is the organization name. In v2, all resources live inside an org,
	// and queries must be scoped to one.
	Org string

	// Bucket is what you're migrating from. It replaces v1's database+retention combo.
	Bucket string

	// ChunkSize controls how many rows come back per streaming page.
	ChunkSize int

	// Filter optionally restricts the time range of data to migrate.
	Filter Filter

	// TLSConfig lets callers pass custom certs, a private CA, or skip verification.
	// Nil means use system defaults.
	TLSConfig *tls.Config
}

// Filter restricts the migration to a time window.
// Both are RFC3339 strings, e.g. "2021-01-01T00:00:00Z". Empty = no limit.
type Filter struct {
	TimeStart string
	TimeEnd   string
}

// Series is one unique time series: a (measurement, field, tag-set) triple.
// For example, cpu.usage_idle with host=a and cpu.usage_idle with host=b
// are two different Series even though measurement and field are the same.
// Explore() returns one *Series per unique combination so we can fetch each separately.
type Series struct {
	Measurement string
	Field       string
	LabelPairs  []LabelPair
}

// LabelPair is a single tag key-value pair, e.g. host=server01.
// After migration these become Prometheus-style labels in VictoriaMetrics.
type LabelPair struct {
	Name  string
	Value string
}

// Client wraps the InfluxDB v2 QueryAPI and holds the migration config.
// We store QueryAPI (not the raw influxdb2.Client) because that's the
// only surface we need — just running Flux queries, no writes.
type Client struct {
	queryAPI  api.QueryAPI
	bucket    string
	chunkSize int
	filter    Filter
}

// NewClient connects to InfluxDB v2 and returns a ready-to-use Client.
// It runs a health check immediately so the user gets a clear error upfront
// rather than discovering a bad token or wrong URL mid-migration.
func NewClient(cfg Config) (*Client, error) {
	opts := influxdb2.DefaultOptions()
	if cfg.TLSConfig != nil {
		opts = opts.SetTLSConfig(cfg.TLSConfig)
	}

	// NewClientWithOptions configures the HTTP client with token auth.
	// No network call happens yet — this is just setup.
	c := influxdb2.NewClientWithOptions(cfg.Addr, cfg.Token, opts)

	// Health() hits GET /health — it tells us if the server is up and
	// if the token has at least read access. v1 used Ping(), but v2
	// replaced /ping with /health which gives richer status info.
	health, err := c.Health(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to reach InfluxDB v2 at %s: %w", cfg.Addr, err)
	}
	log.Printf("connected to InfluxDB v2 at %s (status: %s)", cfg.Addr, health.Status)

	chunkSize := cfg.ChunkSize
	if chunkSize < 1 {
		chunkSize = 10_000
	}

	return &Client{
		queryAPI:  c.QueryAPI(cfg.Org),
		bucket:    cfg.Bucket,
		chunkSize: chunkSize,
		filter:    cfg.Filter,
	}, nil
}

// Bucket returns the bucket name so the processor can optionally attach
// a "bucket" label to migrated series (same idea as v1's "db" label).
func (c *Client) Bucket() string {
	return c.bucket
}

// Explore discovers all unique time series in the bucket.
// It first gets all (measurement, field) pairs via a cheap metadata query,
// then for each pair finds every unique tag combination by reading one row
// per group. Each unique triple (measurement + field + tags) becomes one *Series.
func (c *Client) Explore() ([]*Series, error) {
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

// fieldsByMeasurement returns a map of measurement name → list of field names.
//
// We scan a single row per (measurement, field) combination using keep() +
// distinct(). This is more reliable than schema.fieldKeys() which on some
// InfluxDB v2 versions omits the _measurement column from its output,
// making it impossible to group fields by measurement.
//
// Flux:
//
//	from(bucket: "mybucket")
//	  |> range(start: <min>)
//	  |> keep(columns: ["_measurement", "_field"])
//	  |> distinct(column: "_field")
//
// Each row has _measurement, _field, and _value (which equals the field name).
func (c *Client) fieldsByMeasurement() (map[string][]string, error) {
	start, _ := c.timeRange()
	query := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: %s)
  |> keep(columns: ["_measurement", "_field"])
  |> distinct(column: "_field")
`, c.bucket, start)

	result, err := c.queryAPI.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("fieldKeys query error: %w", err)
	}
	defer result.Close()

	fields := make(map[string][]string)
	for result.Next() {
		r := result.Record()

		// After distinct(column: "_field"), each row has:
		//   _measurement — the measurement this field belongs to
		//   _field       — the field name (also mirrored in _value)
		// We read _field directly via ValueByKey instead of r.Field()
		// because r.Field() may be empty after the keep()+distinct() pipeline.
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
// Flux groups data into tables by tag set automatically, so first() gives us
// one row per unique tag combination — exactly what we need without reading
// all the data.
//
// Flux:
//
//	from(bucket: "mybucket")
//	  |> range(start: ..., stop: ...)
//	  |> filter(fn: (r) => r._measurement == "cpu" and r._field == "usage_idle")
//	  |> first()
func (c *Client) getTagSets(measurement, field string) ([][]LabelPair, error) {
	start, stop := c.timeRange()

	query := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: %s, stop: %s)
  |> filter(fn: (r) => r._measurement == "%s" and r._field == "%s")
  |> first()
`, c.bucket, start, stop, measurement, field)

	result, err := c.queryAPI.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("tag set query failed for %q.%q: %w", measurement, field, err)
	}
	defer result.Close()

	var tagSets [][]LabelPair

	for result.Next() {
		// first() returns exactly one row per Flux table, and one table = one
		// unique tag set. So every row here is a distinct tag combination —
		// we collect tags unconditionally rather than gating on TableChanged().
		tags := extractTags(result.Record().Values())
		tagSets = append(tagSets, tags)
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("error reading tag sets for %q.%q: %w", measurement, field, err)
	}

	// If the series exists but has no tags, we still need to migrate it.
	if len(tagSets) == 0 {
		tagSets = append(tagSets, nil)
	}

	return tagSets, nil
}

// extractTags picks the user-defined tag columns out of a row's value map.
// A Flux row always contains system columns like _time, _value, _field, etc.
// alongside user tags — we skip the system ones and keep everything else.
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
		// fmt.Sprintf handles the case where a tag value comes back as a
		// non-string type from some client versions.
		tags = append(tags, LabelPair{Name: k, Value: fmt.Sprintf("%v", v)})
	}
	return tags
}

// ChunkedResponse wraps a streaming Flux result so the processor can call
// Next() in a loop without holding all data in memory. Each call returns
// one batch of (timestamps, values) from the current Flux table (tag set).
type ChunkedResponse struct {
	result *api.QueryTableResult
	done   bool // true once the result stream is fully consumed
}

// Close releases the underlying HTTP response body. Always defer this.
func (cr *ChunkedResponse) Close() error {
	cr.result.Close()
	return nil
}

// Next reads rows from the stream until either the table changes (meaning we
// hit a different tag set) or the stream ends. Returns empty slices and nil
// error when fully consumed.
// vm.Importer.Input() needs one batch per series, so we stop at table boundaries
// to avoid mixing data from different tag sets into one TimeSeries.
func (cr *ChunkedResponse) Next() ([]int64, []float64, error) {
	// Once the stream is fully consumed, every subsequent call returns empty —
	// this is how do() knows migration for this series is complete.
	if cr.done {
		return nil, nil, nil
	}

	var timestamps []int64
	var values []float64

	for cr.result.Next() {
		// Stop when we move to a new table — the caller will process what we
		// have so far and call Next() again for the next series.
		if cr.result.TableChanged() && len(timestamps) > 0 {
			break
		}

		r := cr.result.Record()

		// record.Time() gives us time.Time directly. No string parsing needed,
		// unlike v1 where timestamps came back as RFC3339 strings.
		ts := fluxTimeToMillis(r.Time())
		timestamps = append(timestamps, ts)

		v, err := toFloat64(r.Value())
		if err != nil {
			return nil, nil, fmt.Errorf("cannot convert value to float64: %w", err)
		}
		values = append(values, v)
	}

	// Only check Err() when we're still in the middle of the stream (non-empty
	// batch). When the inner loop exits with no rows, the HTTP body is already
	// closed — calling Err() at that point can return a spurious "read on closed
	// response body" error that isn't a real failure.
	if len(timestamps) == 0 {
		cr.done = true
		return nil, nil, nil
	}

	if err := cr.result.Err(); err != nil {
		return nil, nil, fmt.Errorf("stream error: %w", err)
	}

	return timestamps, values, nil
}

// FetchDataPoints fetches all data points for a single Series.
// It builds a Flux query that filters to exactly one (measurement, field, tag-set)
// combination and returns a streaming ChunkedResponse.
//
// Flux equivalent of v1's:
//
//	SELECT "usage_idle" FROM "cpu" WHERE "host"='server01' AND time >= ...
func (c *Client) FetchDataPoints(s *Series) (*ChunkedResponse, error) {
	start, stop := c.timeRange()

	// Build the per-tag filter conditions. We use r["tagname"] bracket syntax
	// instead of r.tagname because tag names can contain hyphens, dots, or
	// other characters that aren't valid Flux identifiers.
	tagFilter := ""
	for _, lp := range s.LabelPairs {
		tagFilter += fmt.Sprintf(` and r["%s"] == "%s"`, lp.Name, lp.Value)
	}

	// sort by _time so timestamps arrive in ascending order.
	// VictoriaMetrics handles out-of-order points but it's cheaper to sort here.
	query := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: %s, stop: %s)
  |> filter(fn: (r) => r._measurement == "%s" and r._field == "%s"%s)
  |> sort(columns: ["_time"])
`, c.bucket, start, stop, s.Measurement, s.Field, tagFilter)

	result, err := c.queryAPI.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("data fetch failed for %q.%q: %w", s.Measurement, s.Field, err)
	}

	return &ChunkedResponse{result: result}, nil
}

// timeRange returns Flux-formatted start and stop strings based on the Filter.
// The default start is InfluxDB's minimum representable nanosecond timestamp
// (math.MinInt64 nanos from epoch), which effectively means "all historical data".
// The default stop is now() which Flux evaluates at query time.
func (c *Client) timeRange() (start, stop string) {
	start = "1677-09-21T00:12:43.145224194Z"
	if c.filter.TimeStart != "" {
		start = c.filter.TimeStart
	}
	stop = "now()"
	if c.filter.TimeEnd != "" {
		stop = c.filter.TimeEnd
	}
	return start, stop
}
