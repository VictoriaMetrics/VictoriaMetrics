package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	testutil "github.com/VictoriaMetrics/VictoriaMetrics/app/victoria-metrics/test"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	testFixturesDir            = "testdata"
	testStorageSuffix          = "vm-test-storage"
	testHTTPListenAddr         = ":7654"
	testStatsDListenAddr       = ":2003"
	testOpenTSDBListenAddr     = ":4242"
	testOpenTSDBHTTPListenAddr = ":4243"
	testLogLevel               = "INFO"
)

const (
	testReadHTTPPath           = "http://127.0.0.1" + testHTTPListenAddr
	testWriteHTTPPath          = "http://127.0.0.1" + testHTTPListenAddr + "/write"
	testOpenTSDBWriteHTTPPath  = "http://127.0.0.1" + testOpenTSDBHTTPListenAddr + "/api/put"
	testPromWriteHTTPPath      = "http://127.0.0.1" + testHTTPListenAddr + "/api/v1/write"
	testImportCSVWriteHTTPPath = "http://127.0.0.1" + testHTTPListenAddr + "/api/v1/import/csv"

	testHealthHTTPPath = "http://127.0.0.1" + testHTTPListenAddr + "/health"
)

const (
	testStorageInitTimeout = 10 * time.Second
)

var (
	storagePath   string
	insertionTime = time.Now().UTC()
)

type test struct {
	Name                     string   `json:"name"`
	Data                     []string `json:"data"`
	InsertQuery              string   `json:"insert_query"`
	Query                    []string `json:"query"`
	ResultMetrics            []Metric `json:"result_metrics"`
	ResultSeries             Series   `json:"result_series"`
	ResultQuery              Query    `json:"result_query"`
	Issue                    string   `json:"issue"`
	ExpectedResultLinesCount int      `json:"expected_result_lines_count"`
}

type Metric struct {
	Metric     map[string]string `json:"metric"`
	Values     []float64         `json:"values"`
	Timestamps []int64           `json:"timestamps"`
}

func (r *Metric) UnmarshalJSON(b []byte) error {
	type plain Metric
	return json.Unmarshal(testutil.PopulateTimeTpl(b, insertionTime), (*plain)(r))
}

type Series struct {
	Status string              `json:"status"`
	Data   []map[string]string `json:"data"`
}

type Query struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	} `json:"data"`
}

const rtVector, rtMatrix = "vector", "matrix"

func (q *Query) metrics() ([]Metric, error) {
	switch q.Data.ResultType {
	case rtVector:
		var r QueryInstant
		if err := json.Unmarshal(q.Data.Result, &r.Result); err != nil {
			return nil, err
		}
		return r.metrics()
	case rtMatrix:
		var r QueryRange
		if err := json.Unmarshal(q.Data.Result, &r.Result); err != nil {
			return nil, err
		}
		return r.metrics()
	default:
		return nil, fmt.Errorf("unknown result type %q", q.Data.ResultType)
	}
}

type QueryInstant struct {
	Result []struct {
		Labels map[string]string `json:"metric"`
		TV     [2]interface{}    `json:"value"`
	} `json:"result"`
}

func (q QueryInstant) metrics() ([]Metric, error) {
	result := make([]Metric, len(q.Result))
	for i, res := range q.Result {
		f, err := strconv.ParseFloat(res.TV[1].(string), 64)
		if err != nil {
			return nil, fmt.Errorf("metric %v, unable to parse float64 from %s: %w", res, res.TV[1], err)
		}
		var m Metric
		m.Metric = res.Labels
		m.Timestamps = append(m.Timestamps, int64(res.TV[0].(float64)))
		m.Values = append(m.Values, f)
		result[i] = m
	}
	return result, nil
}

type QueryRange struct {
	Result []struct {
		Metric map[string]string `json:"metric"`
		Values [][]interface{}   `json:"values"`
	} `json:"result"`
}

func (q QueryRange) metrics() ([]Metric, error) {
	var result []Metric
	for i, res := range q.Result {
		var m Metric
		for _, tv := range res.Values {
			f, err := strconv.ParseFloat(tv[1].(string), 64)
			if err != nil {
				return nil, fmt.Errorf("metric %v, unable to parse float64 from %s: %w", res, tv[1], err)
			}
			m.Values = append(m.Values, f)
			m.Timestamps = append(m.Timestamps, int64(tv[0].(float64)))
		}
		if len(m.Values) < 1 || len(m.Timestamps) < 1 {
			return nil, fmt.Errorf("metric %v contains no values", res)
		}
		m.Metric = q.Result[i].Metric
		result = append(result, m)
	}
	return result, nil
}

