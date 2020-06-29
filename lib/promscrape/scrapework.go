package promscrape

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/metrics"
	xxhash "github.com/cespare/xxhash/v2"
)

var (
	suppressScrapeErrors = flag.Bool("promscrape.suppressScrapeErrors", false, "Whether to suppress scrape errors logging. "+
		"The last error for each target is always available at '/targets' page even if scrape errors logging is suppressed")
)

// ScrapeWork represents a unit of work for scraping Prometheus metrics.
type ScrapeWork struct {
	// Unique ID for the ScrapeWork.
	ID uint64

	// Full URL (including query args) for the scrape.
	ScrapeURL string

	// Interval for scraping the ScrapeURL.
	ScrapeInterval time.Duration

	// Timeout for scraping the ScrapeURL.
	ScrapeTimeout time.Duration

	// How to deal with conflicting labels.
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	HonorLabels bool

	// How to deal with scraped timestamps.
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	HonorTimestamps bool

	// Labels to add to the scraped metrics.
	//
	// The list contains at least the following labels according to https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
	//
	//     * job
	//     * __address__
	//     * __scheme__
	//     * __metrics_path__
	//     * __param_<name>
	//     * __meta_*
	//     * user-defined labels set via `relabel_configs` section in `scrape_config`
	//
	// See also https://prometheus.io/docs/concepts/jobs_instances/
	Labels []prompbmarshal.Label

	// Auth config
	AuthConfig *promauth.Config

	// Optional `metric_relabel_configs`.
	MetricRelabelConfigs []promrelabel.ParsedRelabelConfig

	// The maximum number of metrics to scrape after relabeling.
	SampleLimit int

	// The original 'job_name'
	jobNameOriginal string
}

// key returns unique identifier for the given sw.
//
// it can be used for comparing for equality two ScrapeWork objects.
func (sw *ScrapeWork) key() string {
	key := fmt.Sprintf("ScrapeURL=%s, ScrapeInterval=%s, ScrapeTimeout=%s, HonorLabels=%v, HonorTimestamps=%v, Labels=%s, "+
		"AuthConfig=%s, MetricRelabelConfigs=%s, SampleLimit=%d",
		sw.ScrapeURL, sw.ScrapeInterval, sw.ScrapeTimeout, sw.HonorLabels, sw.HonorTimestamps, sw.LabelsString(),
		sw.AuthConfig.String(), sw.metricRelabelConfigsString(), sw.SampleLimit)
	return key
}

func (sw *ScrapeWork) metricRelabelConfigsString() string {
	var sb strings.Builder
	for _, prc := range sw.MetricRelabelConfigs {
		fmt.Fprintf(&sb, "%s", prc.String())
	}
	return sb.String()
}

// Job returns job for the ScrapeWork
func (sw *ScrapeWork) Job() string {
	return promrelabel.GetLabelValueByName(sw.Labels, "job")
}

// LabelsString returns labels in Prometheus format for the given sw.
func (sw *ScrapeWork) LabelsString() string {
	labels := make([]string, 0, len(sw.Labels))
	for _, label := range promrelabel.FinalizeLabels(nil, sw.Labels) {
		labels = append(labels, fmt.Sprintf("%s=%q", label.Name, label.Value))
	}
	return "{" + strings.Join(labels, ", ") + "}"
}

type scrapeWork struct {
	// Config for the scrape.
	Config ScrapeWork

	// ReadData is called for reading the data.
	ReadData func(dst []byte) ([]byte, error)

	// PushData is called for pushing collected data.
	PushData func(wr *prompbmarshal.WriteRequest)

	bodyBuf []byte
	rows    parser.Rows
	tmpRow  parser.Row

	writeRequest prompbmarshal.WriteRequest
	labels       []prompbmarshal.Label
	samples      []prompbmarshal.Sample
}

