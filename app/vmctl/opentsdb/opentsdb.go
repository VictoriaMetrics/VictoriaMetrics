package opentsdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

// Retention objects contain meta data about what to query for our run
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

// RetentionMeta objects exist to pass smaller subsets (only one retention range) of a full Retention object around
type RetentionMeta struct {
	FirstOrder  string
	SecondOrder string
	AggTime     string
}

// Client object holds general config about how queries should be performed
type Client struct {
	Addr string
	// The meta query limit for series returned
	Limit      int
	Retentions []Retention
	Filters    []string
	Normalize  bool
	HardTS     int64
}

// Config contains fields required
// for Client configuration
type Config struct {
	Addr       string
	Limit      int
	Offset     int64
	HardTS     int64
	Retentions []string
	Filters    []string
	Normalize  bool
	MsecsTime  bool
}

// TimeRange contains data about time ranges to query
type TimeRange struct {
	Start int64
	End   int64
}

// MetaResults contains return data from search series lookup queries
type MetaResults struct {
	Type    string `json:"type"`
	Results []Meta `json:"results"`
	//metric       string
	//tags         interface{}
	//limit        int
	//time         int
	//startIndex   int
	//totalResults int
}

// Meta A meta object about a metric
// only contain the tags/etc. and no data
type Meta struct {
	//tsuid  string
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

// ExpressionOutput contains results from actual data queries
type ExpressionOutput struct {
	Outputs []qoObj     `json:"outputs"`
	Query   interface{} `json:"query"`
}

// QoObj contains actual timeseries data from the returned data query
type qoObj struct {
	ID    string      `json:"id"`
	Alias string      `json:"alias"`
	Dps   [][]float64 `json:"dps"`
	//dpsMeta interface{}
	//meta    interface{}
}

// Expression objects format our data queries
/*
All of the following structs are to build a OpenTSDB expression object
*/
type Expression struct {
	Time    timeObj     `json:"time"`
	Filters []filterObj `json:"filters"`
	Metrics []metricObj `json:"metrics"`
	// this just needs to be an empty object, so the value doesn't matter
	Expressions []int       `json:"expressions"`
	Outputs     []outputObj `json:"outputs"`
}

type timeObj struct {
	Start       int64  `json:"start"`
	End         int64  `json:"end"`
	Aggregator  string `json:"aggregator"`
	Downsampler dSObj  `json:"downsampler"`
}

type dSObj struct {
	Interval   string  `json:"interval"`
	Aggregator string  `json:"aggregator"`
	FillPolicy fillObj `json:"fillPolicy"`
}

type fillObj struct {
	// we'll always hard-code to NaN here, so we don't need value
	Policy string `json:"policy"`
}

type filterObj struct {
	Tags []tagObj `json:"tags"`
	ID   string   `json:"id"`
}

type tagObj struct {
	Type    string `json:"type"`
	Tagk    string `json:"tagk"`
	Filter  string `json:"filter"`
	GroupBy bool   `json:"groupBy"`
}

type metricObj struct {
	ID         string  `json:"id"`
	Metric     string  `json:"metric"`
	Filter     string  `json:"filter"`
	FillPolicy fillObj `json:"fillPolicy"`
}

type outputObj struct {
	ID    string `json:"id"`
	Alias string `json:"alias"`
}

/* End expression object structs */

var (
	exprOutput     = outputObj{ID: "a", Alias: "query"}
	exprFillPolicy = fillObj{Policy: "nan"}
)

// FindMetrics discovers all metrics that OpenTSDB knows about (given a filter)
// e.g. /api/suggest?type=metrics&q=system&max=100000
func (c Client) FindMetrics(q string) ([]string, error) {
	resp, err := http.Get(q)
	if err != nil {
		return nil, fmt.Errorf("failed to send GET request to %q: %s", q, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Bad return from OpenTSDB: %q: %v", resp.StatusCode, resp)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve metric data from %q: %s", q, err)
	}
	var metriclist []string
	err = json.Unmarshal(body, &metriclist)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %q: %s", q, err)
	}
	return metriclist, nil
}

// FindSeries discovers all series associated with a metric
// e.g. /api/search/lookup?m=system.load5&limit=1000000
func (c Client) FindSeries(metric string) ([]Meta, error) {
	q := fmt.Sprintf("%s/api/search/lookup?m=%s&limit=%d", c.Addr, metric, c.Limit)
	resp, err := http.Get(q)
	if err != nil {
		return nil, fmt.Errorf("failed to set GET request to %q: %s", q, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Bad return from OpenTSDB: %q: %v", resp.StatusCode, resp)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve series data from %q: %s", q, err)
	}
	var results MetaResults
	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %q: %s", q, err)
	}
	return results.Results, nil
}

