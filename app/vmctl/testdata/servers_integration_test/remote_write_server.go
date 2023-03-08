package remote_read_integration

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/vmimport"
)

type RemoteWriteServer struct {
	server *httptest.Server
	series []vm.TimeSeries
}

// NewRemoteWriteServer prepares test remote write server
func NewRemoteWriteServer(t *testing.T) *RemoteWriteServer {
	rws := &RemoteWriteServer{series: make([]vm.TimeSeries, 0)}
	mux := http.NewServeMux()

	mux.Handle("/api/v1/import", rws.getWriteHandler(t))
	mux.Handle("/health", rws.handlePing())
	rws.server = httptest.NewServer(mux)
	return rws
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
