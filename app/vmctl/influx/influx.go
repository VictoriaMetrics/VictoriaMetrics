package influx

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	influx "github.com/influxdata/influxdb/client/v2"
)

// Client represents a wrapper over
// influx HTTP client
type Client struct {
	influx.Client

	database  string
	retention string
	chunkSize int

	filterSeries string
	filterTime   string
}

// Config contains fields required
// for Client configuration
type Config struct {
	Addr      string
	Username  string
	Password  string
	Database  string
	Retention string
	ChunkSize int

	Filter    Filter
	TLSConfig *tls.Config
}

// Filter contains configuration for filtering
// the timeseries
type Filter struct {
	Series    string
	TimeStart string
	TimeEnd   string
}

// Series holds the time series
type Series struct {
	Measurement string
	Field       string
	LabelPairs  []LabelPair
}

var valueEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

func (s Series) fetchQuery(timeFilter string) string {
	f := &strings.Builder{}
	fmt.Fprintf(f, "select %q from %q", s.Field, s.Measurement)
	if len(s.LabelPairs) > 0 || len(timeFilter) > 0 {
		f.WriteString(" where")
	}
	for i, pair := range s.LabelPairs {
		pairV := valueEscaper.Replace(pair.Value)
		fmt.Fprintf(f, " %q::tag='%s'", pair.Name, pairV)
		if i != len(s.LabelPairs)-1 {
			f.WriteString(" and")
		}
	}
	if len(timeFilter) > 0 {
		if len(s.LabelPairs) > 0 {
			f.WriteString(" and")
		}
		fmt.Fprintf(f, " %s", timeFilter)
	}
	return f.String()
}

// LabelPair is the key-value record
// of time series label
type LabelPair struct {
	Name  string
	Value string
}

// NewClient creates and returns influx client
// configured with passed Config
func NewClient(cfg Config) (*Client, error) {
	c := influx.HTTPConfig{
		Addr:      cfg.Addr,
		Username:  cfg.Username,
		Password:  cfg.Password,
		TLSConfig: cfg.TLSConfig,
	}
	hc, err := influx.NewHTTPClient(c)
	if err != nil {
		return nil, fmt.Errorf("failed to establish conn: %s", err)
	}
	if _, _, err := hc.Ping(time.Second); err != nil {
		return nil, fmt.Errorf("ping failed: %s", err)
	}

	chunkSize := cfg.ChunkSize
	if chunkSize < 1 {
		chunkSize = 10e3
	}

	client := &Client{
		Client:       hc,
		database:     cfg.Database,
		retention:    cfg.Retention,
		chunkSize:    chunkSize,
		filterTime:   timeFilter(cfg.Filter.TimeStart, cfg.Filter.TimeEnd),
		filterSeries: cfg.Filter.Series,
	}
	return client, nil
}

// Database returns database name
func (c Client) Database() string {
	return c.database
}

func timeFilter(start, end string) string {
	if start == "" && end == "" {
		return ""
	}
	var tf string
	if start != "" {
		tf = fmt.Sprintf("time >= '%s'", start)
	}
	if end != "" {
		if tf != "" {
			tf += " and "
		}
		tf += fmt.Sprintf("time <= '%s'", end)
	}
	return tf
}

// Explore checks the existing data schema in influx
// by checking available fields and series,
// which unique combination represents all possible
// time series existing in database.
// The explore required to reduce the load on influx
// by querying field of the exact time series at once,
// instead of fetching all of the values over and over.
//
// May contain non-existing time series.
func (c *Client) Explore() ([]*Series, error) {
	log.Printf("Exploring scheme for database %q", c.database)
	mFields, err := c.fieldsByMeasurement()
	if err != nil {
		return nil, fmt.Errorf("failed to get field keys: %s", err)
	}

	if len(mFields) < 1 {
		return nil, fmt.Errorf("found no numeric fields for import in database %q", c.database)
	}

	series, err := c.getSeries()
	if err != nil {
		return nil, fmt.Errorf("failed to get series: %s", err)
	}

	var iSeries []*Series
	for _, s := range series {
		fields, ok := mFields[s.Measurement]
		if !ok {
			log.Printf("skip measurement %q since it has no fields", s.Measurement)
			continue
		}
		for _, field := range fields {
			is := &Series{
				Measurement: s.Measurement,
				Field:       field,
				LabelPairs:  s.LabelPairs,
			}
			iSeries = append(iSeries, is)
		}
	}
	return iSeries, nil
}

// ChunkedResponse is a wrapper over influx.ChunkedResponse.
// Used for better memory usage control while iterating
// over huge time series.
type ChunkedResponse struct {
	cr    *influx.ChunkedResponse
	iq    influx.Query
	field string
}

// Close closes cr.
func (cr *ChunkedResponse) Close() error {
	return cr.cr.Close()
}