// GetData actually retrieves data for a series at a specified time range
func (c Client) GetData(series Meta, rt RetentionMeta, start int64, end int64) (Metric, error) {
	/*
		Here we build the actual exp query we'll send to OpenTSDB

		This is comprised of a number of different settings. We hard-code
		a few to simplify the JSON object creation.
		There are examples queries available, so not too much detail here...
	*/
	expr := Expression{}
	expr.Outputs = []outputObj{exprOutput}
	expr.Metrics = append(expr.Metrics, metricObj{ID: "a", Metric: series.Metric,
		Filter: "f1", FillPolicy: exprFillPolicy})
	expr.Time = timeObj{Start: start, End: end, Aggregator: rt.FirstOrder,
		Downsampler: dSObj{Interval: rt.AggTime,
			Aggregator: rt.SecondOrder,
			FillPolicy: exprFillPolicy}}
	var TagList []tagObj
	for k, v := range series.Tags {
		/*
			every tag should be a literal_or because that's the closest to a full "==" that
			this endpoint allows for
		*/
		TagList = append(TagList, tagObj{Type: "literal_or", Tagk: k,
			Filter: v, GroupBy: true})
	}
	expr.Filters = append(expr.Filters, filterObj{ID: "f1", Tags: TagList})
	// "expressions" is required in the query object or we get a 5xx, so force it to exist
	expr.Expressions = make([]int, 0)
	inputData, err := json.Marshal(expr)
	if err != nil {
		return Metric{}, fmt.Errorf("failed to marshal query JSON %s", err)
	}

	q := fmt.Sprintf("%s/api/query/exp", c.Addr)
	resp, err := http.Post(q, "application/json", bytes.NewBuffer(inputData))
	if err != nil {
		return Metric{}, fmt.Errorf("failed to send GET request to %q: %s", q, err)
	}
	if resp.StatusCode != 200 {
		return Metric{}, fmt.Errorf("Bad return from OpenTSDB: %q: %v", resp.StatusCode, resp)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Metric{}, fmt.Errorf("could not retrieve series data from %q: %s", q, err)
	}
	var output ExpressionOutput
	err = json.Unmarshal(body, &output)
	if err != nil {
		return Metric{}, fmt.Errorf("failed to unmarshal response from %q: %s", q, err)
	}
	if len(output.Outputs) < 1 {
		// no results returned...return an empty object without error
		return Metric{}, nil
	}
	data := Metric{}
	data.Metric = series.Metric
	data.Tags = series.Tags
	/*
		We evaluate data for correctness before formatting the actual values
		to skip a little bit of time if the series has invalid formatting

		First step is to enforce Prometheus' data model
	*/
	data, err = modifyData(data, c.Normalize)
	if err != nil {
		return Metric{}, fmt.Errorf("invalid series data from %q: %s", q, err)
	}
	/*
		Convert data from OpenTSDB's output format ([[ts,val],[ts,val]...])
		to VictoriaMetrics format: {"timestamps": [ts,ts,ts...], "values": [val,val,val...]}
		The nasty part here is that because an object in each array
		can be a float64, we have to initially cast _all_ objects that way
		then convert the timestamp back to something reasonable.
	*/
	for _, tsobj := range output.Outputs[0].Dps {
		data.Timestamps = append(data.Timestamps, int64(tsobj[0]))
		data.Values = append(data.Values, tsobj[1])
	}
	return data, nil
}

// NewClient creates and returns OpenTSDB client
// configured with passed Config
func NewClient(cfg Config) (*Client, error) {
	var retentions []Retention
	offsetPrint := int64(time.Now().Unix())
	if cfg.MsecsTime {
		// 1000000 == Nanoseconds -> Milliseconds difference
		offsetPrint = int64(time.Now().UnixNano() / 1000000)
	}
	if cfg.HardTS > 0 {
		/*
			HardTS is a specific timestamp we'll be starting at.
			Just present that if it is defined
		*/
		offsetPrint = cfg.HardTS
	} else if cfg.Offset > 0 {
		/*
			Our "offset" is the number of days we should step
			back before starting to scan for data
		*/
		if cfg.MsecsTime {
			offsetPrint = offsetPrint - (cfg.Offset * 24 * 60 * 60 * 1000)
		} else {
			offsetPrint = offsetPrint - (cfg.Offset * 24 * 60 * 60)
		}
	}
	log.Println(fmt.Sprintf("Will collect data starting at TS %v", offsetPrint))
	for _, r := range cfg.Retentions {
		ret, err := convertRetention(r, cfg.Offset, cfg.MsecsTime)
		if err != nil {
			return &Client{}, fmt.Errorf("Couldn't parse retention %q :: %v", r, err)
		}
		retentions = append(retentions, ret)
	}
	client := &Client{
		Addr:       strings.Trim(cfg.Addr, "/"),
		Retentions: retentions,
		Limit:      cfg.Limit,
		Filters:    cfg.Filters,
		Normalize:  cfg.Normalize,
		HardTS:     cfg.HardTS,
	}
	return client, nil
}
