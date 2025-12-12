package promscrape

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/bits"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bloomfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/chunkedbuffer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/leveledbytebufferpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
	"github.com/cespare/xxhash/v2"
)

var (
	suppressScrapeErrors = flag.Bool("promscrape.suppressScrapeErrors", false, "Whether to suppress scrape errors logging. "+
		"The last error for each target is always available at '/targets' page even if scrape errors logging is suppressed. "+
		"See also -promscrape.suppressScrapeErrorsDelay")
	suppressScrapeErrorsDelay = flag.Duration("promscrape.suppressScrapeErrorsDelay", 0, "The delay for suppressing repeated scrape errors logging per each scrape targets. "+
		"This may be used for reducing the number of log lines related to scrape errors. See also -promscrape.suppressScrapeErrors")
	minResponseSizeForStreamParse = flagutil.NewBytes("promscrape.minResponseSizeForStreamParse", 1e6, "The minimum target response size for automatic switching to stream parsing mode, which can reduce memory usage. See https://docs.victoriametrics.com/victoriametrics/vmagent/#stream-parsing-mode")
)

// ScrapeWork represents a unit of work for scraping Prometheus metrics.
//
// It must be immutable during its lifetime, since it is read from concurrently running goroutines.
type ScrapeWork struct {
	// Full URL (including query args) for the scrape.
	ScrapeURL string

	// Interval for scraping the ScrapeURL.
	ScrapeInterval time.Duration

	// Timeout for scraping the ScrapeURL.
	ScrapeTimeout time.Duration

	// MaxScrapeSize sets max amount of data, that can be scraped by a job
	MaxScrapeSize int64

	// How to deal with conflicting labels.
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	HonorLabels bool

	// How to deal with scraped timestamps.
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	HonorTimestamps bool

	// Whether to deny redirects during requests to scrape config.
	DenyRedirects bool

	// Do not support enable_http2 option because of the following reasons:
	//
	// - http2 is used very rarely comparing to http for Prometheus metrics exposition and service discovery
	// - http2 is much harder to debug than http
	// - http2 has very bad security record because of its complexity - see https://portswigger.net/research/http2
	//
	// EnableHTTP2 bool

	// OriginalLabels contains original labels before relabeling.
	//
	// These labels are needed for relabeling troubleshooting at /targets page.
	//
	// OriginalLabels are sorted by name.
	OriginalLabels *promutil.Labels

	// Labels to add to the scraped metrics.
	//
	// The list contains at least the following labels according to https://www.robustperception.io/life-of-a-label/
	//
	//     * job
	//     * instance
	//     * user-defined labels set via `relabel_configs` section in `scrape_config`
	//
	// See also https://prometheus.io/docs/concepts/jobs_instances/
	//
	// Labels are sorted by name.
	Labels *promutil.Labels

	// ExternalLabels contains labels from global->external_labels section of -promscrape.config
	//
	// These labels are added to scraped metrics after the relabeling.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3137
	//
	// ExternalLabels are sorted by name.
	ExternalLabels *promutil.Labels

	// ProxyURL HTTP proxy url
	ProxyURL *proxy.URL

	// Auth config for ProxyUR:
	ProxyAuthConfig *promauth.Config

	// Auth config
	AuthConfig *promauth.Config

	// Optional `relabel_configs`.
	RelabelConfigs *promrelabel.ParsedConfigs

	// Optional `metric_relabel_configs`.
	MetricRelabelConfigs *promrelabel.ParsedConfigs

	// The maximum number of metrics to scrape after relabeling.
	SampleLimit int

	// Whether to disable response compression when querying ScrapeURL.
	DisableCompression bool

	// Whether to disable HTTP keep-alive when querying ScrapeURL.
	DisableKeepAlive bool

	// Whether to parse target responses in a streaming manner.
	StreamParse bool

	// The interval for aligning the first scrape.
	ScrapeAlignInterval time.Duration

	// The offset for the first scrape.
	ScrapeOffset time.Duration

	// Optional limit on the number of unique series the scrape target can expose.
	SeriesLimit int32

	// Optional limit on the number of allowed labels per series.
	LabelLimit int

	// Whether to process stale markers for the given target.
	// See https://docs.victoriametrics.com/victoriametrics/vmagent/#prometheus-staleness-markers
	NoStaleMarkers bool

	// The Tenant Info
	AuthToken *auth.Token

	// The original 'job_name'
	jobNameOriginal string
}

func (sw *ScrapeWork) canSwitchToStreamParseMode() bool {
	// Deny switching to stream parse mode if `sample_limit` or `series_limit` options are set,
	// since these limits cannot be applied in stream parsing mode.
	return sw.SampleLimit <= 0 && sw.SeriesLimit <= 0
}

// key returns unique identifier for the given sw.
//
// It can be used for comparing for equality for two ScrapeWork objects.
func (sw *ScrapeWork) key() string {
	// Do not take into account OriginalLabels, since they can be changed with relabeling.
	// Do not take into account RelabelConfigs, since it is already applied to Labels.
	// Take into account JobNameOriginal in order to capture the case when the original job_name is changed via relabeling.
	key := fmt.Sprintf("JobNameOriginal=%s, ScrapeURL=%s, ScrapeInterval=%s, ScrapeTimeout=%s, HonorLabels=%v, "+
		"HonorTimestamps=%v, DenyRedirects=%v, Labels=%s, ExternalLabels=%s, MaxScrapeSize=%d, "+
		"ProxyURL=%s, ProxyAuthConfig=%s, AuthConfig=%s, MetricRelabelConfigs=%q, "+
		"SampleLimit=%d, DisableCompression=%v, DisableKeepAlive=%v, StreamParse=%v, "+
		"ScrapeAlignInterval=%s, ScrapeOffset=%s, SeriesLimit=%d, LabelLimit=%d, NoStaleMarkers=%v",
		sw.jobNameOriginal, sw.ScrapeURL, sw.ScrapeInterval, sw.ScrapeTimeout, sw.HonorLabels,
		sw.HonorTimestamps, sw.DenyRedirects, sw.Labels.String(), sw.ExternalLabels.String(), sw.MaxScrapeSize,
		sw.ProxyURL.String(), sw.ProxyAuthConfig.String(), sw.AuthConfig.String(), sw.MetricRelabelConfigs.String(),
		sw.SampleLimit, sw.DisableCompression, sw.DisableKeepAlive, sw.StreamParse,
		sw.ScrapeAlignInterval, sw.ScrapeOffset, sw.SeriesLimit, sw.LabelLimit, sw.NoStaleMarkers)
	return key
}

