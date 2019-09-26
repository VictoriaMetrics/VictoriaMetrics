// +build integration

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

const (
	tplWordTime              = "{TIME}"
	tplQuotedWordTime        = `"{TIME}"`
	tplQuotedWordTimeSeconds = `"{TIME_S}"`
	tplQuotedWordTimeMillis  = `"{TIME_MS}"`
)

var (
	storagePath   string
	insertionTime = time.Now().UTC()
)

type test struct {
	Name   string `json:"name"`
	Data   string `json:"data"`
	Query  string `json:"query"`
	Result []Row  `json:"result"`
	Issue  string `json:"issue"`
}

type Row struct {
	Metric     map[string]string `json:"metric"`
	Values     []float64         `json:"values"`
	Timestamps []int64           `json:"timestamps"`
}

func (r *Row) UnmarshalJSON(b []byte) error {
	type withoutInterface Row
	var to withoutInterface
	if err := json.Unmarshal(populateTimeTpl(b), &to); err != nil {
		return err
	}
	*r = Row(to)
	return nil
}

func populateTimeTpl(b []byte) []byte {
	var (
		tplTimeToQuotedMS = [2][]byte{[]byte(tplQuotedWordTimeMillis), []byte(fmt.Sprintf("%d", timeToMillis(insertionTime)))}
		tpsTimeToQuotedS  = [2][]byte{[]byte(tplQuotedWordTimeSeconds), []byte(fmt.Sprintf("%d", insertionTime.Unix()*1e3))}
	)
	tpls := [][2][]byte{
		tplTimeToQuotedMS, tpsTimeToQuotedS,
	}
	for i := range tpls {
		b = bytes.ReplaceAll(b, tpls[i][0], tpls[i][1])
	}
	return b
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
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		log.Printf("cannot stop the webservice: %s", err)
	}
	vminsert.Stop()
	vmstorage.Stop()
	vmselect.Stop()
	fs.MustRemoveAll(storagePath)
	fs.MustStopDirRemover()
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
		for _, test := range readIn("prometheus", t, fmt.Sprintf("%d", timeToMillis(insertionTime))) {
			s := newSuite(t)
			r := testutil.WriteRequest{}
			s.noError(json.Unmarshal([]byte(test.Data), &r.Timeseries))
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
		for _, x := range readIn("influxdb", t, fmt.Sprintf("%d", insertionTime.UnixNano())) {
			test := x
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				httpWrite(t, testWriteHTTPPath, bytes.NewBufferString(test.Data))
			})
		}
	})
	t.Run("graphite", func(t *testing.T) {
		for _, x := range readIn("graphite", t, fmt.Sprintf("%d", insertionTime.Unix())) {
			test := x
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				tcpWrite(t, "127.0.0.1"+testStatsDListenAddr, test.Data)
			})
		}
	})
	t.Run("opentsdb", func(t *testing.T) {
		for _, x := range readIn("opentsdb", t, fmt.Sprintf("%d", insertionTime.Unix())) {
			test := x
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				tcpWrite(t, "127.0.0.1"+testOpenTSDBListenAddr, test.Data)
			})
		}
	})
	t.Run("opentsdbhttp", func(t *testing.T) {
		for _, x := range readIn("opentsdbhttp", t, fmt.Sprintf("%d", insertionTime.Unix())) {
			test := x
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				logger.Infof("writing %s", test.Data)
				httpWrite(t, testOpenTSDBWriteHTTPPath, bytes.NewBufferString(test.Data))
			})
		}
	})
}

func testRead(t *testing.T) {
	for _, engine := range []string{"prometheus", "graphite", "opentsdb", "influxdb", "opentsdbhttp"} {
		t.Run(engine, func(t *testing.T) {
			for _, x := range readIn(engine, t, fmt.Sprintf("%d", insertionTime.UnixNano())) {
				test := x
				t.Run(test.Name, func(t *testing.T) {
					t.Parallel()
					rowContains(t, httpRead(t, testReadHTTPPath, test.Query), test.Result, test.Issue)
				})
			}
		})
	}
}

func readIn(readFor string, t *testing.T, timeStr string) []test {
	t.Helper()
	s := newSuite(t)
	var tt []test
	s.noError(filepath.Walk(filepath.Join(testFixturesDir, readFor), func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) != ".json" {
			return nil
		}
		b, err := ioutil.ReadFile(path)
		s.noError(err)
		item := test{}
		s.noError(json.Unmarshal(b, &item))
		item.Data = strings.Replace(item.Data, tplQuotedWordTime, timeStr, -1)
		item.Data = strings.Replace(item.Data, tplWordTime, timeStr, -1)
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

func httpRead(t *testing.T, address, query string) []Row {
	t.Helper()
	s := newSuite(t)
	resp, err := http.Get(address + query)
	s.noError(err)
	defer resp.Body.Close()
	s.equalInt(resp.StatusCode, 200)
	var rows []Row
	for dec := json.NewDecoder(resp.Body); dec.More(); {
		var row Row
		s.noError(dec.Decode(&row))
		rows = append(rows, row)
	}
	return rows
}

func rowContains(t *testing.T, rows, contains []Row, issue string) {
	t.Helper()
	for _, r := range rows {
		contains = removeIfFound(r, contains)
	}
	if len(contains) > 0 {
		if issue != "" {
			issue = "Regression in " + issue
		}
		t.Fatalf("result rows %+v not found in %+v.%s", contains, rows, issue)
	}
}

func removeIfFound(r Row, contains []Row) []Row {
	for i, item := range contains {
		if reflect.DeepEqual(r.Metric, item.Metric) && reflect.DeepEqual(r.Values, item.Values) &&
			reflect.DeepEqual(r.Timestamps, item.Timestamps) {
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

func timeToMillis(t time.Time) int64 {
	return t.UnixNano() / 1e6
}
