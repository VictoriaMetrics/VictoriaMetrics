package remote_read_integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/native/stream"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

type RemoteWriteServer struct {
	server         *httptest.Server
	series         []vm.TimeSeries
	expectedSeries []vm.TimeSeries
	storage        *storage.Storage
}

// NewRemoteWriteServer prepares test remote write server
func NewRemoteWriteServer(t *testing.T) *RemoteWriteServer {
	rws := &RemoteWriteServer{series: make([]vm.TimeSeries, 0)}
	mux := http.NewServeMux()

	mux.Handle("/api/v1/import", rws.getWriteHandler(t))
	mux.Handle("/health", rws.handlePing())
	mux.Handle("/api/v1/series", rws.seriesHandler(t))
	mux.Handle("/api/v1/export/native", rws.exportNativeHandler(t))
	mux.Handle("/api/v1/import/native", rws.importNativeHandler(t))
	rws.server = httptest.NewServer(mux)
	return rws
}

// Close closes the server.
func (rws *RemoteWriteServer) Close() {
	rws.server.Close()
}

func (rws *RemoteWriteServer) Series(series []vm.TimeSeries) {
	rws.series = append(rws.series, series...)
}

func (rws *RemoteWriteServer) ExpectedSeries(series []vm.TimeSeries) {
	rws.expectedSeries = append(rws.expectedSeries, series...)
}

func (rws *RemoteWriteServer) InitFakeStorage() error {
	s, err := storage.OpenStorage("TestStorage", 0, 0, 0)
	if err != nil {
		return fmt.Errorf("cannot open storage: %s", err)
	}

	vmstorage.Storage = s
	rws.storage = vmstorage.Storage

	var mrs []storage.MetricRow
	for _, series := range rws.series {
		mr := storage.MetricRow{}
		for _, ts := range series.Timestamps {
			mr.Timestamp = ts
		}
		for _, v := range series.Values {
			mr.Value = v
		}
		var labels []prompb.Label
		for _, lp := range series.LabelPairs {
			labels = append(labels, prompb.Label{Name: []byte(lp.Name), Value: []byte(lp.Value)})
		}
		if series.Name != "" {
			labels = append(labels, prompb.Label{Name: []byte("__name__"), Value: []byte(series.Name)})
		}
		mr.MetricNameRaw = storage.MarshalMetricNameRaw(mr.MetricNameRaw[:0], labels)
		mrs = append(mrs, mr)
		if err := s.AddRows(mrs, 4); err != nil {
			return fmt.Errorf("unexpected error in RegisterMetricNames: %s", err)
		}
	}
	s.DebugFlush()
	return nil
}

func (rws *RemoteWriteServer) CloseStorage() {
	rws.storage.MustClose()
	if err := os.RemoveAll("TestStorage"); err != nil {
		log.Fatalf("cannot remove %q: %s", "TestStorage", err)
	}
}

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
	})
}

func (rws *RemoteWriteServer) handlePing() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
}

// LabelValues represents series from api/v1/series response
type LabelValues map[string]string

// Response represents response from api/v1/series
type Response struct {
	Status string        `json:"status"`
	Series []LabelValues `json:"data"`
}

func (rws *RemoteWriteServer) seriesHandler(t *testing.T) http.Handler {
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
			return
		}
	})
}

func (rws *RemoteWriteServer) exportNativeHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		err := prometheus.ExportNativeHandler(now, w, r)
		if err != nil {
			log.Printf("ERRR => %s", err)
			return
		}
	})
}

func (rws *RemoteWriteServer) importNativeHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		common.StartUnmarshalWorkers()
		defer common.StopUnmarshalWorkers()
		err := stream.Parse(r.Body, false, func(block *stream.Block) error {
			log.Printf("BLOCK => %s", block.MetricName.String())
			log.Printf("")
			return nil
		})
		if err != nil {
			log.Printf("GOT ERROR => %s", err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

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