// Job returns job for the ScrapeWork
func (sw *ScrapeWork) Job() string {
	return sw.Labels.Get("job")
}

type scrapeWork struct {
	// Config for the scrape.
	Config *ScrapeWork

	// ReadData is called for reading the scrape response data into dst.
	ReadData func(dst *chunkedbuffer.Buffer) (bool, error)

	// PushData is called for pushing collected data.
	//
	// The PushData must be safe for calling from multiple concurrent goroutines.
	PushData func(at *auth.Token, wr *prompb.WriteRequest)

	// ScrapeGroup is name of ScrapeGroup that
	// scrapeWork belongs to
	ScrapeGroup string

	// This flag is set to true if series_limit is exceeded.
	seriesLimitExceeded atomic.Bool

	// Optional limiter on the number of unique series per scrape target.
	seriesLimiter     *bloomfilter.Limiter
	seriesLimiterOnce sync.Once

	// prevBodyLen contains the previous response body length for the given scrape work.
	// It is used as a hint in order to reduce memory usage for body buffers.
	prevBodyLen int

	// autoMetricsLabelsLen contains the number of automatically generated labels during the previous stream parsing scrape.
	// It is used as a hint in order to reduce memory usage when generating auto metrics in stream parsing mode.
	autoMetricsLabelsLen int

	// prevLabelsLen contains the number labels scraped during the previous scrape.
	// It is used as a hint in order to reduce memory usage when parsing scrape responses.
	prevLabelsLen int

	// lastScrapeCompressed holds the last response from scrape target in the compressed form.
	// It is used for staleness tracking and for populating scrape_series_added metric.
	// The lastScrapeCompressed isn't populated if -promscrape.noStaleMarkers is set. This reduces memory usage.
	lastScrapeCompressed []byte

	// lastScrapeLen contains the length of the last response from scrape target.
	// It is used as a hint in order to reduce memory usage when working with the last scraped response.
	lastScrapeLen int

	// nextErrorLogTime is the timestamp in millisecond when the next scrape error should be logged.
	nextErrorLogTime int64

	// failureRequestsCount is the number of suppressed scrape errors during the last suppressScrapeErrorsDelay
	failureRequestsCount int

	// successRequestsCount is the number of success requests during the last suppressScrapeErrorsDelay
	successRequestsCount int
}

// loadLastScrape appends last scrape response to dst and returns the result.
func (sw *scrapeWork) loadLastScrape(dst []byte) []byte {
	if len(sw.lastScrapeCompressed) == 0 {
		// Nothing to decompress.
		return dst
	}
	b, err := encoding.DecompressZSTD(dst, sw.lastScrapeCompressed)
	if err != nil {
		logger.Panicf("BUG: cannot unpack compressed previous response: %s", err)
	}
	return b
}

func (sw *scrapeWork) storeLastScrape(lastScrapeStr string) {
	if lastScrapeStr == "" {
		sw.lastScrapeCompressed = sw.lastScrapeCompressed[:0]
	} else {
		lastScrape := bytesutil.ToUnsafeBytes(lastScrapeStr)
		sw.lastScrapeCompressed = encoding.CompressZSTDLevel(sw.lastScrapeCompressed[:0], lastScrape, 1)
	}
}

