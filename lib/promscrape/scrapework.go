package promscrape

import (
	"flag"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bloomfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/leveledbytebufferpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
	xxhash "github.com/cespare/xxhash/v2"
)

var (
	suppressScrapeErrors = flag.Bool("promscrape.suppressScrapeErrors", false, "Whether to suppress scrape errors logging. "+
		"The last error for each target is always available at '/targets' page even if scrape errors logging is suppressed")
	noStaleMarkers                = flag.Bool("promscrape.noStaleMarkers", false, "Whether to disable sending Prometheus stale markers for metrics when scrape target disappears. This option may reduce memory usage if stale markers aren't needed for your setup. This option also disables populating the scrape_series_added metric. See https://prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series")
	seriesLimitPerTarget          = flag.Int("promscrape.seriesLimitPerTarget", 0, "Optional limit on the number of unique time series a single scrape target can expose. See https://docs.victoriametrics.com/vmagent.html#cardinality-limiter for more info")
	minResponseSizeForStreamParse = flagutil.NewBytes("promscrape.minResponseSizeForStreamParse", 1e6, "The minimum target response size for automatic switching to stream parsing mode, which can reduce memory usage. See https://docs.victoriametrics.com/vmagent.html#stream-parsing-mode")
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

	// How to deal with conflicting labels.
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	HonorLabels bool

	// How to deal with scraped timestamps.
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	HonorTimestamps bool

	// Whether to deny redirects during requests to scrape config.
	DenyRedirects bool

	// OriginalLabels contains original labels before relabeling.
	//
	// These labels are needed for relabeling troubleshooting at /targets page.
	OriginalLabels []prompbmarshal.Label

	// Labels to add to the scraped metrics.
	//
	// The list contains at least the following labels according to https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
	//
	//     * job
	//     * __address__
	//     * __scheme__
	//     * __metrics_path__
	//     * __scrape_interval__
	//     * __scrape_timeout__
	//     * __param_<name>
	//     * __meta_*
	//     * user-defined labels set via `relabel_configs` section in `scrape_config`
	//
	// See also https://prometheus.io/docs/concepts/jobs_instances/
	Labels []prompbmarshal.Label

	// ProxyURL HTTP proxy url
	ProxyURL *proxy.URL

	// Auth config for ProxyUR:
	ProxyAuthConfig *promauth.Config

	// Auth config
	AuthConfig *promauth.Config

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
	SeriesLimit int

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
	// Take into account JobNameOriginal in order to capture the case when the original job_name is changed via relabeling.
	key := fmt.Sprintf("JobNameOriginal=%s, ScrapeURL=%s, ScrapeInterval=%s, ScrapeTimeout=%s, HonorLabels=%v, HonorTimestamps=%v, DenyRedirects=%v, Labels=%s, "+
		"ProxyURL=%s, ProxyAuthConfig=%s, AuthConfig=%s, MetricRelabelConfigs=%s, SampleLimit=%d, DisableCompression=%v, DisableKeepAlive=%v, StreamParse=%v, "+
		"ScrapeAlignInterval=%s, ScrapeOffset=%s, SeriesLimit=%d",
		sw.jobNameOriginal, sw.ScrapeURL, sw.ScrapeInterval, sw.ScrapeTimeout, sw.HonorLabels, sw.HonorTimestamps, sw.DenyRedirects, sw.LabelsString(),
		sw.ProxyURL.String(), sw.ProxyAuthConfig.String(),
		sw.AuthConfig.String(), sw.MetricRelabelConfigs.String(), sw.SampleLimit, sw.DisableCompression, sw.DisableKeepAlive, sw.StreamParse,
		sw.ScrapeAlignInterval, sw.ScrapeOffset, sw.SeriesLimit)
	return key
}

// Job returns job for the ScrapeWork
func (sw *ScrapeWork) Job() string {
	return promrelabel.GetLabelValueByName(sw.Labels, "job")
}

// LabelsString returns labels in Prometheus format for the given sw.
func (sw *ScrapeWork) LabelsString() string {
	labelsFinalized := promrelabel.FinalizeLabels(nil, sw.Labels)
	return promLabelsString(labelsFinalized)
}

