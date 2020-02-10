package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	testutil "github.com/VictoriaMetrics/VictoriaMetrics/app/victoria-metrics/test"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
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
	testReadHTTPPath          = "http://127.0.0.1" + testHTTPListenAddr
	testWriteHTTPPath         = "http://127.0.0.1" + testHTTPListenAddr + "/write"
	testOpenTSDBWriteHTTPPath = "http://127.0.0.1" + testOpenTSDBHTTPListenAddr + "/api/put"
	testPromWriteHTTPPath     = "http://127.0.0.1" + testHTTPListenAddr + "/api/v1/write"
	testHealthHTTPPath        = "http://127.0.0.1" + testHTTPListenAddr + "/health"
)

const (
	testStorageInitTimeout = 10 * time.Second
)

var (
	storagePath   string
	insertionTime = time.Now().UTC()
)

type test struct {
	Name             string     `json:"name"`
	Data             []string   `json:"data"`
	Query            []string   `json:"query"`
	ResultMetrics    []Metric   `json:"result_metrics"`
	ResultSeries     Series     `json:"result_series"`
	ResultQuery      Query      `json:"result_query"`
	ResultQueryRange QueryRange `json:"result_query_range"`
	Issue            string     `json:"issue"`
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
	Status string    `json:"status"`
	Data   QueryData `json:"data"`
}
type QueryData struct {
	ResultType string            `json:"resultType"`
	Result     []QueryDataResult `json:"result"`
}

type QueryDataResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

func (r *QueryDataResult) UnmarshalJSON(b []byte) error {
	type plain QueryDataResult
	return json.Unmarshal(testutil.PopulateTimeTpl(b, insertionTime), (*plain)(r))
}

type QueryRange struct {
	Status string         `json:"status"`
	Data   QueryRangeData `json:"data"`
}
type QueryRangeData struct {
	ResultType string                 `json:"resultType"`
	Result     []QueryRangeDataResult `json:"result"`
}

type QueryRangeDataResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"`
}

func (r *QueryRangeDataResult) UnmarshalJSON(b []byte) error {
	type plain QueryRangeDataResult
	return json.Unmarshal(testutil.PopulateTimeTpl(b, insertionTime), (*plain)(r))
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
	vmstorage.InitWithoutMetrics()
	vmselect.Init()
	vminsert.Init()
	go httpserver.Serve(*httpListenAddr, requestHandler)
	readyStorageCheckFunc := func() bool {
		resp, err := http.Get(testHealthHTTPPath)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == 200
	}
	if err := waitFor(testStorageInitTimeout, readyStorageCheckFunc); err != nil {
		log.Fatalf("http server can't start for %s seconds, err %s", testStorageInitTimeout, err)
	}
}

func processFlags() {
	envflag.Parse()
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
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		log.Printf("cannot stop the webservice: %s", err)
	}
	vminsert.Stop()
	vmstorage.Stop()
	vmselect.Stop()
	fs.MustRemoveAll(storagePath)
}

func TestWriteRead(t *testing.T) {
	t.Run("write", testWrite)
	time.Sleep(1 * time.Second)
	vmstorage.Stop()
	// open storage after stop in write
	vmstorage.InitWithoutMetrics()
	t.Run("read", testRead)
}

func testWrite(t *testing.T) {
	t.Run("prometheus", func(t *testing.T) {
		for _, test := range readIn("prometheus", t, insertionTime) {
			s := newSuite(t)
			r := testutil.WriteRequest{}
			s.noError(json.Unmarshal([]byte(strings.Join(test.Data, "\n")), &r.Timeseries))
			data, err := testutil.Compress(r)
			s.greaterThan(len(r.Timeseries), 0)
			if err != nil {
				t.Errorf("error compressing %v %s", r, err)
				t.Fail()
			}
			httpWrite(t, testPromWriteHTTPPath, bytes.NewBuffer(data))
		}
	})

	t.Run("influxdb", func(t *testing.T) {
		for _, x := range readIn("influxdb", t, insertionTime) {
			test := x
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				httpWrite(t, testWriteHTTPPath, bytes.NewBufferString(strings.Join(test.Data, "\n")))
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
				httpWrite(t, testOpenTSDBWriteHTTPPath, bytes.NewBufferString(strings.Join(test.Data, "\n")))
			})
		}
	})
}