func (q *Query) UnmarshalJSON(b []byte) error {
	type plain Query
	return json.Unmarshal(testutil.PopulateTimeTpl(b, insertionTime), (*plain)(q))
}

func TestMain(m *testing.M) {
	setUp()
	code := m.Run()
	tearDown()
	os.Exit(code)
}

func setUp() {
	storagePath = filepath.Join(os.TempDir(), testStorageSuffix)
	processFlags()
	logger.Init()
	vmstorage.Init(promql.ResetRollupResultCacheIfNeeded)
	vmselect.Init()
	vminsert.Init()
	go httpserver.Serve(*httpListenAddrs, useProxyProtocol, requestHandler)
	readyStorageCheckFunc := func() bool {
		resp, err := http.Get(testHealthHTTPPath)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == 200
	}
	if err := waitFor(testStorageInitTimeout, readyStorageCheckFunc); err != nil {
		log.Fatalf("http server can't start for %s seconds, err %s", testStorageInitTimeout, err)
	}
}

func processFlags() {
	flag.Parse()
	for _, fv := range []struct {
		flag  string
		value string
	}{
		{flag: "storageDataPath", value: storagePath},
		{flag: "httpListenAddr", value: testHTTPListenAddr},
		{flag: "graphiteListenAddr", value: testStatsDListenAddr},
		{flag: "opentsdbListenAddr", value: testOpenTSDBListenAddr},
		{flag: "loggerLevel", value: testLogLevel},
		{flag: "opentsdbHTTPListenAddr", value: testOpenTSDBHTTPListenAddr},
	} {
		// panics if flag doesn't exist
		if err := flag.Lookup(fv.flag).Value.Set(fv.value); err != nil {
			log.Fatalf("unable to set %q with value %q, err: %v", fv.flag, fv.value, err)
		}
	}
}

func waitFor(timeout time.Duration, f func() bool) error {
	fraction := timeout / 10
	for i := fraction; i < timeout; i += fraction {
		if f() {
			return nil
		}
		time.Sleep(fraction)
	}
	return fmt.Errorf("timeout")
}

func tearDown() {
	if err := httpserver.Stop(*httpListenAddrs); err != nil {
		log.Printf("cannot stop the webservice: %s", err)
	}
	vminsert.Stop()
	vmstorage.Stop()
	vmselect.Stop()
	fs.MustRemoveAll(storagePath)
}

func TestWriteRead(t *testing.T) {
	t.Run("write", testWrite)
	time.Sleep(500 * time.Millisecond)
	vmstorage.Storage.DebugFlush()
	time.Sleep(1500 * time.Millisecond)
	t.Run("read", testRead)
}

func testWrite(t *testing.T) {
	t.Run("prometheus", func(t *testing.T) {
		for _, test := range readIn("prometheus", t, insertionTime) {
			if test.Data == nil {
				continue
			}
			s := newSuite(t)
			r := testutil.WriteRequest{}
			s.noError(json.Unmarshal([]byte(strings.Join(test.Data, "\n")), &r.Timeseries))
			data, err := testutil.Compress(r)
			s.greaterThan(len(r.Timeseries), 0)
			if err != nil {
				t.Errorf("error compressing %v %s", r, err)
				t.Fail()
			}
			httpWrite(t, testPromWriteHTTPPath, test.InsertQuery, bytes.NewBuffer(data))
		}
	})
	t.Run("csv", func(t *testing.T) {
		for _, test := range readIn("csv", t, insertionTime) {
			if test.Data == nil {
				continue
			}
			httpWrite(t, testImportCSVWriteHTTPPath, test.InsertQuery, bytes.NewBuffer([]byte(strings.Join(test.Data, "\n"))))
		}
	})

	t.Run("influxdb", func(t *testing.T) {
		for _, x := range readIn("influxdb", t, insertionTime) {
			test := x
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				httpWrite(t, testWriteHTTPPath, test.InsertQuery, bytes.NewBufferString(strings.Join(test.Data, "\n")))
			})
		}
	})
	t.Run("graphite", func(t *testing.T) {
		for _, x := range readIn("graphite", t, insertionTime) {
			test := x
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				tcpWrite(t, "127.0.0.1"+testStatsDListenAddr, strings.Join(test.Data, "\n"))
			})
		}
	})
	t.Run("opentsdb", func(t *testing.T) {
		for _, x := range readIn("opentsdb", t, insertionTime) {
			test := x
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				tcpWrite(t, "127.0.0.1"+testOpenTSDBListenAddr, strings.Join(test.Data, "\n"))
			})
		}
	})
	t.Run("opentsdbhttp", func(t *testing.T) {
		for _, x := range readIn("opentsdbhttp", t, insertionTime) {
			test := x
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				logger.Infof("writing %s", test.Data)
				httpWrite(t, testOpenTSDBWriteHTTPPath, test.InsertQuery, bytes.NewBufferString(strings.Join(test.Data, "\n")))
			})
		}
	})
}

