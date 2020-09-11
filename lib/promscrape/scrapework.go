package promscrape

import (
	"flag"
	"fmt"
	"math"
	"math/bits"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/leveledbytebufferpool"
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

	// Whether to disable response compression when querying ScrapeURL.
	DisableCompression bool

	// Whether to disable HTTP keep-alive when querying ScrapeURL.
	DisableKeepAlive bool

	// The original 'job_name'
	jobNameOriginal string
}

// key returns unique identifier for the given sw.
//
// it can be used for comparing for equality two ScrapeWork objects.
func (sw *ScrapeWork) key() string {
	key := fmt.Sprintf("ScrapeURL=%s, ScrapeInterval=%s, ScrapeTimeout=%s, HonorLabels=%v, HonorTimestamps=%v, Labels=%s, "+
		"AuthConfig=%s, MetricRelabelConfigs=%s, SampleLimit=%d, DisableCompression=%v, DisableKeepAlive=%v",
		sw.ScrapeURL, sw.ScrapeInterval, sw.ScrapeTimeout, sw.HonorLabels, sw.HonorTimestamps, sw.LabelsString(),
		sw.AuthConfig.String(), sw.metricRelabelConfigsString(), sw.SampleLimit, sw.DisableCompression, sw.DisableKeepAlive)
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

	// ScrapeGroup is name of ScrapeGroup that
	// scrapeWork belongs to
	ScrapeGroup string

	tmpRow parser.Row

	// the seriesMap, seriesAdded and labelsHashBuf are used for fast calculation of `scrape_series_added` metric.
	seriesMap     map[uint64]struct{}
	seriesAdded   int
	labelsHashBuf []byte

	// prevBodyLen contains the previous response body length for the given scrape work.
	// It is used as a hint in order to reduce memory usage for body buffers.
	prevBodyLen int

	// prevRowsLen contains the number rows scraped during the previous scrape.
	// It is used as a hint in order to reduce memory usage when parsing scrape responses.
	prevRowsLen int
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
		sw.scrapeAndLogError(timestamp, timestamp)
	}
	defer ticker.Stop()
	for {
		timestamp += scrapeInterval.Milliseconds()
		select {
		case <-stopCh:
			return
		case tt := <-ticker.C:
			t := tt.UnixNano() / 1e6
			if d := math.Abs(float64(t - timestamp)); d > 0 && d/float64(scrapeInterval.Milliseconds()) > 0.1 {
				// Too big jitter. Adjust timestamp
				timestamp = t
			}
			sw.scrapeAndLogError(timestamp, t)
		}
	}
}

func (sw *scrapeWork) logError(s string) {
	if !*suppressScrapeErrors {
		logger.ErrorfSkipframes(1, "error when scraping %q from job %q with labels %s: %s", sw.Config.ScrapeURL, sw.Config.Job(), sw.Config.LabelsString(), s)
	}
}