func (sw *scrapeWork) run(stopCh <-chan struct{}) {
	// Calculate start time for the first scrape from ScrapeURL and labels.
	// This should spread load when scraping many targets with different
	// scrape urls and labels.
	// This also makes consistent scrape times across restarts
	// for a target with the same ScrapeURL and labels.
	scrapeInterval := sw.Config.ScrapeInterval
	key := fmt.Sprintf("ScrapeURL=%s, Labels=%s", sw.Config.ScrapeURL, sw.Config.LabelsString())
	h := uint32(xxhash.Sum64([]byte(key)))
	randSleep := uint64(float64(scrapeInterval) * (float64(h) / (1 << 32)))
	sleepOffset := uint64(time.Now().UnixNano()) % uint64(scrapeInterval)
	if randSleep < sleepOffset {
		randSleep += uint64(scrapeInterval)
	}
	randSleep -= sleepOffset
	timer := time.NewTimer(time.Duration(randSleep))
	var timestamp int64
	var ticker *time.Ticker
	select {
	case <-stopCh:
		timer.Stop()
		return
	case <-timer.C:
		ticker = time.NewTicker(scrapeInterval)
		timestamp = time.Now().UnixNano() / 1e6
		sw.scrapeAndLogError(timestamp)
	}
	defer ticker.Stop()
	for {
		timestamp += scrapeInterval.Milliseconds()
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			t := time.Now().UnixNano() / 1e6
			if d := t - timestamp; d > 0 && float64(d)/float64(scrapeInterval.Milliseconds()) > 0.1 {
				// Too big jitter. Adjust timestamp
				timestamp = t
			}
			sw.scrapeAndLogError(timestamp)
		}
	}
}

func (sw *scrapeWork) logError(s string) {
	if !*suppressScrapeErrors {
		logger.ErrorfSkipframes(1, "error when scraping %q from job %q with labels %s: %s", sw.Config.ScrapeURL, sw.Config.Job(), sw.Config.LabelsString(), s)
	}
}

func (sw *scrapeWork) scrapeAndLogError(timestamp int64) {
	if err := sw.scrapeInternal(timestamp); err != nil && !*suppressScrapeErrors {
		logger.Errorf("error when scraping %q from job %q with labels %s: %s", sw.Config.ScrapeURL, sw.Config.Job(), sw.Config.LabelsString(), err)
	}
}

var (
	scrapeDuration              = metrics.NewHistogram("vm_promscrape_scrape_duration_seconds")
	scrapeResponseSize          = metrics.NewHistogram("vm_promscrape_scrape_response_size_bytes")
	scrapedSamples              = metrics.NewHistogram("vm_promscrape_scraped_samples")
	scrapesSkippedBySampleLimit = metrics.NewCounter("vm_promscrape_scrapes_skipped_by_sample_limit_total")
	scrapesFailed               = metrics.NewCounter("vm_promscrape_scrapes_failed_total")
	pushDataDuration            = metrics.NewHistogram("vm_promscrape_push_data_duration_seconds")
)

func (sw *scrapeWork) scrapeInternal(timestamp int64) error {
	var err error
	sw.bodyBuf, err = sw.ReadData(sw.bodyBuf[:0])
	endTimestamp := time.Now().UnixNano() / 1e6
	duration := float64(endTimestamp-timestamp) / 1e3
	scrapeDuration.Update(duration)
	scrapeResponseSize.Update(float64(len(sw.bodyBuf)))
	up := 1
	if err != nil {
		up = 0
		scrapesFailed.Inc()
	} else {
		bodyString := bytesutil.ToUnsafeString(sw.bodyBuf)
		sw.rows.UnmarshalWithErrLogger(bodyString, sw.logError)
	}
	srcRows := sw.rows.Rows
	samplesScraped := len(srcRows)
	scrapedSamples.Update(float64(samplesScraped))
	for i := range srcRows {
		sw.addRowToTimeseries(&srcRows[i], timestamp, true)
	}
	sw.rows.Reset()
	if sw.Config.SampleLimit > 0 && len(sw.writeRequest.Timeseries) > sw.Config.SampleLimit {
		prompbmarshal.ResetWriteRequest(&sw.writeRequest)
		up = 0
		scrapesSkippedBySampleLimit.Inc()
	}
	samplesPostRelabeling := len(sw.writeRequest.Timeseries)
	sw.addAutoTimeseries("up", float64(up), timestamp)
	sw.addAutoTimeseries("scrape_duration_seconds", duration, timestamp)
	sw.addAutoTimeseries("scrape_samples_scraped", float64(samplesScraped), timestamp)
	sw.addAutoTimeseries("scrape_samples_post_metric_relabeling", float64(samplesPostRelabeling), timestamp)
	startTime := time.Now()
	sw.PushData(&sw.writeRequest)
	pushDataDuration.UpdateDuration(startTime)
	prompbmarshal.ResetWriteRequest(&sw.writeRequest)
	sw.labels = sw.labels[:0]
	sw.samples = sw.samples[:0]
	tsmGlobal.Update(&sw.Config, up == 1, timestamp, int64(duration*1000), err)
	return err
}