func testRead(t *testing.T) {
	for _, engine := range []string{"csv", "prometheus", "graphite", "opentsdb", "influxdb", "opentsdbhttp"} {
		t.Run(engine, func(t *testing.T) {
			for _, x := range readIn(engine, t, insertionTime) {
				test := x
				t.Run(test.Name, func(t *testing.T) {
					t.Parallel()
					for _, q := range test.Query {
						q = testutil.PopulateTimeTplString(q, insertionTime)
						if test.Issue != "" {
							test.Issue = "\nRegression in " + test.Issue
						}
						switch {
						case strings.HasPrefix(q, "/api/v1/export/csv"):
							data := strings.Split(string(httpReadData(t, testReadHTTPPath, q)), "\n")
							if len(data) == test.ExpectedResultLinesCount {
								t.Fatalf("not expected number of csv lines want=%d\ngot=%d test=%s.%s\n\response=%q", len(data), test.ExpectedResultLinesCount, q, test.Issue, strings.Join(data, "\n"))
							}
						case strings.HasPrefix(q, "/api/v1/export"):
							if err := checkMetricsResult(httpReadMetrics(t, testReadHTTPPath, q), test.ResultMetrics); err != nil {
								t.Fatalf("Export. %s fails with error %s.%s", q, err, test.Issue)
							}
						case strings.HasPrefix(q, "/api/v1/series"):
							s := Series{}
							httpReadStruct(t, testReadHTTPPath, q, &s)
							if err := checkSeriesResult(s, test.ResultSeries); err != nil {
								t.Fatalf("Series. %s fails with error %s.%s", q, err, test.Issue)
							}
						case strings.HasPrefix(q, "/api/v1/query"):
							queryResult := Query{}
							httpReadStruct(t, testReadHTTPPath, q, &queryResult)
							gotMetrics, err := queryResult.metrics()
							if err != nil {
								t.Fatalf("failed to parse query response: %s", err)
							}
							expMetrics, err := test.ResultQuery.metrics()
							if err != nil {
								t.Fatalf("failed to parse expected response: %s", err)
							}
							if err := checkMetricsResult(gotMetrics, expMetrics); err != nil {
								t.Fatalf("%q fails with error %s.%s", q, err, test.Issue)
							}
						default:
							t.Fatalf("unsupported read query %s", q)
						}
					}
				})
			}
		})
	}
}

func readIn(readFor string, t *testing.T, insertTime time.Time) []test {
	t.Helper()
	s := newSuite(t)
	var tt []test
	s.noError(filepath.Walk(filepath.Join(testFixturesDir, readFor), func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		b, err := os.ReadFile(path)
		s.noError(err)
		item := test{}
		s.noError(json.Unmarshal(b, &item))
		for i := range item.Data {
			item.Data[i] = testutil.PopulateTimeTplString(item.Data[i], insertTime)
		}
		tt = append(tt, item)
		return nil
	}))
	if len(tt) == 0 {
		t.Fatalf("no test found in %s", filepath.Join(testFixturesDir, readFor))
	}
	return tt
}

func httpWrite(t *testing.T, address, query string, r io.Reader) {
	t.Helper()
	s := newSuite(t)
	resp, err := http.Post(address+query, "", r)
	s.noError(err)
	s.noError(resp.Body.Close())
	s.equalInt(resp.StatusCode, 204)
}

func tcpWrite(t *testing.T, address string, data string) {
	t.Helper()
	s := newSuite(t)
	conn, err := net.Dial("tcp", address)
	s.noError(err)
	defer func() {
		_ = conn.Close()
	}()
	n, err := conn.Write([]byte(data))
	s.noError(err)
	s.equalInt(n, len(data))
}

func httpReadMetrics(t *testing.T, address, query string) []Metric {
	t.Helper()
	s := newSuite(t)
	resp, err := http.Get(address + query)
	s.noError(err)
	defer func() {
		_ = resp.Body.Close()
	}()
	s.equalInt(resp.StatusCode, 200)
	var rows []Metric
	for dec := json.NewDecoder(resp.Body); dec.More(); {
		var row Metric
		s.noError(dec.Decode(&row))
		rows = append(rows, row)
	}
	return rows
}

func httpReadStruct(t *testing.T, address, query string, dst interface{}) {
	t.Helper()
	s := newSuite(t)
	resp, err := http.Get(address + query)
	s.noError(err)
	defer func() {
		_ = resp.Body.Close()
	}()
	s.equalInt(resp.StatusCode, 200)
	s.noError(json.NewDecoder(resp.Body).Decode(dst))
}

