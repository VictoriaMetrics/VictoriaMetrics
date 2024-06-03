package opentsdb

import (
	"encoding/json"
	"fmt"
	"io"
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
	// The actual ranges will attempt to query (as offsets from now)
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
	MsecsTime  bool

	c *http.Client
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
	Transport  *http.Transport
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
	Metric string            `json:"metric"`
	Tags   map[string]string `json:"tags"`
	//tsuid  string
}

// OtsdbMetric is a single series in OpenTSDB's returned format
type OtsdbMetric struct {
	Metric        string
	Tags          map[string]string
	AggregateTags []string
	Dps           map[int64]float64
}

// Metric holds the time series data in VictoriaMetrics format
type Metric struct {
	Metric     string
	Tags       map[string]string
	Timestamps []int64
	Values     []float64
}

// FindMetrics discovers all metrics that OpenTSDB knows about (given a filter)
// e.g. /api/suggest?type=metrics&q=system&max=100000
func (c Client) FindMetrics(q string) ([]string, error) {

	resp, err := c.c.Get(q)
	if err != nil {
		return nil, fmt.Errorf("failed to send GET request to %q: %s", q, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Bad return from OpenTSDB: %q: %v", resp.StatusCode, resp)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
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
	resp, err := c.c.Get(q)
	if err != nil {
		return nil, fmt.Errorf("failed to set GET request to %q: %s", q, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Bad return from OpenTSDB: %q: %v", resp.StatusCode, resp)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
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
// e.g. /api/query?start=1&end=200&m=sum:1m-avg-none:system.load5{host=host1}
func (c Client) GetData(series Meta, rt RetentionMeta, start int64, end int64, mSecs bool) (Metric, error) {
	/*
		First, build our tag string.
		It's literally just key=value,key=value,...
	*/
	tagStr := ""
	for k, v := range series.Tags {
		tagStr += fmt.Sprintf("%s=%s,", k, v)
	}
	// obviously we don't want trailing commas...
	tagStr = strings.Trim(tagStr, ",")

	/*
		The aggregation policy should already be somewhat formatted:
		FirstOrder (e.g. sum/avg/max/etc.)
		SecondOrder (e.g. sum/avg/max/etc.)
		AggTime	(e.g. 1m/10m/1d/etc.)
		This will build into m=<FirstOrder>:<AggTime>-<SecondOrder>-none:
		Or an example: m=sum:1m-avg-none
	*/
	aggPol := fmt.Sprintf("%s:%s-%s-none", rt.FirstOrder, rt.AggTime, rt.SecondOrder)

	/*
		Our actual query string:
		Start and End are just timestamps
		We then add the aggregation policy, the metric, and the tag set
	*/
	queryStr := fmt.Sprintf("start=%v&end=%v&m=%s:%s{%s}", start, end, aggPol,
		series.Metric, tagStr)

	q := fmt.Sprintf("%s/api/query?%s", c.Addr, queryStr)
	resp, err := c.c.Get(q)
	if err != nil {
		return Metric{}, fmt.Errorf("failed to send GET request to %q: %s", q, err)
	}
	/*
		There are three potential failures here, none of which should kill the entire
		migration run:
		1. bad response code
		2. failure to read response body
		3. bad format of response body
	*/
	if resp.StatusCode != 200 {
		log.Printf("bad response code from OpenTSDB query %v for %q...skipping", resp.StatusCode, q)
		return Metric{}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("couldn't read response body from OpenTSDB query...skipping")
		return Metric{}, nil
	}
	var output []OtsdbMetric
	err = json.Unmarshal(body, &output)
	if err != nil {
		log.Printf("couldn't marshall response body from OpenTSDB query (%s)...skipping", body)
		return Metric{}, nil
	}
	/*
		We expect results to look like:
		[
		  {
			"metric": "zfs_filesystem.available",
			"tags": {
			  "rack": "6",
			  "replica": "1",
			  "host": "c7-bfyii-115",
			  "pool": "dattoarray",
			  "row": "c",
			  "dc": "us-west-3",
			  "group": "legonode"
			},
			"aggregateTags": [],
			"dps": {
			  "1626019200": 32490602877610.668,
			  "1626033600": 32486439014058.668
			}
		  }
		]
		There are two things that could be bad here:
			1. There are no actual stats returned (an empty array -> [])
			2. There are aggregate tags in the results
		An empty array doesn't cast to a OtsdbMetric struct well, and there's no reason to try, so we should just skip it
		Because we're trying to migrate data without transformations, seeing aggregate tags could mean
		we're dropping series on the floor.

		In all "bad" cases, we don't end the migration, we just don't process that particular message
	*/
	if len(output) < 1 {
		// no results returned...return an empty object without error
		return Metric{}, nil
	}
	if len(output) > 1 {
		// multiple series returned for a single query. We can't process this right, so...
		return Metric{}, nil
	}
	if len(output[0].AggregateTags) > 0 {
		// This failure means we've suppressed potential series somehow...
		return Metric{}, nil
	}
	data := Metric{}
	data.Metric = output[0].Metric
	data.Tags = output[0].Tags
	/*
		We evaluate data for correctness before formatting the actual values
		to skip a little bit of time if the series has invalid formatting
	*/
	data, err = modifyData(data, c.Normalize)
	if err != nil {
		return Metric{}, nil
	}

	/*
		Convert data from OpenTSDB's output format ([[ts,val],[ts,val]...])
		to VictoriaMetrics format: {"timestamps": [ts,ts,ts...], "values": [val,val,val...]}
		The nasty part here is that because an object in each array
		can be a float64, we have to initially cast _all_ objects that way
		then convert the timestamp back to something reasonable.
	*/
	for ts, val := range output[0].Dps {
		if !mSecs {
			data.Timestamps = append(data.Timestamps, ts*1000)
		} else {
			data.Timestamps = append(data.Timestamps, ts)
		}
		data.Values = append(data.Values, val)
	}
	return data, nil
}

// NewClient creates and returns OpenTSDB client
// configured with passed Config
func NewClient(cfg Config) (*Client, error) {
	var retentions []Retention
	offsetPrint := int64(time.Now().Unix())
	// convert a number of days to seconds
	offsetSecs := cfg.Offset * 24 * 60 * 60
	if cfg.MsecsTime {
		// 1000000 == Nanoseconds -> Milliseconds difference
		offsetPrint = int64(time.Now().UnixNano() / 1000000)
		// also bump offsetSecs to milliseconds
		offsetSecs = offsetSecs * 1000
	}
	if cfg.HardTS > 0 {
		/*
			HardTS is a specific timestamp we'll be starting at.
			Just present that if it is defined
		*/
		offsetPrint = cfg.HardTS
	} else if offsetSecs > 0 {
		/*
			Our "offset" is the number of days (in seconds) we should step
			back before starting to scan for data
		*/
		offsetPrint = offsetPrint - offsetSecs
	}
	log.Printf("Will collect data starting at TS %v", offsetPrint)
	for _, r := range cfg.Retentions {
		ret, err := convertRetention(r, offsetSecs, cfg.MsecsTime)
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
		MsecsTime:  cfg.MsecsTime,
		c:          &http.Client{Transport: cfg.Transport},
	}
	return client, nil
}
