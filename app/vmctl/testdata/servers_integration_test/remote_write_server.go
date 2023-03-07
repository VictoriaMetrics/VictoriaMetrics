package remote_read_integration

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

type RemoteWriteServer struct {
	server  *httptest.Server
	series  []vm.TimeSeries
	storage *storage.Storage
}

// NewRemoteWriteServer prepares test remote write server
func NewRemoteWriteServer(t *testing.T) *RemoteWriteServer {
	rws := &RemoteWriteServer{series: make([]vm.TimeSeries, 0)}
	mux := http.NewServeMux()

	mux.Handle("/api/v1/import", rws.getWriteHandler(t))
	mux.Handle("/health", rws.handlePing())
	mux.Handle("/api/v1/series", rws.seriesHandler(t))
	mux.Handle("/api/v1/export/native", rws.exportNativeHandler(t))
	// mux.Handle("/api/v1/import/native", rws.importNativeHandler(t))
	rws.server = httptest.NewServer(mux)
	return rws
}

func (rws *RemoteWriteServer) InitStorage(path string) {
	s, err := storage.OpenStorage(path, 0, 0, 0)
	if err != nil {
		log.Fatalf("cannot open storage: %s", err)
	}

	rws.storage = s
}

func (rws *RemoteWriteServer) CloseStorage(path string) {
	rws.storage.MustClose()

	if err := os.RemoveAll(path); err != nil {
		log.Fatalf("cannot remove %q: %s", path, err)
	}
}

// Close closes the server.
func (rws *RemoteWriteServer) Close() {
	rws.server.Close()
}

func (rws *RemoteWriteServer) ExpectedSeries(series []vm.TimeSeries) {
	rws.series = append(rws.series, series...)
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

		if !reflect.DeepEqual(tss, rws.series) {
			w.WriteHeader(http.StatusInternalServerError)
			t.Fatalf("datasets not equal, expected: %#v; \n got: %#v", rws.series, tss)
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

func (rws *RemoteWriteServer) seriesHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		const noDeadline = 1<<64 - 1

		const metricsPerAdd = 1e3
		const addsCount = 10
		const msecPerDay = 24 * 3600 * 1000
		var labels []prompb.Label

		for i := 0; i < addsCount; i++ {
			var mrs []storage.MetricRow
			addID := fmt.Sprintf("%d", i)

			labels = []prompb.Label{
				{[]byte("__name__"), []byte("test_metric")},
				{[]byte("job"), []byte("webservice")},
				{[]byte("instance"), []byte("1.2.3.4")},
				{[]byte("add_id"), []byte(addID)},
			}
			now := time.Now()
			ts := now.UnixNano() / 1e6
			for j := 0; j < metricsPerAdd; j++ {
				mr := storage.MetricRow{Timestamp: ts}
				mr.MetricNameRaw = storage.MarshalMetricNameRaw(mr.MetricNameRaw[:0], labels)
				mrs = append(mrs, mr)
			}
			if err := rws.storage.RegisterMetricNames(nil, mrs); err != nil {
				t.Fatalf("unexpected error in RegisterMetricNames: %s", err)
			}
		}

		rws.storage.DebugFlush()

		now := time.Now()
		ts := now.UnixNano() / 1e6
		start := ts - msecPerDay
		end := ts + 60*1000
		tr := storage.TimeRange{
			MinTimestamp: start,
			MaxTimestamp: end,
		}

		tfs := storage.NewTagFilters()
		if err := tfs.Add([]byte("add_id"), []byte("0"), false, false); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %w", err)
		}
		metricNames, err := rws.storage.SearchMetricNames(nil, []*storage.TagFilters{tfs}, tr, metricsPerAdd*addsCount*100+100, noDeadline)
		if err != nil {
			t.Fatalf("error in SearchMetricNames: %w", err)
		}

		prometheus.WriteSeriesResponse(w, metricNames, nil, func() {
			return
		})
	})
}

func (rws *RemoteWriteServer) exportNativeHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := time.Parse(time.RFC3339, "2023-03-07T00:00:00Z")
		if err != nil {
			t.Fatalf("error parse time: %s", err)
		}
		// @TODO need use storage for export response
		// if err := prometheus.ExportNativeHandler(ts, w, r); err != nil {
		// 	t.Fatalf("error export %s", err)
		// }
	})
}