func (sw *scrapeWork) scrapeAndLogError(scrapeTimestamp, realTimestamp int64) {
	if err := sw.scrapeInternal(scrapeTimestamp, realTimestamp); err != nil && !*suppressScrapeErrors {
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

func (sw *scrapeWork) scrapeInternal(scrapeTimestamp, realTimestamp int64) error {
	body := leveledbytebufferpool.Get(sw.prevBodyLen)
	var err error
	body.B, err = sw.ReadData(body.B[:0])
	endTimestamp := time.Now().UnixNano() / 1e6
	duration := float64(endTimestamp-realTimestamp) / 1e3
	scrapeDuration.Update(duration)
	scrapeResponseSize.Update(float64(len(body.B)))
	up := 1
	wc := writeRequestCtxPool.Get(sw.prevRowsLen)
	if err != nil {
		up = 0
		scrapesFailed.Inc()
	} else {
		bodyString := bytesutil.ToUnsafeString(body.B)
		wc.rows.UnmarshalWithErrLogger(bodyString, sw.logError)
	}
	srcRows := wc.rows.Rows
	samplesScraped := len(srcRows)
	scrapedSamples.Update(float64(samplesScraped))
	if sw.Config.SampleLimit > 0 && samplesScraped > sw.Config.SampleLimit {
		srcRows = srcRows[:0]
		up = 0
		scrapesSkippedBySampleLimit.Inc()
	}
	samplesPostRelabeling := 0
	for i := range srcRows {
		sw.addRowToTimeseries(wc, &srcRows[i], scrapeTimestamp, true)
		if len(wc.labels) > 40000 {
			// Limit the maximum size of wc.writeRequest.
			// This should reduce memory usage when scraping targets with millions of metrics and/or labels.
			// For example, when scraping /federate handler from Prometheus - see https://prometheus.io/docs/prometheus/latest/federation/
			samplesPostRelabeling += len(wc.writeRequest.Timeseries)
			sw.updateSeriesAdded(wc)
			startTime := time.Now()
			sw.PushData(&wc.writeRequest)
			pushDataDuration.UpdateDuration(startTime)
			wc.resetNoRows()
		}
	}
	samplesPostRelabeling += len(wc.writeRequest.Timeseries)
	sw.updateSeriesAdded(wc)
	seriesAdded := sw.finalizeSeriesAdded(samplesPostRelabeling)
	sw.addAutoTimeseries(wc, "up", float64(up), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_duration_seconds", duration, scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_samples_scraped", float64(samplesScraped), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_samples_post_metric_relabeling", float64(samplesPostRelabeling), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_series_added", float64(seriesAdded), scrapeTimestamp)
	startTime := time.Now()
	sw.PushData(&wc.writeRequest)
	pushDataDuration.UpdateDuration(startTime)
	sw.prevRowsLen = samplesScraped
	wc.reset()
	writeRequestCtxPool.Put(wc)
	// body must be released only after wc is released, since wc refers to body.
	sw.prevBodyLen = len(body.B)
	leveledbytebufferpool.Put(body)
	tsmGlobal.Update(&sw.Config, sw.ScrapeGroup, up == 1, realTimestamp, int64(duration*1000), err)
	return err
}

// leveledWriteRequestCtxPool allows reducing memory usage when writeRequesCtx
// structs contain mixed number of labels.
//
// Its logic has been copied from leveledbytebufferpool.
type leveledWriteRequestCtxPool struct {
	pools [30]sync.Pool
}

func (lwp *leveledWriteRequestCtxPool) Get(rowsCapacity int) *writeRequestCtx {
	id, capacityNeeded := lwp.getPoolIDAndCapacity(rowsCapacity)
	for i := 0; i < 2; i++ {
		if id < 0 || id >= len(lwp.pools) {
			break
		}
		if v := lwp.pools[id].Get(); v != nil {
			return v.(*writeRequestCtx)
		}
		id++
	}
	return &writeRequestCtx{
		labels: make([]prompbmarshal.Label, 0, capacityNeeded),
	}
}

func (lwp *leveledWriteRequestCtxPool) Put(wc *writeRequestCtx) {
	capacity := cap(wc.rows.Rows)
	id, _ := lwp.getPoolIDAndCapacity(capacity)
	wc.reset()
	lwp.pools[id].Put(wc)
}

func (lwp *leveledWriteRequestCtxPool) getPoolIDAndCapacity(size int) (int, int) {
	size--
	if size < 0 {
		size = 0
	}
	size >>= 3
	id := bits.Len(uint(size))
	if id > len(lwp.pools) {
		id = len(lwp.pools) - 1
	}
	return id, (1 << (id + 3))
}

type writeRequestCtx struct {
	rows         parser.Rows
	writeRequest prompbmarshal.WriteRequest
	labels       []prompbmarshal.Label
	samples      []prompbmarshal.Sample
}

func (wc *writeRequestCtx) reset() {
	wc.rows.Reset()
	wc.resetNoRows()
}

func (wc *writeRequestCtx) resetNoRows() {
	prompbmarshal.ResetWriteRequest(&wc.writeRequest)
	wc.labels = wc.labels[:0]
	wc.samples = wc.samples[:0]
}

var writeRequestCtxPool leveledWriteRequestCtxPool

func (sw *scrapeWork) updateSeriesAdded(wc *writeRequestCtx) {
	if sw.seriesMap == nil {
		sw.seriesMap = make(map[uint64]struct{}, len(wc.writeRequest.Timeseries))
	}
	m := sw.seriesMap
	for _, ts := range wc.writeRequest.Timeseries {
		h := sw.getLabelsHash(ts.Labels)
		if _, ok := m[h]; !ok {
			m[h] = struct{}{}
			sw.seriesAdded++
		}
	}
}

func (sw *scrapeWork) finalizeSeriesAdded(lastScrapeSize int) int {
	seriesAdded := sw.seriesAdded
	sw.seriesAdded = 0
	if len(sw.seriesMap) > 4*lastScrapeSize {
		// Reset seriesMap, since it occupies more than 4x metrics collected during the last scrape.
		sw.seriesMap = make(map[uint64]struct{}, lastScrapeSize)
	}
	return seriesAdded
}

func (sw *scrapeWork) getLabelsHash(labels []prompbmarshal.Label) uint64 {
	// It is OK if there will be hash collisions for distinct sets of labels,
	// since the accuracy for `scrape_series_added` metric may be lower than 100%.
	b := sw.labelsHashBuf[:0]
	for _, label := range labels {
		b = append(b, label.Name...)
		b = append(b, label.Value...)
	}
	sw.labelsHashBuf = b
	return xxhash.Sum64(b)
}

// addAutoTimeseries adds automatically generated time series with the given name, value and timestamp.
//
// See https://prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series
func (sw *scrapeWork) addAutoTimeseries(wc *writeRequestCtx, name string, value float64, timestamp int64) {
	sw.tmpRow.Metric = name
	sw.tmpRow.Tags = nil
	sw.tmpRow.Value = value
	sw.tmpRow.Timestamp = timestamp
	sw.addRowToTimeseries(wc, &sw.tmpRow, timestamp, false)
}

func (sw *scrapeWork) addRowToTimeseries(wc *writeRequestCtx, r *parser.Row, timestamp int64, needRelabel bool) {
	labelsLen := len(wc.labels)
	wc.labels = appendLabels(wc.labels, r.Metric, r.Tags, sw.Config.Labels, sw.Config.HonorLabels)
	if needRelabel {
		wc.labels = promrelabel.ApplyRelabelConfigs(wc.labels, labelsLen, sw.Config.MetricRelabelConfigs, true)
	} else {
		wc.labels = promrelabel.FinalizeLabels(wc.labels[:labelsLen], wc.labels[labelsLen:])
		promrelabel.SortLabels(wc.labels[labelsLen:])
	}
	if len(wc.labels) == labelsLen {
		// Skip row without labels.
		return
	}
	sampleTimestamp := r.Timestamp
	if !sw.Config.HonorTimestamps || sampleTimestamp == 0 {
		sampleTimestamp = timestamp
	}
	wc.samples = append(wc.samples, prompbmarshal.Sample{
		Value:     r.Value,
		Timestamp: sampleTimestamp,
	})
	wr := &wc.writeRequest
	wr.Timeseries = append(wr.Timeseries, prompbmarshal.TimeSeries{
		Labels:  wc.labels[labelsLen:],
		Samples: wc.samples[len(wc.samples)-1:],
	})
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
