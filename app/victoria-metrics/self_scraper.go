package main

import (
	"flag"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/appmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeserieslimits"
)

var (
	selfScrapeInterval = flag.Duration("selfScrapeInterval", 0, "Interval for self-scraping own metrics at /metrics page")
	selfScrapeInstance = flag.String("selfScrapeInstance", "self", "Value for 'instance' label, which is added to self-scraped metrics")
	selfScrapeJob      = flag.String("selfScrapeJob", "victoria-metrics", "Value for 'job' label, which is added to self-scraped metrics")
)

var selfScraperStopCh chan struct{}
var selfScraperWG sync.WaitGroup

func startSelfScraper() {
	selfScraperStopCh = make(chan struct{})
	selfScraperWG.Go(func() {
		selfScraper(*selfScrapeInterval)
	})
}

func stopSelfScraper() {
	close(selfScraperStopCh)
	selfScraperWG.Wait()
}

func selfScraper(scrapeInterval time.Duration) {
	if scrapeInterval <= 0 {
		// Self-scrape is disabled.
		return
	}
	logger.Infof("started self-scraping `/metrics` page with interval %.3f seconds", scrapeInterval.Seconds())

	var bb bytesutil.ByteBuffer
	var rows prometheus.Rows
	var metadataRows prometheus.MetadataRows
	var mrs []storage.MetricRow
	var labels []prompb.Label
	t := time.NewTicker(scrapeInterval)
	f := func(currentTime time.Time, sendStaleMarkers bool) {
		currentTimestamp := currentTime.UnixNano() / 1e6
		bb.Reset()
		appmetrics.WritePrometheusMetrics(&bb)
		s := bytesutil.ToUnsafeString(bb.B)
		rows.Reset()
		// Parse metrics and optionally metadata when enabled
		if prommetadata.IsEnabled() {
			rows, metadataRows = prometheus.UnmarshalWithMetadata(rows, metadataRows, s, nil)
		} else {
			rows.UnmarshalWithErrLogger(s, nil)
		}
		mrs = mrs[:0]
		for i := range rows.Rows {
			r := &rows.Rows[i]
			labels = labels[:0]
			labels = addLabel(labels, "", r.Metric)
			labels = addLabel(labels, "job", *selfScrapeJob)
			labels = addLabel(labels, "instance", *selfScrapeInstance)
			for j := range r.Tags {
				t := &r.Tags[j]
				labels = addLabel(labels, t.Key, t.Value)
			}
			if timeserieslimits.IsExceeding(labels) {
				// Skip metric with exceeding labels.
				continue
			}
			if len(mrs) < cap(mrs) {
				mrs = mrs[:len(mrs)+1]
			} else {
				mrs = append(mrs, storage.MetricRow{})
			}
			mr := &mrs[len(mrs)-1]
			mr.MetricNameRaw = storage.MarshalMetricNameRaw(mr.MetricNameRaw[:0], labels)
			mr.Timestamp = currentTimestamp
			if sendStaleMarkers {
				mr.Value = decimal.StaleNaN
			} else {
				mr.Value = r.Value
			}
		}
		if err := vmstorage.AddRows(mrs); err != nil {
			logger.Errorf("cannot store self-scraped metrics: %s", err)
		}
		if len(metadataRows.Rows) > 0 {
			mms := make([]metricsmetadata.Row, 0, len(metadataRows.Rows))
			for _, mm := range metadataRows.Rows {
				mms = append(mms, metricsmetadata.Row{
					MetricFamilyName: bytesutil.ToUnsafeBytes(mm.Metric),
					Help:             bytesutil.ToUnsafeBytes(mm.Help),
					Type:             mm.Type,
				})
			}
			if err := vmstorage.AddMetadataRows(mms); err != nil {
				logger.Errorf("cannot store self-scraped metrics metadata: %s", err)
			}
		}
	}
	for {
		select {
		case <-selfScraperStopCh:
			f(time.Now(), true)
			t.Stop()
			logger.Infof("stopped self-scraping `/metrics` page")
			return
		case currentTime := <-t.C:
			f(currentTime, false)
		}
	}
}

func addLabel(dst []prompb.Label, key, value string) []prompb.Label {
	if len(dst) < cap(dst) {
		dst = dst[:len(dst)+1]
	} else {
		dst = append(dst, prompb.Label{})
	}
	lb := &dst[len(dst)-1]
	lb.Name = key
	lb.Value = value
	return dst
}
