package remote_read_integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/native/stream"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/vmimport"
)

// LabelValues represents series from api/v1/series response
type LabelValues map[string]string

// Response represents response from api/v1/series
type Response struct {
	Status string        `json:"status"`
	Series []LabelValues `json:"data"`
}

type MetricNamesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

// RemoteWriteServer represents fake remote write server with database
type RemoteWriteServer struct {
	server         *httptest.Server
	series         []vm.TimeSeries
	expectedSeries []vm.TimeSeries
}

// NewRemoteWriteServer prepares test remote write server
func NewRemoteWriteServer(t *testing.T) *RemoteWriteServer {
	rws := &RemoteWriteServer{series: make([]vm.TimeSeries, 0)}
	mux := http.NewServeMux()

	mux.Handle("/api/v1/import", rws.getWriteHandler(t))
	mux.Handle("/health", rws.handlePing())
	mux.Handle("/api/v1/series", rws.seriesHandler())
	mux.Handle("/api/v1/label/__name__/values", rws.valuesHandler())
	mux.Handle("/api/v1/export/native", rws.exportNativeHandler())
	mux.Handle("/api/v1/import/native", rws.importNativeHandler(t))
	rws.server = httptest.NewServer(mux)
	return rws
}

// Close closes the server
func (rws *RemoteWriteServer) Close() {
	rws.server.Close()
}

// Series saves generated series for fake database
func (rws *RemoteWriteServer) Series(series []vm.TimeSeries) {
	rws.series = append(rws.series, series...)
}

// ExpectedSeries saves expected results to check in the handler
func (rws *RemoteWriteServer) ExpectedSeries(series []vm.TimeSeries) {
	rws.expectedSeries = append(rws.expectedSeries, series...)
}

// URL returns server url
func (rws *RemoteWriteServer) URL() string {
	return rws.server.URL
}

func (rws *RemoteWriteServer) getWriteHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tss []vm.TimeSeries
		scanner := bufio.NewScanner(r.Body)
		var rows parser.Rows
		for scanner.Scan() {

			rows.Unmarshal(scanner.Text())
			for _, row := range rows.Rows {
				var labelPairs []vm.LabelPair
				var ts vm.TimeSeries
				nameValue := ""
				for _, tag := range row.Tags {
					if string(tag.Key) == "__name__" {
						nameValue = string(tag.Value)
						continue
					}
					labelPairs = append(labelPairs, vm.LabelPair{Name: string(tag.Key), Value: string(tag.Value)})
				}

				ts.Values = append(ts.Values, row.Values...)
				ts.Timestamps = append(ts.Timestamps, row.Timestamps...)
				ts.Name = nameValue
				ts.LabelPairs = labelPairs
				tss = append(tss, ts)
			}
			rows.Reset()
		}

		if !reflect.DeepEqual(tss, rws.expectedSeries) {
			w.WriteHeader(http.StatusInternalServerError)
			t.Fatalf("datasets not equal, expected: %#v; \n got: %#v", rws.expectedSeries, tss)
			return
		}

		w.WriteHeader(http.StatusNoContent)
		return
	})
}

func (rws *RemoteWriteServer) handlePing() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
}

func (rws *RemoteWriteServer) seriesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var labelValues []LabelValues
		for _, ser := range rws.series {
			metricNames := make(LabelValues)
			if ser.Name != "" {
				metricNames["__name__"] = ser.Name
			}
			for _, p := range ser.LabelPairs {
				metricNames[p.Name] = p.Value
			}
			labelValues = append(labelValues, metricNames)
		}

		resp := Response{
			Status: "success",
			Series: labelValues,
		}

		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			log.Printf("error send series: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}

func (rws *RemoteWriteServer) valuesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		labelNames := make(map[string]struct{})
		for _, ser := range rws.series {
			if ser.Name != "" {
				labelNames[ser.Name] = struct{}{}
			}
		}

		metricNames := make([]string, 0, len(labelNames))
		for k := range labelNames {
			metricNames = append(metricNames, k)
		}
		resp := MetricNamesResponse{
			Status: "success",
			Data:   metricNames,
		}

		buf := bytes.NewBuffer(nil)
		err := json.NewEncoder(buf).Encode(resp)
		if err != nil {
			log.Printf("error send series: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(buf.Bytes())
		if err != nil {
			log.Printf("error send series: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		return
	})
}

func (rws *RemoteWriteServer) exportNativeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		err := prometheus.ExportNativeHandler(now, w, r)
		if err != nil {
			log.Printf("error export series via native protocol: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		return
	})
}

func (rws *RemoteWriteServer) importNativeHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		common.StartUnmarshalWorkers()
		defer common.StopUnmarshalWorkers()

		var gotTimeSeries []vm.TimeSeries
		var mx sync.RWMutex

		err := stream.Parse(r.Body, false, func(block *stream.Block) error {
			mn := &block.MetricName
			var timeseries vm.TimeSeries
			timeseries.Name = string(mn.MetricGroup)
			timeseries.Timestamps = append(timeseries.Timestamps, block.Timestamps...)
			timeseries.Values = append(timeseries.Values, block.Values...)

			for i := range mn.Tags {
				tag := &mn.Tags[i]
				timeseries.LabelPairs = append(timeseries.LabelPairs, vm.LabelPair{
					Name:  string(tag.Key),
					Value: string(tag.Value),
				})
			}

			mx.Lock()
			gotTimeSeries = append(gotTimeSeries, timeseries)
			mx.Unlock()

			return nil
		})
		if err != nil {
			log.Printf("error parse stream blocks: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// got timeseries should be sorted
		// because they are processed independently
		sort.SliceStable(gotTimeSeries, func(i, j int) bool {
			iv, jv := gotTimeSeries[i], gotTimeSeries[j]
			switch {
			case iv.Values[0] != jv.Values[0]:
				return iv.Values[0] < jv.Values[0]
			case iv.Timestamps[0] != jv.Timestamps[0]:
				return iv.Timestamps[0] < jv.Timestamps[0]
			default:
				return iv.Name < jv.Name
			}
		})

		if !reflect.DeepEqual(gotTimeSeries, rws.expectedSeries) {
			w.WriteHeader(http.StatusInternalServerError)
			t.Errorf("datasets not equal, expected: %#v;\n got: %#v", rws.expectedSeries, gotTimeSeries)
			return
		}

		w.WriteHeader(http.StatusNoContent)
		return
	})
}

// GenerateVNSeries generates test timeseries
func GenerateVNSeries(start, end, numOfSeries, numOfSamples int64) []vm.TimeSeries {
	var ts []vm.TimeSeries
	j := 0
	for i := 0; i < int(numOfSeries); i++ {
		if i%3 == 0 {
			j++
		}

		timeSeries := vm.TimeSeries{
			Name: fmt.Sprintf("vm_metric_%d", j),
			LabelPairs: []vm.LabelPair{
				{Name: "job", Value: strconv.Itoa(i)},
			},
		}

		ts = append(ts, timeSeries)
	}

	for i := range ts {
		t, v := generateTimeStampsAndValues(i, start, end, numOfSamples)
		ts[i].Timestamps = t
		ts[i].Values = v
	}

	return ts
}

func generateTimeStampsAndValues(idx int, startTime, endTime, numOfSamples int64) ([]int64, []float64) {
	delta := (endTime - startTime) / numOfSamples

	var timestamps []int64
	var values []float64
	t := startTime
	for t != endTime {
		v := 100 * int64(idx)
		timestamps = append(timestamps, t*1000)
		values = append(values, float64(v))
		t = t + delta
	}

	return timestamps, values
}