func promLabelsString(labels []prompbmarshal.Label) string {
	// Calculate the required memory for storing serialized labels.
	n := 2 // for `{...}`
	for _, label := range labels {
		n += len(label.Name) + len(label.Value)
		n += 4 // for `="...",`
	}
	b := make([]byte, 0, n)
	b = append(b, '{')
	for i, label := range labels {
		b = append(b, label.Name...)
		b = append(b, '=')
		b = strconv.AppendQuote(b, label.Value)
		if i+1 < len(labels) {
			b = append(b, ',')
		}
	}
	b = append(b, '}')
	return bytesutil.ToUnsafeString(b)
}

type scrapeWork struct {
	// Config for the scrape.
	Config *ScrapeWork

	// ReadData is called for reading the data.
	ReadData func(dst []byte) ([]byte, error)

	// GetStreamReader is called if Config.StreamParse is set.
	GetStreamReader func() (*streamReader, error)

	// PushData is called for pushing collected data.
	PushData func(wr *prompbmarshal.WriteRequest)

	// ScrapeGroup is name of ScrapeGroup that
	// scrapeWork belongs to
	ScrapeGroup string

	tmpRow parser.Row

	// This flag is set to true if series_limit is exceeded.
	seriesLimitExceeded bool

	// labelsHashBuf is used for calculating the hash on series labels
	labelsHashBuf []byte

	// Optional limiter on the number of unique series per scrape target.
	seriesLimiter *bloomfilter.Limiter

	// Optional counter on the number of dropped samples if the limit on the number of unique series is set.
	seriesLimiterRowsDroppedTotal *metrics.Counter

	// prevBodyLen contains the previous response body length for the given scrape work.
	// It is used as a hint in order to reduce memory usage for body buffers.
	prevBodyLen int

	// prevLabelsLen contains the number labels scraped during the previous scrape.
	// It is used as a hint in order to reduce memory usage when parsing scrape responses.
	prevLabelsLen int

	// lastScrape holds the last response from scrape target.
	// It is used for staleness tracking and for populating scrape_series_added metric.
	// The lastScrape isn't populated if -promscrape.noStaleMarkers is set. This reduces memory usage.
	lastScrape []byte

	// lastScrapeCompressed is used for storing the compressed lastScrape between scrapes
	// in stream parsing mode in order to reduce memory usage when the lastScrape size
	// equals to or exceeds -promscrape.minResponseSizeForStreamParse
	lastScrapeCompressed []byte
}

func (sw *scrapeWork) loadLastScrape() string {
	if len(sw.lastScrapeCompressed) > 0 {
		b, err := encoding.DecompressZSTD(sw.lastScrape[:0], sw.lastScrapeCompressed)
		if err != nil {
			logger.Panicf("BUG: cannot unpack compressed previous response: %s", err)
		}
		sw.lastScrape = b
	}
	return bytesutil.ToUnsafeString(sw.lastScrape)
}

func (sw *scrapeWork) storeLastScrape(lastScrape []byte) {
	mustCompress := minResponseSizeForStreamParse.N > 0 && len(lastScrape) >= minResponseSizeForStreamParse.N
	if mustCompress {
		sw.lastScrapeCompressed = encoding.CompressZSTDLevel(sw.lastScrapeCompressed[:0], lastScrape, 1)
		sw.lastScrape = nil
	} else {
		sw.lastScrape = append(sw.lastScrape[:0], lastScrape...)
		sw.lastScrapeCompressed = nil
	}
}