// addAutoTimeseries adds automatically generated time series with the given name, value and timestamp.
//
// See https://prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series
func (sw *scrapeWork) addAutoTimeseries(name string, value float64, timestamp int64) {
	sw.tmpRow.Metric = name
	sw.tmpRow.Tags = nil
	sw.tmpRow.Value = value
	sw.tmpRow.Timestamp = timestamp
	sw.addRowToTimeseries(&sw.tmpRow, timestamp, false)
}

func (sw *scrapeWork) addRowToTimeseries(r *parser.Row, timestamp int64, needRelabel bool) {
	labelsLen := len(sw.labels)
	sw.labels = appendLabels(sw.labels, r.Metric, r.Tags, sw.Config.Labels, sw.Config.HonorLabels)
	if needRelabel {
		sw.labels = promrelabel.ApplyRelabelConfigs(sw.labels, labelsLen, sw.Config.MetricRelabelConfigs, true)
	} else {
		sw.labels = promrelabel.FinalizeLabels(sw.labels[:labelsLen], sw.labels[labelsLen:])
		promrelabel.SortLabels(sw.labels[labelsLen:])
	}
	if len(sw.labels) == labelsLen {
		// Skip row without labels.
		return
	}
	labels := sw.labels[labelsLen:]
	sw.samples = append(sw.samples, prompbmarshal.Sample{})
	sample := &sw.samples[len(sw.samples)-1]
	sample.Value = r.Value
	sample.Timestamp = r.Timestamp
	if !sw.Config.HonorTimestamps || sample.Timestamp == 0 {
		sample.Timestamp = timestamp
	}
	wr := &sw.writeRequest
	wr.Timeseries = append(wr.Timeseries, prompbmarshal.TimeSeries{})
	ts := &wr.Timeseries[len(wr.Timeseries)-1]
	ts.Labels = labels
	ts.Samples = sw.samples[len(sw.samples)-1:]
}

func appendLabels(dst []prompbmarshal.Label, metric string, src []parser.Tag, extraLabels []prompbmarshal.Label, honorLabels bool) []prompbmarshal.Label {
	dstLen := len(dst)
	dst = append(dst, prompbmarshal.Label{
		Name:  "__name__",
		Value: metric,
	})
	for i := range src {
		tag := &src[i]
		dst = append(dst, prompbmarshal.Label{
			Name:  tag.Key,
			Value: tag.Value,
		})
	}
	dst = append(dst, extraLabels...)
	labels := dst[dstLen:]
	if len(labels) <= 1 {
		// Fast path - only a single label.
		return dst
	}

	// de-duplicate labels
	dstLabels := labels[:0]
	for i := range labels {
		label := &labels[i]
		prevLabel := promrelabel.GetLabelByName(dstLabels, label.Name)
		if prevLabel == nil {
			dstLabels = append(dstLabels, *label)
			continue
		}
		if honorLabels {
			// Skip the extra label with the same name.
			continue
		}
		// Rename the prevLabel to "exported_" + label.Name.
		// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
		exportedName := "exported_" + label.Name
		if promrelabel.GetLabelByName(dstLabels, exportedName) != nil {
			// Override duplicate with the current label.
			*prevLabel = *label
			continue
		}
		prevLabel.Name = exportedName
		dstLabels = append(dstLabels, *label)
	}
	return dst[:dstLen+len(dstLabels)]
}
