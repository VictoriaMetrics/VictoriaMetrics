package opentsdb

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
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
	Filters []string
	Normalize	bool
}

// Config contains fields required
// for Client configuration
type Config struct {
	Addr      string
	Limit	int
	Retentions	[]string
	Filters	[]string
	Normalize	bool
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
func (c Client) GetData(series string, rt Retention, start int64, end int64) (Metric, error) {
	q := &strings.Builder{}
	fmt.Fprintf(q, "%q/api/query?start=%q&end=%q&m=%q:%q-%q-none:%q",
					c.Addr, start, end, rt.FirstOrder, rt.AggTime,
					rt.SecondOrder,
					series)
	resp, err := http.Get(q.String())
	if err != nil {
		return Metric{}, fmt.Errorf("Could not properly make request to %s: %s", c.Addr, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Metric{}, fmt.Errorf("Could not retrieve series data from %s: %s", c.Addr, err)
	}
	var data Metric
	err = json.Unmarshal(body, &data)
	if err != nil {
		return Metric{}, fmt.Errorf("Invalid series data from %s: %s", c.Addr, err)
	}
	data, err = modifyData(data, c.Normalize)
	if err != nil {
		return Metric{}, fmt.Errorf("Invalid series data from %s: %s", c.Addr, err)
	}
	return data, nil
}

// NewClient creates and returns influx client
// configured with passed Config
func NewClient(cfg Config) (*Client, error) {
	/*
	if _, _, err := hc.Ping(time.Second); err != nil {
		return nil, fmt.Errorf("ping failed: %s", err)
	}
	*/
	var retentions []Retention
	for _, r := range cfg.Retentions {
		first, aggTime, second, tr := convertRetention(r)
		retentions = append(retentions, Retention{FirstOrder: first, SecondOrder: second,
							AggTime: aggTime, QueryRanges: tr})
	}
	client := &Client{
		Addr:	cfg.Addr,
		Retentions:	retentions,
		Limit:	cfg.Limit,
		Filters: cfg.Filters,
		Normalize:	cfg.Normalize,
	}
	return client, nil
}