func (sw *scrapeWork) run(stopCh <-chan struct{}, globalStopCh <-chan struct{}) {
	var randSleep uint64
	scrapeInterval := sw.Config.ScrapeInterval
	scrapeAlignInterval := sw.Config.ScrapeAlignInterval
	scrapeOffset := sw.Config.ScrapeOffset
	if scrapeOffset > 0 {
		scrapeAlignInterval = scrapeInterval
	}
	if scrapeAlignInterval <= 0 {
		// Calculate start time for the first scrape from ScrapeURL and labels.
		// This should spread load when scraping many targets with different
		// scrape urls and labels.
		// This also makes consistent scrape times across restarts
		// for a target with the same ScrapeURL and labels.
		//
		// Include clusterName to the key in order to guarantee that the same
		// scrape target is scraped at different offsets per each cluster.
		// This guarantees that the deduplication consistently leaves samples received from the same vmagent.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2679
		//
		// Include clusterMemberID to the key in order to guarantee that each member in vmagent cluster
		// scrapes replicated targets at different time offsets. This guarantees that the deduplication consistently leaves samples
		// received from the same vmagent replica.
		// See https://docs.victoriametrics.com/victoriametrics/vmagent/#scraping-big-number-of-targets
		key := fmt.Sprintf("clusterName=%s, clusterMemberID=%d, ScrapeURL=%s, Labels=%s", *clusterName, clusterMemberID, sw.Config.ScrapeURL, sw.Config.Labels.String())
		h := xxhash.Sum64(bytesutil.ToUnsafeBytes(key))
		randSleep = uint64(float64(scrapeInterval) * (float64(h) / (1 << 64)))
		sleepOffset := uint64(time.Now().UnixNano()) % uint64(scrapeInterval)
		if randSleep < sleepOffset {
			randSleep += uint64(scrapeInterval)
		}
		randSleep -= sleepOffset
	} else {
		d := uint64(scrapeAlignInterval)
		randSleep = d - uint64(time.Now().UnixNano())%d
		if scrapeOffset > 0 {
			randSleep += uint64(scrapeOffset)
		}
		randSleep %= uint64(scrapeInterval)
	}
	timer := timerpool.Get(time.Duration(randSleep))
	var timestamp int64
	var ticker *time.Ticker
	select {
	case <-stopCh:
		timerpool.Put(timer)
		return
	case <-timer.C:
		timerpool.Put(timer)
		ticker = time.NewTicker(scrapeInterval)
		timestamp = time.Now().UnixMilli()
		sw.scrapeAndLogError(timestamp, timestamp)
	}
	defer ticker.Stop()
	for {
		timestamp += scrapeInterval.Milliseconds()
		select {
		case <-stopCh:
			t := time.Now().UnixMilli()
			select {
			case <-globalStopCh:
				// Do not send staleness markers on graceful shutdown as Prometheus does.
				// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2013#issuecomment-1006994079
			default:
				// The code below is CPU-bound, while it may allocate big amounts of memory.
				// That's why it is a good idea to limit the number of concurrent goroutines,
				// which may execute this code, in order to limit memory usage under high load
				// without sacrificing the performance.
				processScrapedDataConcurrencyLimitCh <- struct{}{}

				// Send staleness markers to all the metrics scraped last time from the target
				// when the given target disappears as Prometheus does.
				// Use the current real timestamp for staleness markers, so queries
				// stop returning data just after the time the target disappears.
				bbLastScrape := leveledbytebufferpool.Get(sw.lastScrapeLen)
				bbLastScrape.B = sw.loadLastScrape(bbLastScrape.B)
				lastScrapeStr := bytesutil.ToUnsafeString(bbLastScrape.B)
				sw.sendStaleSeries(lastScrapeStr, "", t, true)
				leveledbytebufferpool.Put(bbLastScrape)

				<-processScrapedDataConcurrencyLimitCh
			}
			if sl := sw.getSeriesLimiter(); sl != nil {
				sl.MustStop()
				sw.seriesLimiter = nil
			}
			return
		case tt := <-ticker.C:
			t := tt.UnixMilli()
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
		logger.ErrorfSkipframes(1, "error when scraping %q from job %q with labels %s: %s; "+
			"scrape errors can be disabled by -promscrape.suppressScrapeErrors command-line flag",
			sw.Config.ScrapeURL, sw.Config.Job(), sw.Config.Labels.String(), s)
	}
}

func (sw *scrapeWork) scrapeAndLogError(scrapeTimestamp, realTimestamp int64) {
	err := sw.scrapeInternal(scrapeTimestamp, realTimestamp)
	if *suppressScrapeErrors {
		return
	}
	if err == nil {
		sw.successRequestsCount++
		return
	}
	sw.failureRequestsCount++
	if sw.nextErrorLogTime == 0 {
		sw.nextErrorLogTime = realTimestamp + suppressScrapeErrorsDelay.Milliseconds()
	}
	if realTimestamp < sw.nextErrorLogTime {
		return
	}
	totalRequests := sw.failureRequestsCount + sw.successRequestsCount
	if !errors.Is(err, context.Canceled) {
		logger.Warnf("cannot scrape target %q (%s) %d out of %d times during -promscrape.suppressScrapeErrorsDelay=%s; the last error: %s",
			sw.Config.ScrapeURL, sw.Config.Labels.String(), sw.failureRequestsCount, totalRequests, *suppressScrapeErrorsDelay, err)
	}
	sw.nextErrorLogTime = realTimestamp + suppressScrapeErrorsDelay.Milliseconds()
	sw.failureRequestsCount = 0
	sw.successRequestsCount = 0
}

var (
	scrapeDuration              = metrics.NewHistogram("vm_promscrape_scrape_duration_seconds")
	scrapeResponseSize          = metrics.NewHistogram("vm_promscrape_scrape_response_size_bytes")
	scrapedSamples              = metrics.NewHistogram("vm_promscrape_scraped_samples")
	scrapesSkippedBySampleLimit = metrics.NewCounter("vm_promscrape_scrapes_skipped_by_sample_limit_total")
	scrapesSkippedByLabelLimit  = metrics.NewCounter("vm_promscrape_scrapes_skipped_by_label_limit_total")
	scrapesFailed               = metrics.NewCounter("vm_promscrape_scrapes_failed_total")
	pushDataDuration            = metrics.NewHistogram("vm_promscrape_push_data_duration_seconds")
)

func (sw *scrapeWork) needStreamParseMode(responseSize int) bool {
	if *streamParse || sw.Config.StreamParse {
		return true
	}
	if minResponseSizeForStreamParse.N <= 0 {
		return false
	}
	return sw.Config.canSwitchToStreamParseMode() && responseSize >= minResponseSizeForStreamParse.IntN()
}

// getTargetResponse() fetches response from sw target in the same way as when scraping the target.
func (sw *scrapeWork) getTargetResponse() ([]byte, error) {
	cb := chunkedbuffer.Get()
	defer chunkedbuffer.Put(cb)

	isGzipped, err := sw.ReadData(cb)
	if err != nil {
		return nil, err
	}

	var bb bytesutil.ByteBuffer
	err = readFromBuffer(&bb, cb, isGzipped)
	return bb.B, err
}

func (sw *scrapeWork) scrapeInternal(scrapeTimestamp, realTimestamp int64) error {
	// Read the whole scrape response into cb.
	// It is OK to do this for stream parsing mode, since the most of RAM
	// is occupied during parsing of the read response body below.
	// This also allows measuring the real scrape duration, which doesn't include
	// the time needed for processing of the read response.
	cb := chunkedbuffer.Get()
	isGzipped, err := sw.ReadData(cb)

	// Measure scrape duration.
	endTimestamp := time.Now().UnixMilli()
	scrapeDurationSeconds := float64(endTimestamp-realTimestamp) / 1e3
	scrapeDuration.Update(scrapeDurationSeconds)

	// The code below is CPU-bound, while it may allocate big amounts of memory.
	// That's why it is a good idea to limit the number of concurrent goroutines,
	// which may execute this code, in order to limit memory usage under high load
	// without sacrificing the performance.
	processScrapedDataConcurrencyLimitCh <- struct{}{}

	// Copy the read scrape response to body in order to parse it and send
	// the parsed results to remote storage.
	body := leveledbytebufferpool.Get(sw.prevBodyLen)
	if err == nil {
		err = readFromBuffer(body, cb, isGzipped)
	}
	chunkedbuffer.Put(cb)

	bodyLen := len(body.B)
	sw.prevBodyLen = bodyLen
	scrapeResponseSize.Update(float64(bodyLen))

	if err == nil && sw.needStreamParseMode(bodyLen) {
		// Process response body from scrape target in streaming manner.
		// This case is optimized for targets exposing more than ten thousand of metrics per target,
		// such as kube-state-metrics.
		err = sw.processDataInStreamMode(scrapeTimestamp, realTimestamp, body, scrapeDurationSeconds)
	} else {
		// Process response body from scrape target at once.
		// This case should work more optimally than stream parse for common case when scrape target exposes
		// up to a few thousand metrics.
		err = sw.processDataOneShot(scrapeTimestamp, realTimestamp, body.B, scrapeDurationSeconds, err)
	}

	leveledbytebufferpool.Put(body)

	<-processScrapedDataConcurrencyLimitCh

	return err
}

var processScrapedDataConcurrencyLimitCh = make(chan struct{}, cgroup.AvailableCPUs())

func readFromBuffer(dst *bytesutil.ByteBuffer, src *chunkedbuffer.Buffer, isGzipped bool) error {
	if !isGzipped {
		src.MustWriteTo(dst)
		return nil
	}

	reader, err := protoparserutil.GetUncompressedReader(src.NewReader(), "gzip")
	if err != nil {
		return fmt.Errorf("cannot decompress response body: %w", err)
	}
	_, err = dst.ReadFrom(reader)
	protoparserutil.PutUncompressedReader(reader)
	if err != nil {
		return fmt.Errorf("cannot read gzipped response body: %w", err)
	}
	return nil
}

func (sw *scrapeWork) processDataOneShot(scrapeTimestamp, realTimestamp int64, body []byte, scrapeDurationSeconds float64, err error) error {
	up := 1

	bbLastScrape := leveledbytebufferpool.Get(sw.lastScrapeLen)
	bbLastScrape.B = sw.loadLastScrape(bbLastScrape.B)
	lastScrapeStr := bytesutil.ToUnsafeString(bbLastScrape.B)

	bodyString := bytesutil.ToUnsafeString(body)
	cfg := sw.Config
	areIdenticalSeries := areIdenticalSeries(cfg, lastScrapeStr, bodyString)

	wc := writeRequestCtxPool.Get(sw.prevLabelsLen)
	if err != nil {
		up = 0
		scrapesFailed.Inc()
	} else {
		if prommetadata.IsEnabled() {
			wc.rows, wc.metadataRows = parser.UnmarshalWithMetadata(wc.rows, wc.metadataRows, bodyString, sw.logError)
		} else {
			wc.rows.UnmarshalWithErrLogger(bodyString, sw.logError)
		}
	}
	samplesPostRelabeling := 0
	samplesScraped := len(wc.rows.Rows)
	scrapedSamples.Update(float64(samplesScraped))
	wc.addMetadata(wc.metadataRows.Rows)
	scrapeErr := wc.addRows(cfg, wc.rows.Rows, scrapeTimestamp, true)
	if scrapeErr == nil {
		samplesPostRelabeling = len(wc.writeRequest.Timeseries)
		if cfg.SampleLimit > 0 && samplesPostRelabeling > cfg.SampleLimit {
			scrapesSkippedBySampleLimit.Inc()
			scrapeErr = fmt.Errorf("the response from %q exceeds sample_limit=%d; "+
				"either reduce the sample count for the target or increase sample_limit", cfg.ScrapeURL, cfg.SampleLimit)
		}
	}
	if scrapeErr != nil {
		if errors.Is(err, errLabelsLimitExceeded) {
			scrapesSkippedByLabelLimit.Inc()
			scrapeErr = fmt.Errorf("the response from %q contains samples with a number of labels exceeding label_limit=%d; "+
				"either reduce the labels count for the target or increase label_limit", cfg.ScrapeURL, cfg.LabelLimit)
		}
		err = scrapeErr
		// use wc.writeRequest.Reset() instead of wc.reset()
		// in order to keep the len(wc.labels), which is used for initializing sw.prevLabelsLen below.
		//
		// This prevents from excess memory allocations at wc for the next scrape for the given scrapeWork.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/commit/12f26668a6bbf804db0e41a2683f1bd232e985d6#r155481168
		wc.writeRequest.Reset()
		up = 0
	}

	if up == 0 {
		bodyString = ""
	}
	seriesAdded := 0
	if !areIdenticalSeries {
		// The returned value for seriesAdded may be bigger than the real number of added series
		// if some series were removed during relabeling.
		// This is a trade-off between performance and accuracy.
		seriesAdded = getSeriesAdded(lastScrapeStr, bodyString)
	}
	samplesDropped := 0
	if sw.seriesLimitExceeded.Load() || !areIdenticalSeries {
		samplesDropped = wc.applySeriesLimit(sw)
	}
	responseSize := len(bodyString)

	am := &autoMetrics{
		up:                        up,
		scrapeDurationSeconds:     scrapeDurationSeconds,
		scrapeResponseSize:        responseSize,
		samplesScraped:            samplesScraped,
		samplesPostRelabeling:     samplesPostRelabeling,
		seriesAdded:               seriesAdded,
		seriesLimitSamplesDropped: samplesDropped,
	}
	wc.addAutoMetrics(sw, am, scrapeTimestamp)

	sw.pushData(&wc.writeRequest)
	sw.prevLabelsLen = len(wc.labels)
	writeRequestCtxPool.Put(wc)

	if !areIdenticalSeries {
		// Send stale markers for disappeared metrics with the real scrape timestamp
		// in order to guarantee that query doesn't return data after this time for the disappeared metrics.
		sw.sendStaleSeries(lastScrapeStr, bodyString, realTimestamp, false)
		sw.storeLastScrape(bodyString)
		sw.lastScrapeLen = len(bodyString)
	}
	leveledbytebufferpool.Put(bbLastScrape)

	tsmGlobal.Update(sw, up == 1, realTimestamp, int64(scrapeDurationSeconds*1000), responseSize, samplesScraped, err)
	return err
}

func (sw *scrapeWork) processDataInStreamMode(scrapeTimestamp, realTimestamp int64, body *bytesutil.ByteBuffer, scrapeDurationSeconds float64) error {
	var samplesScraped atomic.Int64
	var samplesPostRelabeling atomic.Int64
	var samplesDroppedTotal atomic.Int64
	var maxLabelsLen atomic.Int64

	maxLabelsLen.Store(int64(sw.prevLabelsLen))

	bbLastScrape := leveledbytebufferpool.Get(sw.lastScrapeLen)
	bbLastScrape.B = sw.loadLastScrape(bbLastScrape.B)
	lastScrapeStr := bytesutil.ToUnsafeString(bbLastScrape.B)

	bodyString := bytesutil.ToUnsafeString(body.B)
	cfg := sw.Config
	areIdenticalSeries := areIdenticalSeries(cfg, lastScrapeStr, bodyString)

	r := body.NewReader()
	err := stream.Parse(r, scrapeTimestamp, "", false, prommetadata.IsEnabled(), func(rows []parser.Row, mms []parser.Metadata) error {
		labelsLen := maxLabelsLen.Load()
		wc := writeRequestCtxPool.Get(int(labelsLen))
		defer func() {
			newLabelsLen := len(wc.labels)
			for {
				n := maxLabelsLen.Load()
				if int64(newLabelsLen) <= n {
					break
				}
				if maxLabelsLen.CompareAndSwap(n, int64(newLabelsLen)) {
					break
				}
			}
			writeRequestCtxPool.Put(wc)
		}()

		samplesScraped.Add(int64(len(rows)))
		if err := wc.addRows(cfg, rows, scrapeTimestamp, true); err != nil {
			if errors.Is(err, errLabelsLimitExceeded) {
				scrapesSkippedByLabelLimit.Inc()
				return fmt.Errorf("the response from %q contains samples with a number of labels exceeding label_limit=%d; "+
					"either reduce the labels count for the target or increase label_limit", cfg.ScrapeURL, cfg.LabelLimit)
			}
			return err
		}
		n := samplesPostRelabeling.Add(int64(len(wc.writeRequest.Timeseries)))
		if cfg.SampleLimit > 0 && int(n) > cfg.SampleLimit {
			scrapesSkippedBySampleLimit.Inc()
			return fmt.Errorf("the response from %q exceeds sample_limit=%d; "+
				"either reduce the sample count for the target or increase sample_limit", cfg.ScrapeURL, cfg.SampleLimit)
		}
		wc.addMetadata(mms)

		if sw.seriesLimitExceeded.Load() || !areIdenticalSeries {
			samplesDropped := wc.applySeriesLimit(sw)
			samplesDroppedTotal.Add(int64(samplesDropped))
		}

		sw.pushData(&wc.writeRequest)
		return nil
	}, sw.logError)

	sw.prevLabelsLen = int(maxLabelsLen.Load())
	scrapedSamples.Update(float64(samplesScraped.Load()))
	up := 1
	if err != nil {
		// Mark the scrape as failed even if it already read and pushed some samples
		// to remote storage. This makes the logic compatible with Prometheus.
		up = 0
		bodyString = ""
		scrapesFailed.Inc()
	}
	seriesAdded := 0
	if !areIdenticalSeries {
		// The returned value for seriesAdded may be bigger than the real number of added series
		// if some series were removed during relabeling.
		// This is a trade-off between performance and accuracy.
		seriesAdded = getSeriesAdded(lastScrapeStr, bodyString)
	}
	responseSize := len(bodyString)

	am := &autoMetrics{
		up:                        up,
		scrapeDurationSeconds:     scrapeDurationSeconds,
		scrapeResponseSize:        responseSize,
		samplesScraped:            int(samplesScraped.Load()),
		samplesPostRelabeling:     int(samplesPostRelabeling.Load()),
		seriesAdded:               seriesAdded,
		seriesLimitSamplesDropped: int(samplesDroppedTotal.Load()),
	}
	sw.pushAutoMetrics(am, scrapeTimestamp)

	if !areIdenticalSeries {
		// Send stale markers for disappeared metrics with the real scrape timestamp
		// in order to guarantee that query doesn't return data after this time for the disappeared metrics.
		sw.sendStaleSeries(lastScrapeStr, bodyString, realTimestamp, false)
		sw.storeLastScrape(bodyString)
		sw.lastScrapeLen = len(bodyString)
	}
	leveledbytebufferpool.Put(bbLastScrape)

	tsmGlobal.Update(sw, up == 1, realTimestamp, int64(scrapeDurationSeconds*1000), responseSize, int(samplesScraped.Load()), err)
	// Do not track active series in streaming mode, since this may need too big amounts of memory
	// when the target exports too big number of metrics.
	return err
}

// pushAutoMetrics pushes am with the given timestamp to sw.
func (sw *scrapeWork) pushAutoMetrics(am *autoMetrics, timestamp int64) {
	wc := writeRequestCtxPool.Get(sw.autoMetricsLabelsLen)
	wc.addAutoMetrics(sw, am, timestamp)
	sw.pushData(&wc.writeRequest)
	sw.autoMetricsLabelsLen = len(wc.labels)
	writeRequestCtxPool.Put(wc)
}

// pushData sends wr to the remote storage.
//
// sw is used as a read-only configuration source.
func (sw *scrapeWork) pushData(wr *prompb.WriteRequest) {
	startTime := time.Now()
	sw.PushData(sw.Config.AuthToken, wr)
	pushDataDuration.UpdateDuration(startTime)
}

func areIdenticalSeries(cfg *ScrapeWork, prevData, currData string) bool {
	if cfg.NoStaleMarkers && cfg.SeriesLimit <= 0 {
		// Do not spend CPU time on tracking the changes in series if stale markers are disabled.
		// The check for series_limit is needed for https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3660
		return true
	}
	return parser.AreIdenticalSeriesFast(prevData, currData)
}

// leveledWriteRequestCtxPool allows reducing memory usage when writeRequestCtx
// structs contain mixed number of labels.
//
// Its logic has been copied from leveledbytebufferpool.
type leveledWriteRequestCtxPool struct {
	pools [8]sync.Pool
}

func (lwp *leveledWriteRequestCtxPool) Get(labelsLen int) *writeRequestCtx {
	id, _ := lwp.getPoolIDAndCapacity(labelsLen)
	for i := 0; i < 2; i++ {
		if id < 0 || id >= len(lwp.pools) {
			break
		}
		if v := lwp.pools[id].Get(); v != nil {
			return v.(*writeRequestCtx)
		}
		id++
	}

	return &writeRequestCtx{}
}

func (lwp *leveledWriteRequestCtxPool) Put(wc *writeRequestCtx) {
	capacity := cap(wc.labels)
	id, poolCapacity := lwp.getPoolIDAndCapacity(capacity)
	if capacity <= poolCapacity {
		wc.reset()
		lwp.pools[id].Put(wc)
	}
}

func (lwp *leveledWriteRequestCtxPool) getPoolIDAndCapacity(size int) (int, int) {
	size--
	if size < 0 {
		size = 0
	}
	size >>= 8
	id := bits.Len(uint(size))
	if id >= len(lwp.pools) {
		id = len(lwp.pools) - 1
	}
	return id, (1 << (id + 8))
}

type writeRequestCtx struct {
	rows         parser.Rows
	metadataRows parser.MetadataRows

	writeRequest prompb.WriteRequest
	labels       []prompb.Label
	samples      []prompb.Sample
}

func (wc *writeRequestCtx) reset() {
	wc.rows.Reset()
	wc.metadataRows.Reset()

	wc.writeRequest.Reset()

	clear(wc.labels)
	wc.labels = wc.labels[:0]

	wc.samples = wc.samples[:0]
}

var writeRequestCtxPool leveledWriteRequestCtxPool

func getSeriesAdded(lastScrape, currScrape string) int {
	if currScrape == "" {
		return 0
	}
	bodyString := parser.GetRowsDiff(currScrape, lastScrape)
	return strings.Count(bodyString, "\n")
}

func (sw *scrapeWork) initSeriesLimiter() {
	if sw.Config.SeriesLimit > 0 {
		sw.seriesLimiter = bloomfilter.NewLimiter(sw.Config.SeriesLimit, 24*time.Hour)
	}
}

func (sw *scrapeWork) getSeriesLimiter() *bloomfilter.Limiter {
	sw.seriesLimiterOnce.Do(sw.initSeriesLimiter)
	return sw.seriesLimiter
}

// applySeriesLimit enforces the series limit on wc according to sw.Config.SeriesLimit
//
// If the series limit is exceeded, then sw.seriesLimitExceeded is set to true.
//
// Returns the number of dropped time series due to the limit.
func (wc *writeRequestCtx) applySeriesLimit(sw *scrapeWork) int {
	sl := sw.getSeriesLimiter()
	if sl == nil {
		return 0
	}
	dstSeries := wc.writeRequest.Timeseries[:0]
	samplesDropped := 0
	for _, ts := range wc.writeRequest.Timeseries {
		h := getLabelsHash(ts.Labels)
		if !sl.Add(h) {
			samplesDropped++
			continue
		}
		dstSeries = append(dstSeries, ts)
	}
	clear(wc.writeRequest.Timeseries[len(dstSeries):])
	wc.writeRequest.Timeseries = dstSeries
	if samplesDropped > 0 {
		sw.seriesLimitExceeded.Store(true)
	}
	return samplesDropped
}

var sendStaleSeriesConcurrencyLimitCh = make(chan struct{}, cgroup.AvailableCPUs())

func (sw *scrapeWork) sendStaleSeries(lastScrape, currScrape string, timestamp int64, addAutoSeries bool) {
	// This function is CPU-bound, while it may allocate big amounts of memory.
	// That's why it is a good idea to limit the number of concurrent calls to this function
	// in order to limit memory usage under high load without sacrificing the performance.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3668
	sendStaleSeriesConcurrencyLimitCh <- struct{}{}
	defer func() {
		<-sendStaleSeriesConcurrencyLimitCh
	}()
	if sw.Config.NoStaleMarkers {
		return
	}
	bodyString := lastScrape
	if currScrape != "" {
		bodyString = parser.GetRowsDiff(lastScrape, currScrape)
	}
	if bodyString != "" {
		// Send stale markers in streaming mode in order to reduce memory usage
		// when stale markers for targets exposing big number of metrics must be generated.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3668
		// and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3675
		br := bytes.NewBufferString(bodyString)
		err := stream.Parse(br, timestamp, "", false, false, func(rows []parser.Row, _ []parser.Metadata) error {
			wc := writeRequestCtxPool.Get(sw.prevLabelsLen)
			defer writeRequestCtxPool.Put(wc)

			if err := wc.addRows(sw.Config, rows, timestamp, true); err != nil {
				sw.logError(fmt.Errorf("cannot send stale markers: %w", err).Error())
				return err
			}

			// Apply series limit to stale markers in order to prevent sending stale markers for newly created series.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3660
			if sw.seriesLimitExceeded.Load() {
				wc.applySeriesLimit(sw)
			}

			setStaleMarkersForRows(wc.writeRequest.Timeseries)
			sw.pushData(&wc.writeRequest)
			return nil
		}, sw.logError)
		if err != nil {
			sw.logError(fmt.Errorf("cannot send stale markers: %w", err).Error())
		}
	}
	if addAutoSeries {
		var wc writeRequestCtx
		var am autoMetrics
		wc.addAutoMetrics(sw, &am, timestamp)
		setStaleMarkersForRows(wc.writeRequest.Timeseries)
		sw.pushData(&wc.writeRequest)
	}
}

func setStaleMarkersForRows(series []prompb.TimeSeries) {
	for _, tss := range series {
		samples := tss.Samples
		for i := range samples {
			samples[i].Value = decimal.StaleNaN
		}
		staleSamplesCreated.Add(len(samples))
	}
}

var staleSamplesCreated = metrics.NewCounter(`vm_promscrape_stale_samples_created_total`)

var labelsHashBufferPool = &bytesutil.ByteBufferPool{}

func getLabelsHash(labels []prompb.Label) uint64 {
	// It is OK if there will be hash collisions for distinct sets of labels,
	// since the accuracy for `scrape_series_added` metric may be lower than 100%.

	bb := labelsHashBufferPool.Get()
	b := bb.B

	for _, label := range labels {
		b = append(b, label.Name...)
		b = append(b, label.Value...)
	}
	h := xxhash.Sum64(b)

	bb.B = b
	labelsHashBufferPool.Put(bb)

	return h
}

type autoMetrics struct {
	up                        int
	scrapeDurationSeconds     float64
	scrapeResponseSize        int
	samplesScraped            int
	samplesPostRelabeling     int
	seriesAdded               int
	seriesLimitSamplesDropped int
}

func isAutoMetric(s string) bool {
	if s == "up" {
		return true
	}
	if !strings.HasPrefix(s, "scrape_") {
		return false
	}
	switch s {
	case "scrape_duration_seconds",
		"scrape_response_size_bytes",
		"scrape_samples_limit",
		"scrape_samples_post_metric_relabeling",
		"scrape_samples_scraped",
		"scrape_series_added",
		"scrape_series_current",
		"scrape_series_limit",
		"scrape_series_limit_samples_dropped",
		"scrape_timeout_seconds":
		return true
	default:
		return false
	}
}

// addAutoMetrics adds am with the given timestamp to wc.
//
// See https://docs.victoriametrics.com/victoriametrics/vmagent/#automatically-generated-metrics
//
// sw is used as read-only config source.
func (wc *writeRequestCtx) addAutoMetrics(sw *scrapeWork, am *autoMetrics, timestamp int64) {
	rows := getAutoRows()
	dst := slicesutil.SetLength(rows.Rows, 11)[:0]

	dst = appendRow(dst, "scrape_duration_seconds", am.scrapeDurationSeconds, timestamp)
	dst = appendRow(dst, "scrape_response_size_bytes", float64(am.scrapeResponseSize), timestamp)

	if sampleLimit := sw.Config.SampleLimit; sampleLimit > 0 {
		// Expose scrape_samples_limit metric if sample_limit config is set for the target.
		// See https://github.com/VictoriaMetrics/operator/issues/497
		dst = appendRow(dst, "scrape_samples_limit", float64(sampleLimit), timestamp)
	}
	dst = appendRow(dst, "scrape_samples_post_metric_relabeling", float64(am.samplesPostRelabeling), timestamp)
	dst = appendRow(dst, "scrape_samples_scraped", float64(am.samplesScraped), timestamp)
	dst = appendRow(dst, "scrape_series_added", float64(am.seriesAdded), timestamp)
	if sl := sw.getSeriesLimiter(); sl != nil {
		dst = appendRow(dst, "scrape_series_current", float64(sl.CurrentItems()), timestamp)
		dst = appendRow(dst, "scrape_series_limit_samples_dropped", float64(am.seriesLimitSamplesDropped), timestamp)
		dst = appendRow(dst, "scrape_series_limit", float64(sl.MaxItems()), timestamp)
	}
	if labelLimit := sw.Config.LabelLimit; labelLimit > 0 {
		dst = appendRow(dst, "scrape_labels_limit", float64(labelLimit), timestamp)
	}
	dst = appendRow(dst, "scrape_timeout_seconds", sw.Config.ScrapeTimeout.Seconds(), timestamp)
	dst = appendRow(dst, "up", float64(am.up), timestamp)

	err := wc.addRows(sw.Config, dst, timestamp, false)
	if err != nil {
		sw.logError(fmt.Errorf("cannot add auto metrics: %w", err).Error())
	}

	rows.Rows = dst
	putAutoRows(rows)
}

func getAutoRows() *parser.Rows {
	v := autoRowsPool.Get()
	if v == nil {
		return &parser.Rows{}
	}
	return v.(*parser.Rows)
}

func putAutoRows(rows *parser.Rows) {
	rows.Reset()
	autoRowsPool.Put(rows)
}

var autoRowsPool sync.Pool

func appendRow(dst []parser.Row, metric string, value float64, timestamp int64) []parser.Row {
	return append(dst, parser.Row{
		Metric:    metric,
		Value:     value,
		Timestamp: timestamp,
	})
}

func (wc *writeRequestCtx) addRows(cfg *ScrapeWork, rows []parser.Row, timestamp int64, needRelabel bool) error {
	// pre-allocate buffers
	labelsLen := 0
	for i := range rows {
		labelsLen += len(rows[i].Tags) + 1
	}
	labelsLenPrev := len(wc.labels)
	wc.labels = slicesutil.SetLength(wc.labels, labelsLenPrev+labelsLen)[:labelsLenPrev]

	samplesLenPrev := len(wc.samples)
	wc.samples = slicesutil.SetLength(wc.samples, samplesLenPrev+len(rows))[:samplesLenPrev]

	timeseriesLenPrev := len(wc.writeRequest.Timeseries)
	wc.writeRequest.Timeseries = slicesutil.SetLength(wc.writeRequest.Timeseries, timeseriesLenPrev+len(rows))[:timeseriesLenPrev]

	// add rows
	for i := range rows {
		err := wc.addRow(cfg, &rows[i], timestamp, needRelabel)
		if err != nil {
			return err
		}
	}
	return nil
}

var errLabelsLimitExceeded = errors.New("label_limit exceeded")

func (wc *writeRequestCtx) addMetadata(metadataRows []parser.Metadata) {
	if len(metadataRows) == 0 {
		return
	}
	mms := wc.writeRequest.Metadata[:0]
	for i := range metadataRows {
		row := &metadataRows[i]
		if len(mms) < cap(mms) {
			mms = mms[:len(mms)+1]
		} else {
			mms = append(mms, prompb.MetricMetadata{})
		}
		md := &mms[len(mms)-1]
		md.MetricFamilyName = row.Metric
		md.Type = row.Type
		md.Help = row.Help
	}
	wc.writeRequest.Metadata = mms
}

// addRow adds r with the given timestamp to wc.
//
// The cfg is used as a read-only configuration source.
func (wc *writeRequestCtx) addRow(cfg *ScrapeWork, r *parser.Row, timestamp int64, needRelabel bool) error {
	metric := r.Metric

	// Add `exported_` prefix to metrics, which clash with the automatically generated
	// metric names only if the following conditions are met:
	//
	// - The `honor_labels` option isn't set to true in the scrape_config.
	//   If `honor_labels: true`, then the scraped metric name must remain unchanged
	//   because the user explicitly asked about it in the config.
	// - The metric has no labels (tags). If it has labels, then the metric value
	//   will be written into a separate time series comparing to automatically generated time series.
	//
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3557
	// and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3406
	if needRelabel && !cfg.HonorLabels && len(r.Tags) == 0 && isAutoMetric(metric) {
		bb := bbPool.Get()
		bb.B = append(bb.B, "exported_"...)
		bb.B = append(bb.B, metric...)
		metric = bytesutil.InternBytes(bb.B)
		bbPool.Put(bb)
	}
	labelsLen := len(wc.labels)
	targetLabels := cfg.Labels.GetLabels()
	wc.labels = appendLabels(wc.labels, metric, r.Tags, targetLabels, cfg.HonorLabels)
	if needRelabel {
		wc.labels = cfg.MetricRelabelConfigs.Apply(wc.labels, labelsLen)
	}
	wc.labels = promrelabel.FinalizeLabels(wc.labels[:labelsLen], wc.labels[labelsLen:])
	if len(wc.labels) == labelsLen {
		// Skip row without labels.
		return nil
	}
	// Add labels from `global->external_labels` section after the relabeling like Prometheus does.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3137
	externalLabels := cfg.ExternalLabels.GetLabels()
	wc.labels = appendExtraLabels(wc.labels, externalLabels, labelsLen, cfg.HonorLabels)
	labelLimit := cfg.LabelLimit
	labelsAfterRelabeling := len(wc.labels) - labelsLen
	if labelLimit > 0 && labelsAfterRelabeling > labelLimit {
		wc.labels = wc.labels[:labelsLen]
		return errLabelsLimitExceeded
	}
	sampleTimestamp := r.Timestamp
	if !cfg.HonorTimestamps || sampleTimestamp == 0 {
		sampleTimestamp = timestamp
	}
	wc.samples = append(wc.samples, prompb.Sample{
		Value:     r.Value,
		Timestamp: sampleTimestamp,
	})
	wr := &wc.writeRequest
	wr.Timeseries = append(wr.Timeseries, prompb.TimeSeries{
		Labels:  wc.labels[labelsLen:],
		Samples: wc.samples[len(wc.samples)-1:],
	})
	return nil
}

var bbPool bytesutil.ByteBufferPool

func appendLabels(dst []prompb.Label, metric string, src []parser.Tag, extraLabels []prompb.Label, honorLabels bool) []prompb.Label {
	dstLen := len(dst)
	dst = append(dst, prompb.Label{
		Name:  "__name__",
		Value: metric,
	})
	for i := range src {
		tag := &src[i]
		dst = append(dst, prompb.Label{
			Name:  tag.Key,
			Value: tag.Value,
		})
	}
	return appendExtraLabels(dst, extraLabels, dstLen, honorLabels)
}

func appendExtraLabels(dst, extraLabels []prompb.Label, offset int, honorLabels bool) []prompb.Label {
	// Add extraLabels to labels.
	// Handle duplicates in the same way as Prometheus does.
	if len(dst) == offset {
		// Fast path - add extraLabels to dst without the need to de-duplicate.
		dst = append(dst, extraLabels...)
		return dst
	}
	offsetEnd := len(dst)
	for _, label := range extraLabels {
		labels := dst[offset:offsetEnd]
		prevLabel := promrelabel.GetLabelByName(labels, label.Name)
		if prevLabel == nil {
			// Fast path - the label doesn't exist in labels, so just add it to dst.
			dst = append(dst, label)
			continue
		}
		if honorLabels {
			// Skip the extra label with the same name.
			continue
		}
		// Rename the prevLabel to "exported_" + label.Name
		// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
		exportedName := "exported_" + label.Name
		exportedLabel := promrelabel.GetLabelByName(labels, exportedName)
		if exportedLabel != nil {
			// The label with the name exported_<label.Name> already exists.
			// Add yet another 'exported_' prefix to it.
			exportedLabel.Name = "exported_" + exportedName
		}
		prevLabel.Name = exportedName
		dst = append(dst, label)
	}
	return dst
}
