package opentsdb

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type Retention struct {
	/*
	OpenTSDB has two levels of aggregation,
	First, we aggregate any un-mentioned tags into the last result
	Second, we aggregate into buckets over time
	To simulate this with config, we have
	FirstOrder (e.g. sum/avg/max/etc.)
	SecondOrder (e.g. sum/avg/max/etc.)
	AggTime	(e.g. 1m/10m/1d/etc.)
	This will build into m=<FirstOrder>:<AggTime>-<SecondOrder>-none:
	Or an example: m=sum:1m-avg-none
	*/
	FirstOrder	string
	SecondOrder	string
	AggTime	string
	// The actual ranges will will attempt to query (as offsets from now)
	QueryRanges	[]TimeRange
}

type Client struct {
	Addr	string
	// The meta query limit for series returned
	Limit	int
	Retentions	[]Retention
}

// Config contains fields required
// for Client configuration
type Config struct {
	Addr      string
	Limit	int
	Retentions	[]string
	Filters	[]string
}

// data about time ranges to query
type TimeRange struct {
	Start	int64
	End	int64
}

// A meta object about a metric
// only contain the tags/etc. and no data
type Meta struct {
	tsuid	string
	Metric	string
	Tags	map[string]string
}

// Metric holds the time series data
type Metric struct {
	Metric	string
	Tags	map[string]string
	AggregateTags	[]string
	Dps	map[string]float64
}

// Find all metrics that OpenTSDB knows about with a filter
// e.g. /api/suggest?type=metrics&q=system
func (c Client) FindMetrics(filter string) ([]string, error) {
	q := &strings.Builder{}
	fmt.Fprintf(q, "%q/api/suggest?type=metrics&q=%q", c.Addr, filter)
	resp, err := http.Get(q.String())
	if err != nil {
		return nil, fmt.Errorf("Could not properly make request to %s: %s", c.Addr, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Could not retrieve metric data from %s: %s", c.Addr, err)
	}
	var metriclist []string
	err = json.Unmarshal(body, &metriclist)
	if err != nil {
		return nil, fmt.Errorf("Invalid metric data from %s: %s", c.Addr, err)
	}
	return metriclist, nil
}

// Find all series associated with a metric
// e.g. /api/search/lookup?m=system.load5&limit=1000000
func (c Client) FindSeries(metric string) ([]Meta, error) {
	q := &strings.Builder{}
	fmt.Fprintf(q, "%q/api/search/lookup?m=%q&limit=%q", c.Addr, metric, c.Limit)
	resp, err := http.Get(q.String())
	if err != nil {
		return nil, fmt.Errorf("Could not properly make request to %s: %s", c.Addr, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Could not retrieve series data from %s: %s", c.Addr, err)
	}
	var serieslist []Meta
	err = json.Unmarshal(body, &serieslist)
	if err != nil {
		return nil, fmt.Errorf("Invalid series data from %s: %s", c.Addr, err)
	}
	return serieslist, nil
}

// Get data for series
func (c Client) GetData(series string, start int, end int) (Metric, error) {
	q := &strings.Builder{}
	fmt.Fprintf(q, "%q/api/query?start=%q&end=%q&m=%q:%q-%q-none:%q",
					c.Addr, start, end, c.FirstOrder, c.AggTime, c.SecondOrder,
					series)
	resp, err := http.Get(q.String())
	if err != nil {
		return nil, fmt.Errorf("Could not properly make request to %s: %s", c.Addr, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Could not retrieve series data from %s: %s", c.Addr, err)
	}
	var data Metric
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("Invalid series data from %s: %s", c.Addr, err)
	}
	return data, nil
}

// NewClient creates and returns influx client
// configured with passed Config
func NewClient(cfg Config) (*Client, error) {
	if _, _, err := hc.Ping(time.Second); err != nil {
		return nil, fmt.Errorf("ping failed: %s", err)
	}
	var retentions []Retention
	for r := range cfg.Retentions {
		first, aggTime, second, tr := ConverRetention(r)
		append(retentions, Retention{FirstOrder: first, SecondOrder: second,
			AggTime: aggTime, TimeRanges: tr})
	}
	client := &Client{
		Addr:	cfg.Addr,
		Retentions:	retentions,
		Limit:	cfg.Limit,
		Filters: cfg.Filters,
	}
	return client, nil
}
/*
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

	series, err := c.getSeries()
	if err != nil {
		return nil, fmt.Errorf("failed to get series: %s", err)
	}

	var iSeries []*Series
	for _, s := range series {
		fields, ok := mFields[s.Measurement]
		if !ok {
			return nil, fmt.Errorf("can't find field keys for measurement %q", s.Measurement)
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
		return nil, fmt.Errorf("query %q err: %s", q.Command, err)
	}
	if len(res.Results) < 1 {
		return nil, fmt.Errorf("exploration query %q returned 0 results", q.Command)
	}
	return parseResult(res.Results[0])
}*/