func testRead(t *testing.T) {
	for _, engine := range []string{"prometheus", "graphite", "opentsdb", "influxdb", "opentsdbhttp"} {
		t.Run(engine, func(t *testing.T) {
			for _, x := range readIn(engine, t, insertionTime) {
				test := x
				t.Run(test.Name, func(t *testing.T) {
					t.Parallel()
					for _, q := range test.Query {
						q = testutil.PopulateTimeTplString(q, insertionTime)
						if test.Issue != "" {
							test.Issue = "Regression in " + test.Issue
						}
						switch true {
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
						case strings.HasPrefix(q, "/api/v1/query_range"):
							queryResult := QueryRange{}
							httpReadStruct(t, testReadHTTPPath, q, &queryResult)
							if err := checkQueryRangeResult(queryResult, test.ResultQueryRange); err != nil {
								t.Fatalf("Query Range. %s fails with error %s.%s", q, err, test.Issue)
							}
						case strings.HasPrefix(q, "/api/v1/query"):
							queryResult := Query{}
							httpReadStruct(t, testReadHTTPPath, q, &queryResult)
							if err := checkQueryResult(queryResult, test.ResultQuery); err != nil {
								t.Fatalf("Query. %s fails with error %s.%s", q, err, test.Issue)
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
	s.noError(filepath.Walk(filepath.Join(testFixturesDir, readFor), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		b, err := ioutil.ReadFile(path)
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

func httpWrite(t *testing.T, address string, r io.Reader) {
	t.Helper()
	s := newSuite(t)
	resp, err := http.Post(address, "", r)
	s.noError(err)
	s.noError(resp.Body.Close())
	s.equalInt(resp.StatusCode, 204)
}

func tcpWrite(t *testing.T, address string, data string) {
	t.Helper()
	s := newSuite(t)
	conn, err := net.Dial("tcp", address)
	s.noError(err)
	defer conn.Close()
	n, err := conn.Write([]byte(data))
	s.noError(err)
	s.equalInt(n, len(data))
}

func httpReadMetrics(t *testing.T, address, query string) []Metric {
	t.Helper()
	s := newSuite(t)
	resp, err := http.Get(address + query)
	s.noError(err)
	defer resp.Body.Close()
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
	defer resp.Body.Close()
	s.equalInt(resp.StatusCode, 200)
	s.noError(json.NewDecoder(resp.Body).Decode(dst))
}

func checkMetricsResult(got, want []Metric) error {
	for _, r := range append([]Metric(nil), got...) {
		want = removeIfFoundMetrics(r, want)
	}
	if len(want) > 0 {
		return fmt.Errorf("exptected metrics %+v not found in %+v", want, got)
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

func checkQueryResult(got, want Query) error {
	if got.Status != want.Status {
		return fmt.Errorf("status mismatch %q - %q", want.Status, got.Status)
	}
	if got.Data.ResultType != want.Data.ResultType {
		return fmt.Errorf("result type mismatch %q - %q", want.Data.ResultType, got.Data.ResultType)
	}
	wantData := append([]QueryDataResult(nil), want.Data.Result...)
	for _, r := range got.Data.Result {
		wantData = removeIfFoundQueryData(r, wantData)
	}
	if len(wantData) > 0 {
		return fmt.Errorf("expected query result %+v not found in %+v", wantData, got.Data.Result)
	}
	return nil
}

func removeIfFoundQueryData(r QueryDataResult, contains []QueryDataResult) []QueryDataResult {
	for i, item := range contains {
		if reflect.DeepEqual(r.Metric, item.Metric) && reflect.DeepEqual(r.Value[0], item.Value[0]) && reflect.DeepEqual(r.Value[1], item.Value[1]) {
			contains[i] = contains[len(contains)-1]
			return contains[:len(contains)-1]
		}
	}
	return contains
}

func checkQueryRangeResult(got, want QueryRange) error {
	if got.Status != want.Status {
		return fmt.Errorf("status mismatch %q - %q", want.Status, got.Status)
	}
	if got.Data.ResultType != want.Data.ResultType {
		return fmt.Errorf("result type mismatch %q - %q", want.Data.ResultType, got.Data.ResultType)
	}
	wantData := append([]QueryRangeDataResult(nil), want.Data.Result...)
	for _, r := range got.Data.Result {
		wantData = removeIfFoundQueryRangeData(r, wantData)
	}
	if len(wantData) > 0 {
		return fmt.Errorf("expected query range result %+v not found in %+v", wantData, got.Data.Result)
	}
	return nil
}

func removeIfFoundQueryRangeData(r QueryRangeDataResult, contains []QueryRangeDataResult) []QueryRangeDataResult {
	for i, item := range contains {
		if reflect.DeepEqual(r.Metric, item.Metric) && reflect.DeepEqual(r.Values, item.Values) {
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
