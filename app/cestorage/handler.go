package main

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/promremotewrite/stream"
)

func requestHandler(estimators []*Estimator) httpserver.RequestHandler {
	return func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/v1/write" {
			handleRemoteWrite(w, r, estimators)
			return true
		}
		return false
	}
}

func handleRemoteWrite(w http.ResponseWriter, r *http.Request, estimators []*Estimator) {
	isVMRemoteWrite := r.Header.Get("Content-Encoding") == "zstd"
	err := stream.Parse(r.Body, isVMRemoteWrite, func(tss []prompb.TimeSeries, _ []prompb.MetricMetadata) error {
		for i := range tss {
			for _, e := range estimators {
				e.insert(tss[i].Labels)
			}
		}
		return nil
	})
	if err != nil {
		httpserver.Errorf(w, r, "error parsing remote write request: %s", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