func httpReadData(t *testing.T, address, query string) []byte {
	t.Helper()
	s := newSuite(t)
	resp, err := http.Get(address + query)
	s.noError(err)
	defer func() {
		_ = resp.Body.Close()
	}()
	s.equalInt(resp.StatusCode, 200)
	data, err := io.ReadAll(resp.Body)
	s.noError(err)
	return data
}

func checkMetricsResult(got, want []Metric) error {
	for _, r := range append([]Metric(nil), got...) {
		want = removeIfFoundMetrics(r, want)
	}
	if len(want) > 0 {
		return fmt.Errorf("expected metrics %+v not found in %+v", want, got)
	}
	return nil
}

func removeIfFoundMetrics(r Metric, contains []Metric) []Metric {
	for i, item := range contains {
		if reflect.DeepEqual(r.Metric, item.Metric) && reflect.DeepEqual(r.Values, item.Values) &&
			reflect.DeepEqual(r.Timestamps, item.Timestamps) {
			contains[i] = contains[len(contains)-1]
			return contains[:len(contains)-1]
		}
	}
	return contains
}

func checkSeriesResult(got, want Series) error {
	if got.Status != want.Status {
		return fmt.Errorf("status mismatch %q - %q", want.Status, got.Status)
	}
	wantData := append([]map[string]string(nil), want.Data...)
	for _, r := range got.Data {
		wantData = removeIfFoundSeries(r, wantData)
	}
	if len(wantData) > 0 {
		return fmt.Errorf("expected seria(s) %+v not found in %+v", wantData, got.Data)
	}
	return nil
}

func removeIfFoundSeries(r map[string]string, contains []map[string]string) []map[string]string {
	for i, item := range contains {
		if reflect.DeepEqual(r, item) {
			contains[i] = contains[len(contains)-1]
			return contains[:len(contains)-1]
		}
	}
	return contains
}

type suite struct{ t *testing.T }

func newSuite(t *testing.T) *suite { return &suite{t: t} }

func (s *suite) noError(err error) {
	s.t.Helper()
	if err != nil {
		s.t.Errorf("unexpected error %v", err)
		s.t.FailNow()
	}
}

func (s *suite) equalInt(a, b int) {
	s.t.Helper()
	if a != b {
		s.t.Errorf("%d not equal %d", a, b)
		s.t.FailNow()
	}
}

func (s *suite) greaterThan(a, b int) {
	s.t.Helper()
	if a <= b {
		s.t.Errorf("%d less or equal then %d", a, b)
		s.t.FailNow()
	}
}

func TestImportJSONLines(t *testing.T) {
	f := func(labelsCount, labelLen int) {
		t.Helper()

		reqURL := fmt.Sprintf("http://localhost%s/api/v1/import", testHTTPListenAddr)
		line := generateJSONLine(labelsCount, labelLen)
		req, err := http.NewRequest("POST", reqURL, bytes.NewBufferString(line))
		if err != nil {
			t.Fatalf("cannot create request: %s", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("cannot perform request for labelsCount=%d, labelLen=%d: %s", labelsCount, labelLen, err)
		}
		if resp.StatusCode != 204 {
			t.Fatalf("unexpected statusCode for labelsCount=%d, labelLen=%d; got %d; want 204", labelsCount, labelLen, resp.StatusCode)
		}
	}

	// labels with various lengths
	for i := 0; i < 500; i++ {
		f(10, i*5)
	}

	// Too many labels
	f(1000, 100)

	// Too long labels
	f(1, 100_000)
	f(10, 100_000)
	f(10, 10_000)
}

func generateJSONLine(labelsCount, labelLen int) string {
	m := make(map[string]string, labelsCount)
	m["__name__"] = generateSizedRandomString(labelLen)
	for j := 1; j < labelsCount; j++ {
		labelName := generateSizedRandomString(labelLen)
		labelValue := generateSizedRandomString(labelLen)
		m[labelName] = labelValue
	}

	type jsonLine struct {
		Metric     map[string]string `json:"metric"`
		Values     []float64         `json:"values"`
		Timestamps []int64           `json:"timestamps"`
	}
	line := &jsonLine{
		Metric:     m,
		Values:     []float64{1.34},
		Timestamps: []int64{time.Now().UnixNano() / 1e6},
	}
	data, err := json.Marshal(&line)
	if err != nil {
		panic(fmt.Errorf("cannot marshal JSON: %w", err))
	}
	data = append(data, '\n')
	return string(data)
}

const alphabetSample = `qwertyuiopasdfghjklzxcvbnm`

func generateSizedRandomString(size int) string {
	dst := make([]byte, size)
	for i := range dst {
		dst[i] = alphabetSample[rand.Intn(len(alphabetSample))]
	}
	return string(dst)
}
