package v2

import (
	"context"
	"crypto/tls"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

type Config struct {
	Addr   string
	Token  string
	Org    string
	Bucket string

	Filter    Filter
	TLSConfig *tls.Config
}

// Filter contains configuration for filtering
// the timeseries
type Filter struct {
	TimeStart string
	TimeEnd   string
}

type Series struct {
	Measurement string
	Field       string
}

type Adapter struct {
	client   influxdb2.Client
	queryAPI api.QueryAPI
	bucket   string
	start    string
	stop     string
}

func (a *Adapter) Bucket() string {
	return a.bucket
}

func NewAdapter(cfg Config) *Adapter {
	client := influxdb2.NewClientWithOptions(
		cfg.Addr,
		cfg.Token,
		influxdb2.
			DefaultOptions().
			SetTLSConfig(cfg.TLSConfig),
	)
	adapter := &Adapter{
		client:   client,
		queryAPI: client.QueryAPI(cfg.Org),
		bucket:   cfg.Bucket,
		start:    "-inf",
		stop:     "now()",
	}
	if cfg.Filter.TimeStart != "" {
		adapter.start = fmt.Sprintf("%s", cfg.Filter.TimeStart)
	}
	if cfg.Filter.TimeEnd != "" {
		adapter.stop = fmt.Sprintf("%s", cfg.Filter.TimeEnd)
	}
	return adapter
}

func (a *Adapter) Explore(ctx context.Context) ([]Series, error) {
	query := fmt.Sprintf(`
		from(bucket: "%s")
    	|> range(start: %s, stop: %s)
    	|> first()`, a.bucket, a.start, a.stop)
	results, err := a.queryAPI.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query Influx for first record of each series: %w", err)
	}
	var series []Series
	for results.Next() {
		if err := results.Err(); err != nil {
			return nil, fmt.Errorf("failed to parse Influx record: %w", err)
		}
		result := results.Record()
		series = append(series, Series{
			Measurement: result.Measurement(),
			Field:       result.Field(),
		})
	}
	return series, nil
}

func (a *Adapter) Fetch(ctx context.Context, s Series) (*api.QueryTableResult, error) {
	results, err := a.queryAPI.Query(ctx, fmt.Sprintf(`
        from(bucket: "%s")
        |> range(start: %s, stop: %s)
        |> filter(fn: (r) => r["_measurement"] == "%s")
		|> filter(fn: (r) => r["_field"] == "%s")`,
		a.bucket, a.start, a.stop, s.Measurement, s.Field),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to query Influx for datapoints: %w", err,
		)
	}
	return results, nil
}

func (a *Adapter) Close() {
	a.client.Close()
}
