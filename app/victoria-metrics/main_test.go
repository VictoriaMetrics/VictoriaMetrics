// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	testFixturesDir        = "testdata"
	testStorageSuffix      = "vm-test-storage"
	testHTTPListenAddr     = ":7654"
	testStatsDListenAddr   = ":2003"
	testOpenTSDBListenAddr = ":4242"
	testLogLevel           = "INFO"
)

const (
	testReadHTTPPath   = "http://127.0.0.1" + testHTTPListenAddr
	testWriteHTTPPath  = "http://127.0.0.1" + testHTTPListenAddr + "/write"
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
	Name   string `json:"name"`
	Data   string `json:"data"`
	Query  string `json:"query"`
	Result []Row  `json:"result"`
}

type Row struct {
	Metric     map[string]string `json:"metric"`
	Values     []float64         `json:"values"`
	Timestamps []int64           `json:"timestamps"`
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
	for _, fs := range []struct {
		flag  string
		value string
	}{
		{flag: "storageDataPath", value: storagePath},
		{flag: "httpListenAddr", value: testHTTPListenAddr},
		{flag: "graphiteListenAddr", value: testStatsDListenAddr},
		{flag: "opentsdbListenAddr", value: testOpenTSDBListenAddr},
		{flag: "loggerLevel", value: testLogLevel},
	} {
		// panics if flag doesn't exist
		if err := flag.Lookup(fs.flag).Value.Set(fs.value); err != nil {
			log.Fatalf("unable to set %q with value %q, err: %v", fs.flag, fs.value, err)
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
	vminsert.Stop()
	vmstorage.Stop()
	vmselect.Stop()
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		log.Fatalf("cannot stop the webservice: %s", err)
	}
	os.RemoveAll(storagePath)
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
	t.Run("influxdb", func(t *testing.T) {
		for _, test := range readIn("influxdb", t, fmt.Sprintf("%d", insertionTime.UnixNano())) {
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				httpWrite(t, testWriteHTTPPath, test.Data)
			})
		}
	})
	t.Run("graphite", func(t *testing.T) {
		for _, test := range readIn("graphite", t, fmt.Sprintf("%d", insertionTime.Unix())) {
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				tcpWrite(t, "127.0.0.1"+testStatsDListenAddr, test.Data)
			})
		}
	})
	t.Run("opentsdb", func(t *testing.T) {
		for _, test := range readIn("opentsdb", t, fmt.Sprintf("%d", insertionTime.Unix())) {
			t.Run(test.Name, func(t *testing.T) {
				t.Parallel()
				tcpWrite(t, "127.0.0.1"+testOpenTSDBListenAddr, test.Data)
			})
		}
	})
}

func testRead(t *testing.T) {
	for _, engine := range []string{"graphite", "opentsdb", "influxdb"} {
		t.Run(engine, func(t *testing.T) {
			for _, test := range readIn(engine, t, fmt.Sprintf("%d", insertionTime.UnixNano())) {
				test := test
				t.Run(test.Name, func(t *testing.T) {
					t.Parallel()
					rowContains(t, httpRead(t, testReadHTTPPath, test.Query), test.Result)
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
		item.Data = strings.Replace(item.Data, "{TIME}", timeStr, 1)
		tt = append(tt, item)
		return nil
	}))
	if len(tt) == 0 {
		t.Fatalf("no test found in %s", filepath.Join(testFixturesDir, readFor))
	}
	return tt
}

func httpWrite(t *testing.T, address string, data string) {
	t.Helper()
	s := newSuite(t)
	resp, err := http.Post(address, "", bytes.NewBufferString(data))
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

func rowContains(t *testing.T, rows, contains []Row) {
	t.Helper()
	for _, r := range rows {
		contains = removeIfFound(r, contains)
	}
	if len(contains) > 0 {
		t.Fatalf("result rows %+v not found in %+v", contains, rows)
	}
}

func removeIfFound(r Row, contains []Row) []Row {
	for i, item := range contains {
		// todo check time
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
