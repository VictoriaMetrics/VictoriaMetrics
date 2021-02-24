package opentsdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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
	FirstOrder  string
	SecondOrder string
	AggTime     string
	// The actual ranges will will attempt to query (as offsets from now)
	QueryRanges []TimeRange
}

type Client struct {
	Addr string
	// The meta query limit for series returned
	Limit      int
	Retentions []Retention
	Filters    []string
	Normalize  bool
}

// Config contains fields required
// for Client configuration
type Config struct {
	Addr       string
	Limit      int
	Retentions []string
	Filters    []string
	Normalize  bool
}

// data about time ranges to query
type TimeRange struct {
	Start int64
	End   int64
}

type MetaResults struct {
	Type         string      `json:"type"`
	metric       string      `json:"metric"`
	tags         interface{} `json:"tags"`
	limit        int         `json:"limit"`
	time         int         `json:"time"`
	Results      []Meta      `json:"results"`
	startIndex   int         `json:"startIndex"`
	totalResults int         `json:"totalResults"`
}

// A meta object about a metric
// only contain the tags/etc. and no data
type Meta struct {
	tsuid  string            `json:"tsuid"`
	Metric string            `json:"metric"`
	Tags   map[string]string `json:"tags"`
}

// Metric holds the time series data
type Metric struct {
	Metric     string
	Tags       map[string]string
	Timestamps []int64
	Values     []float64
}

type ExpressionOutput struct {
	Outputs []QoObj     `json:"outputs"`
	Query   interface{} `json:"query"`
}

type QoObj struct {
	Id      string      `json:"id"`
	Alias   string      `json:"alias"`
	Dps     [][]float64 `json:"dps"`
	dpsMeta interface{} `json:"dpsMeta"`
	meta    interface{} `json:"meta"`
}

/*
All of the following structs are to build a OpenTSDB expression object
*/
type Expression struct {
	Time    TimeObj     `json:"time"`
	Filters []FilterObj `json:"filters"`
	Metrics []MetricObj `json:"metrics"`
	// this just needs to be an empty object, so the value doesn't matter
	Expressions []int       `json:"expressions"`
	Outputs     []OutputObj `json:"outputs"`
}

type TimeObj struct {
	Start       int64  `json:"start"`
	End         int64  `json:"end"`
	Aggregator  string `json:"aggregator"`
	Downsampler DSObj  `json:"downsampler"`
}

type DSObj struct {
	Interval   string  `json:"interval"`
	Aggregator string  `json:"aggregator"`
	FillPolicy FillObj `json:"fillPolicy"`
}

type FillObj struct {
	// we'll always hard-code to NaN here, so we don't need value
	Policy string `json:"policy"`
}

type FilterObj struct {
	Tags []TagObj `json:"tags"`
	Id   string   `json:"id"`
}

type TagObj struct {
	Type    string `json:"type"`
	Tagk    string `json:"tagk"`
	Filter  string `json:"filter"`
	GroupBy bool   `json:"groupBy"`
}

type MetricObj struct {
	Id         string  `json:"id"`
	Metric     string  `json:"metric"`
	Filter     string  `json:"filter"`
	FillPolicy FillObj `json:"fillPolicy"`
}

type OutputObj struct {
	Id    string `json:"id"`
	Alias string `json:"alias"`
}

/* End expression object structs */

// Find all metrics that OpenTSDB knows about with a filter
// e.g. /api/suggest?type=metrics&q=system
func (c Client) FindMetrics(filter string) ([]string, error) {
	q := fmt.Sprintf("%s/api/suggest?type=metrics&q=%s&max=%d", c.Addr, filter, c.Limit)
	log.Println(q)
	resp, err := http.Get(q)
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
	q := fmt.Sprintf("%s/api/search/lookup?m=%s&limit=%d", c.Addr, metric, c.Limit)
	resp, err := http.Get(q)
	if err != nil {
		return nil, fmt.Errorf("Could not properly make request to %s: %s", c.Addr, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Could not retrieve series data from %s: %s", c.Addr, err)
	}
	var results MetaResults
	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, fmt.Errorf("Invalid series data from %s: %s", c.Addr, err)
	}
	return results.Results, nil
}

// Get data for series
func (c Client) GetData(series Meta, rt Retention, start int64, end int64) (Metric, error) {
	expr := Expression{}
	expr.Outputs = append(expr.Outputs, OutputObj{Id: "a", Alias: "query"})
	expr.Metrics = append(expr.Metrics, MetricObj{Id: "a", Metric: series.Metric,
		Filter: "f1", FillPolicy: FillObj{Policy: "nan"}})
	expr.Time = TimeObj{Start: start, End: end, Aggregator: rt.FirstOrder,
		Downsampler: DSObj{Interval: rt.AggTime,
			Aggregator: rt.SecondOrder,
			FillPolicy: FillObj{Policy: "nan"}}}
	var TagList []TagObj
	for k, v := range series.Tags {
		TagList = append(TagList, TagObj{Type: "literal_or", Tagk: k,
			Filter: v, GroupBy: true})
	}
	expr.Filters = append(expr.Filters, FilterObj{Id: "f1", Tags: TagList})
	// "expressions" is required in the query object or we get a 5xx, so force it to exist
	expr.Expressions = make([]int, 0)
	inputData, err := json.Marshal(expr)
	// log.Println("Query: ", string(inputData))
	q := fmt.Sprintf("%s/api/query/exp", c.Addr)
	resp, err := http.Post(q, "application/json", bytes.NewBuffer(inputData))
	if err != nil {
		return Metric{}, fmt.Errorf("Could not properly make request to %s: %s", c.Addr, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Metric{}, fmt.Errorf("Could not retrieve series data from %s: %s", c.Addr, err)
	}
	var output ExpressionOutput
	err = json.Unmarshal(body, &output)
	if err != nil {
		// log.Println("Incoming data: ", string(body))
		return Metric{}, fmt.Errorf("Invalid series data from %s: %s", c.Addr, err)
	}
	if len(output.Outputs) < 1 {
		// log.Println("Incoming data: ", string(body))
		return Metric{}, nil
	}
	// log.Println("De-serialized: ", output)
	data := Metric{}
	data.Metric = series.Metric
	data.Tags = series.Tags
	for _, tsobj := range output.Outputs[0].Dps {
		data.Timestamps = append(data.Timestamps, int64(tsobj[0]))
		data.Values = append(data.Values, tsobj[1])
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
		Addr:       strings.Trim(cfg.Addr, "/"),
		Retentions: retentions,
		Limit:      cfg.Limit,
		Filters:    cfg.Filters,
		Normalize:  cfg.Normalize,
	}
	return client, nil
}