func (sw *scrapeWork) finalizeLastScrape() {
	if len(sw.lastScrapeCompressed) > 0 {
		// The compressed lastScrape is available in sw.lastScrapeCompressed.
		// Release the memory occupied by sw.lastScrape, so it won't be occupied between scrapes.
		sw.lastScrape = nil
	}
	if len(sw.lastScrape) > 0 {
		// Release the memory occupied by sw.lastScrapeCompressed, so it won't be occupied between scrapes.
		sw.lastScrapeCompressed = nil
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
		// Include clusterMemberNum to the key in order to guarantee that each member in vmagent cluster
		// scrapes replicated targets at different time offsets. This guarantees that the deduplication consistently leaves samples
		// received from the same vmagent replica.
		// See https://docs.victoriametrics.com/vmagent.html#scraping-big-number-of-targets
		key := fmt.Sprintf("ClusterMemberNum=%d, ScrapeURL=%s, Labels=%s", *clusterMemberNum, sw.Config.ScrapeURL, sw.Config.LabelsString())
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
		timestamp = time.Now().UnixNano() / 1e6
		sw.scrapeAndLogError(timestamp, timestamp)
	}
	defer ticker.Stop()
	for {
		timestamp += scrapeInterval.Milliseconds()
		select {
		case <-stopCh:
			t := time.Now().UnixNano() / 1e6
			lastScrape := sw.loadLastScrape()
			select {
			case <-globalStopCh:
				// Do not send staleness markers on graceful shutdown as Prometheus does.
				// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2013#issuecomment-1006994079
			default:
				// Send staleness markers when the given target disappears.
				sw.sendStaleSeries(lastScrape, "", t, true)
			}
			if sw.seriesLimiter != nil {
				job := sw.Config.Job()
				metrics.UnregisterMetric(fmt.Sprintf(`promscrape_series_limit_rows_dropped_total{scrape_job_original=%q,scrape_job=%q,scrape_target=%q}`,
					sw.Config.jobNameOriginal, job, sw.Config.ScrapeURL))
				metrics.UnregisterMetric(fmt.Sprintf(`promscrape_series_limit_max_series{scrape_job_original=%q,scrape_job=%q,scrape_target=%q}`,
					sw.Config.jobNameOriginal, job, sw.Config.ScrapeURL))
				metrics.UnregisterMetric(fmt.Sprintf(`promscrape_series_limit_current_series{scrape_job_original=%q,scrape_job=%q,scrape_target=%q}`,
					sw.Config.jobNameOriginal, job, sw.Config.ScrapeURL))
				sw.seriesLimiter.MustStop()
			}
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
		logger.ErrorfSkipframes(1, "error when scraping %q from job %q with labels %s: %s; "+
			"scrape errors can be disabled by -promscrape.suppressScrapeErrors command-line flag",
			sw.Config.ScrapeURL, sw.Config.Job(), sw.Config.LabelsString(), s)
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

func (sw *scrapeWork) mustSwitchToStreamParseMode(responseSize int) bool {
	if minResponseSizeForStreamParse.N <= 0 {
		return false
	}
	return sw.Config.canSwitchToStreamParseMode() && responseSize >= minResponseSizeForStreamParse.N
}

func (sw *scrapeWork) scrapeInternal(scrapeTimestamp, realTimestamp int64) error {
	if *streamParse || sw.Config.StreamParse || sw.mustSwitchToStreamParseMode(sw.prevBodyLen) {
		// Read data from scrape targets in streaming manner.
		// This case is optimized for targets exposing more than ten thousand of metrics per target.
		return sw.scrapeStream(scrapeTimestamp, realTimestamp)
	}

	// Common case: read all the data from scrape target to memory (body) and then process it.
	// This case should work more optimally than stream parse code for common case when scrape target exposes
	// up to a few thousand metrics.
	body := leveledbytebufferpool.Get(sw.prevBodyLen)
	var err error
	body.B, err = sw.ReadData(body.B[:0])
	endTimestamp := time.Now().UnixNano() / 1e6
	duration := float64(endTimestamp-realTimestamp) / 1e3
	scrapeDuration.Update(duration)
	scrapeResponseSize.Update(float64(len(body.B)))
	up := 1
	wc := writeRequestCtxPool.Get(sw.prevLabelsLen)
	lastScrape := sw.loadLastScrape()
	bodyString := bytesutil.ToUnsafeString(body.B)
	areIdenticalSeries := *noStaleMarkers || parser.AreIdenticalSeriesFast(lastScrape, bodyString)
	if err != nil {
		up = 0
		scrapesFailed.Inc()
	} else {
		wc.rows.UnmarshalWithErrLogger(bodyString, sw.logError)
	}
	srcRows := wc.rows.Rows
	samplesScraped := len(srcRows)
	scrapedSamples.Update(float64(samplesScraped))
	for i := range srcRows {
		sw.addRowToTimeseries(wc, &srcRows[i], scrapeTimestamp, true)
	}
	samplesPostRelabeling := len(wc.writeRequest.Timeseries)
	if sw.Config.SampleLimit > 0 && samplesPostRelabeling > sw.Config.SampleLimit {
		wc.resetNoRows()
		up = 0
		scrapesSkippedBySampleLimit.Inc()
		err = fmt.Errorf("the response from %q exceeds sample_limit=%d; "+
			"either reduce the sample count for the target or increase sample_limit", sw.Config.ScrapeURL, sw.Config.SampleLimit)
	}
	if up == 0 {
		bodyString = ""
	}
	seriesAdded := 0
	if !areIdenticalSeries {
		// The returned value for seriesAdded may be bigger than the real number of added series
		// if some series were removed during relabeling.
		// This is a trade-off between performance and accuracy.
		seriesAdded = sw.getSeriesAdded(lastScrape, bodyString)
	}
	if sw.seriesLimitExceeded || !areIdenticalSeries {
		if sw.applySeriesLimit(wc) {
			sw.seriesLimitExceeded = true
		}
	}
	sw.addAutoTimeseries(wc, "up", float64(up), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_duration_seconds", duration, scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_samples_scraped", float64(samplesScraped), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_samples_post_metric_relabeling", float64(samplesPostRelabeling), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_series_added", float64(seriesAdded), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_timeout_seconds", sw.Config.ScrapeTimeout.Seconds(), scrapeTimestamp)
	sw.pushData(&wc.writeRequest)
	sw.prevLabelsLen = len(wc.labels)
	sw.prevBodyLen = len(bodyString)
	wc.reset()
	mustSwitchToStreamParse := sw.mustSwitchToStreamParseMode(len(bodyString))
	if !mustSwitchToStreamParse {
		// Return wc to the pool if the parsed response size was smaller than -promscrape.minResponseSizeForStreamParse
		// This should reduce memory usage when scraping targets with big responses.
		writeRequestCtxPool.Put(wc)
	}
	// body must be released only after wc is released, since wc refers to body.
	if !areIdenticalSeries {
		sw.sendStaleSeries(lastScrape, bodyString, scrapeTimestamp, false)
		sw.storeLastScrape(body.B)
	}
	sw.finalizeLastScrape()
	if !mustSwitchToStreamParse {
		// Return body to the pool only if its size is smaller than -promscrape.minResponseSizeForStreamParse
		// This should reduce memory usage when scraping targets which return big responses.
		leveledbytebufferpool.Put(body)
	}
	tsmGlobal.Update(sw.Config, sw.ScrapeGroup, up == 1, realTimestamp, int64(duration*1000), samplesScraped, err)
	return err
}

func (sw *scrapeWork) pushData(wr *prompbmarshal.WriteRequest) {
	startTime := time.Now()
	sw.PushData(wr)
	pushDataDuration.UpdateDuration(startTime)
}

type streamBodyReader struct {
	sr          *streamReader
	body        []byte
	bodyLen     int
	captureBody bool
}

func (sbr *streamBodyReader) Read(b []byte) (int, error) {
	n, err := sbr.sr.Read(b)
	sbr.bodyLen += n
	if sbr.captureBody {
		sbr.body = append(sbr.body, b[:n]...)
	}
	return n, err
}

func (sw *scrapeWork) scrapeStream(scrapeTimestamp, realTimestamp int64) error {
	samplesScraped := 0
	samplesPostRelabeling := 0
	wc := writeRequestCtxPool.Get(sw.prevLabelsLen)
	// Do not pool sbr and do not pre-allocate sbr.body in order to reduce memory usage when scraping big responses.
	sbr := &streamBodyReader{
		captureBody: !*noStaleMarkers,
	}

	sr, err := sw.GetStreamReader()
	if err != nil {
		err = fmt.Errorf("cannot read data: %s", err)
	} else {
		var mu sync.Mutex
		sbr.sr = sr
		err = parser.ParseStream(sbr, scrapeTimestamp, false, func(rows []parser.Row) error {
			mu.Lock()
			defer mu.Unlock()
			samplesScraped += len(rows)
			for i := range rows {
				sw.addRowToTimeseries(wc, &rows[i], scrapeTimestamp, true)
			}
			// Push the collected rows to sw before returning from the callback, since they cannot be held
			// after returning from the callback - this will result in data race.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825#issuecomment-723198247
			samplesPostRelabeling += len(wc.writeRequest.Timeseries)
			if sw.Config.SampleLimit > 0 && samplesPostRelabeling > sw.Config.SampleLimit {
				wc.resetNoRows()
				scrapesSkippedBySampleLimit.Inc()
				return fmt.Errorf("the response from %q exceeds sample_limit=%d; "+
					"either reduce the sample count for the target or increase sample_limit", sw.Config.ScrapeURL, sw.Config.SampleLimit)
			}
			sw.pushData(&wc.writeRequest)
			wc.resetNoRows()
			return nil
		}, sw.logError)
		sr.MustClose()
	}
	lastScrape := sw.loadLastScrape()
	bodyString := bytesutil.ToUnsafeString(sbr.body)
	areIdenticalSeries := *noStaleMarkers || parser.AreIdenticalSeriesFast(lastScrape, bodyString)

	scrapedSamples.Update(float64(samplesScraped))
	endTimestamp := time.Now().UnixNano() / 1e6
	duration := float64(endTimestamp-realTimestamp) / 1e3
	scrapeDuration.Update(duration)
	scrapeResponseSize.Update(float64(sbr.bodyLen))
	up := 1
	if err != nil {
		if samplesScraped == 0 {
			up = 0
		}
		scrapesFailed.Inc()
	}
	seriesAdded := 0
	if !areIdenticalSeries {
		// The returned value for seriesAdded may be bigger than the real number of added series
		// if some series were removed during relabeling.
		// This is a trade-off between performance and accuracy.
		seriesAdded = sw.getSeriesAdded(lastScrape, bodyString)
	}
	sw.addAutoTimeseries(wc, "up", float64(up), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_duration_seconds", duration, scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_samples_scraped", float64(samplesScraped), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_samples_post_metric_relabeling", float64(samplesPostRelabeling), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_series_added", float64(seriesAdded), scrapeTimestamp)
	sw.addAutoTimeseries(wc, "scrape_timeout_seconds", sw.Config.ScrapeTimeout.Seconds(), scrapeTimestamp)
	sw.pushData(&wc.writeRequest)
	sw.prevLabelsLen = len(wc.labels)
	sw.prevBodyLen = sbr.bodyLen
	wc.reset()
	writeRequestCtxPool.Put(wc)
	if !areIdenticalSeries {
		sw.sendStaleSeries(lastScrape, bodyString, scrapeTimestamp, false)
		sw.storeLastScrape(sbr.body)
	}
	sw.finalizeLastScrape()
	tsmGlobal.Update(sw.Config, sw.ScrapeGroup, up == 1, realTimestamp, int64(duration*1000), samplesScraped, err)
	// Do not track active series in streaming mode, since this may need too big amounts of memory
	// when the target exports too big number of metrics.
	return err
}

// leveledWriteRequestCtxPool allows reducing memory usage when writeRequesCtx
// structs contain mixed number of labels.
//
// Its logic has been copied from leveledbytebufferpool.
type leveledWriteRequestCtxPool struct {
	pools [13]sync.Pool
}

func (lwp *leveledWriteRequestCtxPool) Get(labelsCapacity int) *writeRequestCtx {
	id, capacityNeeded := lwp.getPoolIDAndCapacity(labelsCapacity)
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
	size >>= 3
	id := bits.Len(uint(size))
	if id >= len(lwp.pools) {
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

func (sw *scrapeWork) getSeriesAdded(lastScrape, currScrape string) int {
	if currScrape == "" {
		return 0
	}
	bodyString := parser.GetRowsDiff(currScrape, lastScrape)
	return strings.Count(bodyString, "\n")
}

func (sw *scrapeWork) applySeriesLimit(wc *writeRequestCtx) bool {
	seriesLimit := *seriesLimitPerTarget
	if sw.Config.SeriesLimit > 0 {
		seriesLimit = sw.Config.SeriesLimit
	}
	if sw.seriesLimiter == nil && seriesLimit > 0 {
		job := sw.Config.Job()
		sw.seriesLimiter = bloomfilter.NewLimiter(seriesLimit, 24*time.Hour)
		sw.seriesLimiterRowsDroppedTotal = metrics.GetOrCreateCounter(fmt.Sprintf(`promscrape_series_limit_rows_dropped_total{scrape_job_original=%q,scrape_job=%q,scrape_target=%q}`,
			sw.Config.jobNameOriginal, job, sw.Config.ScrapeURL))
		_ = metrics.GetOrCreateGauge(fmt.Sprintf(`promscrape_series_limit_max_series{scrape_job_original=%q,scrape_job=%q,scrape_target=%q}`,
			sw.Config.jobNameOriginal, job, sw.Config.ScrapeURL), func() float64 {
			return float64(sw.seriesLimiter.MaxItems())
		})
		_ = metrics.GetOrCreateGauge(fmt.Sprintf(`promscrape_series_limit_current_series{scrape_job_original=%q,scrape_job=%q,scrape_target=%q}`,
			sw.Config.jobNameOriginal, job, sw.Config.ScrapeURL), func() float64 {
			return float64(sw.seriesLimiter.CurrentItems())
		})
	}
	hsl := sw.seriesLimiter
	if hsl == nil {
		return false
	}
	dstSeries := wc.writeRequest.Timeseries[:0]
	limitExceeded := false
	for _, ts := range wc.writeRequest.Timeseries {
		h := sw.getLabelsHash(ts.Labels)
		if !hsl.Add(h) {
			// The limit on the number of hourly unique series per scrape target has been exceeded.
			// Drop the metric.
			sw.seriesLimiterRowsDroppedTotal.Inc()
			limitExceeded = true
			continue
		}
		dstSeries = append(dstSeries, ts)
	}
	wc.writeRequest.Timeseries = dstSeries
	return limitExceeded
}

func (sw *scrapeWork) sendStaleSeries(lastScrape, currScrape string, timestamp int64, addAutoSeries bool) {
	if *noStaleMarkers {
		return
	}
	bodyString := lastScrape
	if currScrape != "" {
		bodyString = parser.GetRowsDiff(lastScrape, currScrape)
	}
	wc := &writeRequestCtx{}
	if bodyString != "" {
		wc.rows.Unmarshal(bodyString)
		srcRows := wc.rows.Rows
		for i := range srcRows {
			sw.addRowToTimeseries(wc, &srcRows[i], timestamp, true)
		}
	}
	if addAutoSeries {
		sw.addAutoTimeseries(wc, "up", 0, timestamp)
		sw.addAutoTimeseries(wc, "scrape_duration_seconds", 0, timestamp)
		sw.addAutoTimeseries(wc, "scrape_samples_scraped", 0, timestamp)
		sw.addAutoTimeseries(wc, "scrape_samples_post_metric_relabeling", 0, timestamp)
		sw.addAutoTimeseries(wc, "scrape_series_added", 0, timestamp)
	}
	series := wc.writeRequest.Timeseries
	if len(series) == 0 {
		return
	}
	// Substitute all the values with Prometheus stale markers.
	for _, tss := range series {
		samples := tss.Samples
		for i := range samples {
			samples[i].Value = decimal.StaleNaN
		}
		staleSamplesCreated.Add(len(samples))
	}
	sw.pushData(&wc.writeRequest)
}

var staleSamplesCreated = metrics.NewCounter(`promscrape_stale_samples_created_total`)

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
		wc.labels = sw.Config.MetricRelabelConfigs.Apply(wc.labels, labelsLen, true)
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