// Next reads the next part/chunk of time series.
// Returns io.EOF when time series was read entirely.
func (cr *ChunkedResponse) Next() ([]int64, []float64, error) {
	resp, err := cr.cr.NextResponse()
	if err != nil {
		return nil, nil, err
	}
	if resp.Error() != nil {
		return nil, nil, fmt.Errorf("response error for %s: %s", cr.iq.Command, resp.Error())
	}
	if len(resp.Results) != 1 {
		return nil, nil, fmt.Errorf("unexpected number of results in response: %d", len(resp.Results))
	}
	results, err := parseResult(resp.Results[0])
	if err != nil {
		return nil, nil, err
	}
	if len(results) < 1 {
		return nil, nil, nil
	}
	r := results[0]

	const key = "time"
	timestamps, ok := r.values[key]
	if !ok {
		return nil, nil, fmt.Errorf("response doesn't contain field %q", key)
	}

	fieldValues, ok := r.values[cr.field]
	if !ok {
		return nil, nil, fmt.Errorf("response doesn't contain filed %q", cr.field)
	}
	values := make([]float64, len(fieldValues))
	for i, fv := range fieldValues {
		v, err := toFloat64(fv)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert value %q.%v to float64: %s",
				cr.field, v, err)
		}
		values[i] = v
	}

	ts := make([]int64, len(results[0].values[key]))
	for i, v := range timestamps {
		t, err := parseDate(v.(string))
		if err != nil {
			return nil, nil, err
		}
		ts[i] = t
	}
	return ts, values, nil
}

// FetchDataPoints performs SELECT request to fetch
// datapoints for particular field.
func (c *Client) FetchDataPoints(s *Series) (*ChunkedResponse, error) {
	iq := influx.Query{
		Command:         s.fetchQuery(c.filterTime),
		Database:        c.database,
		RetentionPolicy: c.retention,
		Chunked:         true,
		ChunkSize:       1e4,
	}
	cr, err := c.QueryAsChunk(iq)
	if err != nil {
		return nil, fmt.Errorf("query %q err: %s", iq.Command, err)
	}
	return &ChunkedResponse{cr, iq, s.Field}, nil
}

func (c *Client) fieldsByMeasurement() (map[string][]string, error) {
	q := influx.Query{
		Command:         "show field keys",
		Database:        c.database,
		RetentionPolicy: c.retention,
	}
	log.Printf("fetching fields: %s", stringify(q))
	qValues, err := c.do(q)
	if err != nil {
		return nil, fmt.Errorf("error while executing query %q: %s", q.Command, err)
	}

	var total int
	var skipped int
	const fKey = "fieldKey"
	const fType = "fieldType"
	result := make(map[string][]string, len(qValues))
	for _, qv := range qValues {
		types := qv.values[fType]
		fields := qv.values[fKey]
		values := make([]string, 0)
		for key, field := range fields {
			if types[key].(string) == "string" {
				skipped++
				continue
			}
			values = append(values, field.(string))
			total++
		}
		result[qv.name] = values
	}

	if skipped > 0 {
		log.Printf("found %d fields; skipped %d non-numeric fields", total, skipped)
	} else {
		log.Printf("found %d fields", total)
	}
	return result, nil
}

func (c *Client) getSeries() ([]*Series, error) {
	com := "show series"
	if c.filterSeries != "" {
		com = fmt.Sprintf("%s %s", com, c.filterSeries)
	}
	q := influx.Query{
		Command:         com,
		Database:        c.database,
		RetentionPolicy: c.retention,
		Chunked:         true,
		ChunkSize:       c.chunkSize,
	}

	log.Printf("fetching series: %s", stringify(q))
	cr, err := c.QueryAsChunk(q)
	if err != nil {
		return nil, fmt.Errorf("error while executing query %q: %s", q.Command, err)
	}

	const key = "key"
	var result []*Series
	for {
		resp, err := cr.NextResponse()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if resp.Error() != nil {
			return nil, fmt.Errorf("response error for query %q: %s", q.Command, resp.Error())
		}
		qValues, err := parseResult(resp.Results[0])
		if err != nil {
			return nil, err
		}
		for _, qv := range qValues {
			for _, v := range qv.values[key] {
				s := &Series{}
				if err := s.unmarshal(v.(string)); err != nil {
					return nil, err
				}
				result = append(result, s)
			}
		}
	}
	log.Printf("found %d series", len(result))
	return result, nil
}

func (c *Client) do(q influx.Query) ([]queryValues, error) {
	res, err := c.Query(q)
	if err != nil {
		return nil, fmt.Errorf("query error: %s", err)
	}
	if res.Error() != nil {
		return nil, fmt.Errorf("response error: %s", res.Error())
	}
	if len(res.Results) < 1 {
		return nil, fmt.Errorf("query returned 0 results")
	}
	return parseResult(res.Results[0])
}
