package remotewrite

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bloomfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ratelimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/streamaggr"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
	"github.com/cespare/xxhash/v2"
)

var (
	remoteWriteURLs = flagutil.NewArrayString("remoteWrite.url", "Remote storage URL to write data to. It must support either VictoriaMetrics remote write protocol "+
		"or Prometheus remote_write protocol. Example url: http://<victoriametrics-host>:8428/api/v1/write . "+
		"Pass multiple -remoteWrite.url options in order to replicate the collected data to multiple remote storage systems. "+
		"The data can be sharded among the configured remote storage systems if -remoteWrite.shardByURL flag is set")
	remoteWriteMultitenantURLs = flagutil.NewArrayString("remoteWrite.multitenantURL", "Base path for multitenant remote storage URL to write data to. "+
		"See https://docs.victoriametrics.com/vmagent/#multitenancy for details. Example url: http://<vminsert>:8480 . "+
		"Pass multiple -remoteWrite.multitenantURL flags in order to replicate data to multiple remote storage systems. "+
		"This flag is deprecated in favor of -enableMultitenantHandlers . See https://docs.victoriametrics.com/vmagent/#multitenancy")
	enableMultitenantHandlers = flag.Bool("enableMultitenantHandlers", false, "Whether to process incoming data via multitenant insert handlers according to "+
		"https://docs.victoriametrics.com/cluster-victoriametrics/#url-format . By default incoming data is processed via single-node insert handlers "+
		"according to https://docs.victoriametrics.com/#how-to-import-time-series-data ."+
		"See https://docs.victoriametrics.com/vmagent/#multitenancy for details")

	shardByURL = flag.Bool("remoteWrite.shardByURL", false, "Whether to shard outgoing series across all the remote storage systems enumerated via -remoteWrite.url . "+
		"By default the data is replicated across all the -remoteWrite.url . See https://docs.victoriametrics.com/vmagent/#sharding-among-remote-storages . "+
		"See also -remoteWrite.shardByURLReplicas")
	shardByURLReplicas = flag.Int("remoteWrite.shardByURLReplicas", 1, "How many copies of data to make among remote storage systems enumerated via -remoteWrite.url "+
		"when -remoteWrite.shardByURL is set. See https://docs.victoriametrics.com/vmagent/#sharding-among-remote-storages")
	shardByURLLabels = flagutil.NewArrayString("remoteWrite.shardByURL.labels", "Optional list of labels, which must be used for sharding outgoing samples "+
		"among remote storage systems if -remoteWrite.shardByURL command-line flag is set. By default all the labels are used for sharding in order to gain "+
		"even distribution of series over the specified -remoteWrite.url systems. See also -remoteWrite.shardByURL.ignoreLabels")
	shardByURLIgnoreLabels = flagutil.NewArrayString("remoteWrite.shardByURL.ignoreLabels", "Optional list of labels, which must be ignored when sharding outgoing samples "+
		"among remote storage systems if -remoteWrite.shardByURL command-line flag is set. By default all the labels are used for sharding in order to gain "+
		"even distribution of series over the specified -remoteWrite.url systems. See also -remoteWrite.shardByURL.labels")

	tmpDataPath = flag.String("remoteWrite.tmpDataPath", "vmagent-remotewrite-data", "Path to directory for storing pending data, which isn't sent to the configured -remoteWrite.url . "+
		"See also -remoteWrite.maxDiskUsagePerURL and -remoteWrite.disableOnDiskQueue")
	keepDanglingQueues = flag.Bool("remoteWrite.keepDanglingQueues", false, "Keep persistent queues contents at -remoteWrite.tmpDataPath in case there are no matching -remoteWrite.url. "+
		"Useful when -remoteWrite.url is changed temporarily and persistent queue files will be needed later on.")
	queues = flag.Int("remoteWrite.queues", cgroup.AvailableCPUs()*2, "The number of concurrent queues to each -remoteWrite.url. Set more queues if default number of queues "+
		"isn't enough for sending high volume of collected data to remote storage. "+
		"Default value depends on the number of available CPU cores. It should work fine in most cases since it minimizes resource usage")
	showRemoteWriteURL = flag.Bool("remoteWrite.showURL", false, "Whether to show -remoteWrite.url in the exported metrics. "+
		"It is hidden by default, since it can contain sensitive info such as auth key")
	maxPendingBytesPerURL = flagutil.NewArrayBytes("remoteWrite.maxDiskUsagePerURL", 0, "The maximum file-based buffer size in bytes at -remoteWrite.tmpDataPath "+
		"for each -remoteWrite.url. When buffer size reaches the configured maximum, then old data is dropped when adding new data to the buffer. "+
		"Buffered data is stored in ~500MB chunks. It is recommended to set the value for this flag to a multiple of the block size 500MB. "+
		"Disk usage is unlimited if the value is set to 0")
	significantFigures = flagutil.NewArrayInt("remoteWrite.significantFigures", 0, "The number of significant figures to leave in metric values before writing them "+
		"to remote storage. See https://en.wikipedia.org/wiki/Significant_figures . Zero value saves all the significant figures. "+
		"This option may be used for improving data compression for the stored metrics. See also -remoteWrite.roundDigits")
	roundDigits = flagutil.NewArrayInt("remoteWrite.roundDigits", 100, "Round metric values to this number of decimal digits after the point before "+
		"writing them to remote storage. "+
		"Examples: -remoteWrite.roundDigits=2 would round 1.236 to 1.24, while -remoteWrite.roundDigits=-1 would round 126.78 to 130. "+
		"By default, digits rounding is disabled. Set it to 100 for disabling it for a particular remote storage. "+
		"This option may be used for improving data compression for the stored metrics")
	sortLabels = flag.Bool("sortLabels", false, `Whether to sort labels for incoming samples before writing them to all the configured remote storage systems. `+
		`This may be needed for reducing memory usage at remote storage when the order of labels in incoming samples is random. `+
		`For example, if m{k1="v1",k2="v2"} may be sent as m{k2="v2",k1="v1"}`+
		`Enabled sorting for labels can slow down ingestion performance a bit`)
	maxHourlySeries = flag.Int("remoteWrite.maxHourlySeries", 0, "The maximum number of unique series vmagent can send to remote storage systems during the last hour. "+
		"Excess series are logged and dropped. This can be useful for limiting series cardinality. See https://docs.victoriametrics.com/vmagent/#cardinality-limiter")
	maxDailySeries = flag.Int("remoteWrite.maxDailySeries", 0, "The maximum number of unique series vmagent can send to remote storage systems during the last 24 hours. "+
		"Excess series are logged and dropped. This can be useful for limiting series churn rate. See https://docs.victoriametrics.com/vmagent/#cardinality-limiter")
	maxIngestionRate = flag.Int("maxIngestionRate", 0, "The maximum number of samples vmagent can receive per second. Data ingestion is paused when the limit is exceeded. "+
		"By default there are no limits on samples ingestion rate. See also -remoteWrite.rateLimit")

	streamAggrConfig = flagutil.NewArrayString("remoteWrite.streamAggr.config", "Optional path to file with stream aggregation config. "+
		"See https://docs.victoriametrics.com/stream-aggregation/ . "+
		"See also -remoteWrite.streamAggr.keepInput, -remoteWrite.streamAggr.dropInput and -remoteWrite.streamAggr.dedupInterval")
	streamAggrKeepInput = flagutil.NewArrayBool("remoteWrite.streamAggr.keepInput", "Whether to keep all the input samples after the aggregation "+
		"with -remoteWrite.streamAggr.config. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to the corresponding -remoteWrite.url . See also -remoteWrite.streamAggr.dropInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrDropInput = flagutil.NewArrayBool("remoteWrite.streamAggr.dropInput", "Whether to drop all the input samples after the aggregation "+
		"with -remoteWrite.streamAggr.config. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to the corresponding -remoteWrite.url . See also -remoteWrite.streamAggr.keepInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrDedupInterval = flagutil.NewArrayDuration("remoteWrite.streamAggr.dedupInterval", 0, "Input samples are de-duplicated with this interval before optional aggregation "+
		"with -remoteWrite.streamAggr.config . See also -dedup.minScrapeInterval and https://docs.victoriametrics.com/stream-aggregation/#deduplication")
	streamAggrIgnoreOldSamples = flagutil.NewArrayBool("remoteWrite.streamAggr.ignoreOldSamples", "Whether to ignore input samples with old timestamps outside the current aggregation interval "+
		"for the corresponding -remoteWrite.streamAggr.config . See https://docs.victoriametrics.com/stream-aggregation/#ignoring-old-samples")
	streamAggrIgnoreFirstIntervals = flag.Int("remoteWrite.streamAggr.ignoreFirstIntervals", 0, "Number of aggregation intervals to skip after the start. Increase this value if you observe incorrect aggregation results after vmagent restarts. It could be caused by receiving unordered delayed data from clients pushing data into the vmagent. "+
		"See https://docs.victoriametrics.com/stream-aggregation/#ignore-aggregation-intervals-on-start")
	streamAggrDropInputLabels = flagutil.NewArrayString("streamAggr.dropInputLabels", "An optional list of labels to drop from samples "+
		"before stream de-duplication and aggregation . See https://docs.victoriametrics.com/stream-aggregation/#dropping-unneeded-labels")

	disableOnDiskQueue = flag.Bool("remoteWrite.disableOnDiskQueue", false, "Whether to disable storing pending data to -remoteWrite.tmpDataPath "+
		"when the configured remote storage systems cannot keep up with the data ingestion rate. See https://docs.victoriametrics.com/vmagent/#disabling-on-disk-persistence ."+
		"See also -remoteWrite.dropSamplesOnOverload")
	dropSamplesOnOverload = flag.Bool("remoteWrite.dropSamplesOnOverload", false, "Whether to drop samples when -remoteWrite.disableOnDiskQueue is set and if the samples "+
		"cannot be pushed into the configured remote storage systems in a timely manner. See https://docs.victoriametrics.com/vmagent/#disabling-on-disk-persistence")
)

var (
	// rwctxsDefault contains statically populated entries when -remoteWrite.url is specified.
	rwctxsDefault []*remoteWriteCtx

	// rwctxsMap contains dynamically populated entries when -remoteWrite.multitenantURL is specified.
	rwctxsMap     = make(map[tenantmetrics.TenantID][]*remoteWriteCtx)
	rwctxsMapLock sync.Mutex

	// Data without tenant id is written to defaultAuthToken if -remoteWrite.multitenantURL is specified.
	defaultAuthToken = &auth.Token{}

	// ErrQueueFullHTTPRetry must be returned when TryPush() returns false.
	ErrQueueFullHTTPRetry = &httpserver.ErrorWithStatusCode{
		Err: fmt.Errorf("remote storage systems cannot keep up with the data ingestion rate; retry the request later " +
			"or remove -remoteWrite.disableOnDiskQueue from vmagent command-line flags, so it could save pending data to -remoteWrite.tmpDataPath; " +
			"see https://docs.victoriametrics.com/vmagent/#disabling-on-disk-persistence"),
		StatusCode: http.StatusTooManyRequests,
	}
)

// MultitenancyEnabled returns true if -enableMultitenantHandlers or -remoteWrite.multitenantURL is specified.
func MultitenancyEnabled() bool {
	return *enableMultitenantHandlers || len(*remoteWriteMultitenantURLs) > 0
}

// Contains the current relabelConfigs.
var allRelabelConfigs atomic.Pointer[relabelConfigs]

// maxQueues limits the maximum value for `-remoteWrite.queues`. There is no sense in setting too high value,
// since it may lead to high memory usage due to big number of buffers.
var maxQueues = cgroup.AvailableCPUs() * 16

const persistentQueueDirname = "persistent-queue"

// InitSecretFlags must be called after flag.Parse and before any logging.
func InitSecretFlags() {
	if !*showRemoteWriteURL {
		// remoteWrite.url can contain authentication codes, so hide it at `/metrics` output.
		flagutil.RegisterSecretFlag("remoteWrite.url")
	}
}

var (
	shardByURLLabelsMap       map[string]struct{}
	shardByURLIgnoreLabelsMap map[string]struct{}
)

// Init initializes remotewrite.
//
// It must be called after flag.Parse().
//
// Stop must be called for graceful shutdown.
func Init() {
	if len(*remoteWriteURLs) == 0 && len(*remoteWriteMultitenantURLs) == 0 {
		logger.Fatalf("at least one `-remoteWrite.url` or `-remoteWrite.multitenantURL` command-line flag must be set")
	}
	if len(*remoteWriteURLs) > 0 && len(*remoteWriteMultitenantURLs) > 0 {
		logger.Fatalf("cannot set both `-remoteWrite.url` and `-remoteWrite.multitenantURL` command-line flags")
	}
	if *maxHourlySeries > 0 {
		hourlySeriesLimiter = bloomfilter.NewLimiter(*maxHourlySeries, time.Hour)
		_ = metrics.NewGauge(`vmagent_hourly_series_limit_max_series`, func() float64 {
			return float64(hourlySeriesLimiter.MaxItems())
		})
		_ = metrics.NewGauge(`vmagent_hourly_series_limit_current_series`, func() float64 {
			return float64(hourlySeriesLimiter.CurrentItems())
		})
	}
	if *maxDailySeries > 0 {
		dailySeriesLimiter = bloomfilter.NewLimiter(*maxDailySeries, 24*time.Hour)
		_ = metrics.NewGauge(`vmagent_daily_series_limit_max_series`, func() float64 {
			return float64(dailySeriesLimiter.MaxItems())
		})
		_ = metrics.NewGauge(`vmagent_daily_series_limit_current_series`, func() float64 {
			return float64(dailySeriesLimiter.CurrentItems())
		})
	}

	if *queues > maxQueues {
		*queues = maxQueues
	}
	if *queues <= 0 {
		*queues = 1
	}

	if len(*shardByURLLabels) > 0 && len(*shardByURLIgnoreLabels) > 0 {
		logger.Fatalf("-remoteWrite.shardByURL.labels and -remoteWrite.shardByURL.ignoreLabels cannot be set simultaneously; " +
			"see https://docs.victoriametrics.com/vmagent/#sharding-among-remote-storages")
	}
	shardByURLLabelsMap = newMapFromStrings(*shardByURLLabels)
	shardByURLIgnoreLabelsMap = newMapFromStrings(*shardByURLIgnoreLabels)

	initLabelsGlobal()

	// Register SIGHUP handler for config reload before loadRelabelConfigs.
	// This guarantees that the config will be re-read if the signal arrives just after loadRelabelConfig.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240
	sighupCh := procutil.NewSighupChan()

	rcs, err := loadRelabelConfigs()
	if err != nil {
		logger.Fatalf("cannot load relabel configs: %s", err)
	}
	allRelabelConfigs.Store(rcs)
	relabelConfigSuccess.Set(1)
	relabelConfigTimestamp.Set(fasttime.UnixTimestamp())

	if len(*remoteWriteURLs) > 0 {
		rwctxsDefault = newRemoteWriteCtxs(nil, *remoteWriteURLs)
	}
	dropDanglingQueues()

	// Start config reloader.
	configReloaderWG.Add(1)
	go func() {
		defer configReloaderWG.Done()
		for {
			select {
			case <-sighupCh:
			case <-configReloaderStopCh:
				return
			}
			reloadRelabelConfigs()
			reloadStreamAggrConfigs()
		}
	}()
}

func dropDanglingQueues() {
	if *keepDanglingQueues {
		return
	}
	if len(*remoteWriteMultitenantURLs) > 0 {
		// Do not drop dangling queues for *remoteWriteMultitenantURLs, since it is impossible to determine
		// unused queues for multitenant urls - they are created on demand when new sample for the given
		// tenant is pushed to remote storage.
		return
	}
	// Remove dangling persistent queues, if any.
	// This is required for the case when the number of queues has been changed or URL have been changed.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4014
	//
	// In case if there were many persistent queues with identical *remoteWriteURLs
	// the queue with the last index will be dropped.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6140
	existingQueues := make(map[string]struct{}, len(rwctxsDefault))
	for _, rwctx := range rwctxsDefault {
		existingQueues[rwctx.fq.Dirname()] = struct{}{}
	}

	queuesDir := filepath.Join(*tmpDataPath, persistentQueueDirname)
	files := fs.MustReadDir(queuesDir)
	removed := 0
	for _, f := range files {
		dirname := f.Name()
		if _, ok := existingQueues[dirname]; !ok {
			logger.Infof("removing dangling queue %q", dirname)
			fullPath := filepath.Join(queuesDir, dirname)
			fs.MustRemoveAll(fullPath)
			removed++
		}
	}
	if removed > 0 {
		logger.Infof("removed %d dangling queues from %q, active queues: %d", removed, *tmpDataPath, len(rwctxsDefault))
	}
}

func reloadRelabelConfigs() {
	relabelConfigReloads.Inc()
	logger.Infof("reloading relabel configs pointed by -remoteWrite.relabelConfig and -remoteWrite.urlRelabelConfig")
	rcs, err := loadRelabelConfigs()
	if err != nil {
		relabelConfigReloadErrors.Inc()
		relabelConfigSuccess.Set(0)
		logger.Errorf("cannot reload relabel configs; preserving the previous configs; error: %s", err)
		return
	}
	allRelabelConfigs.Store(rcs)
	relabelConfigSuccess.Set(1)
	relabelConfigTimestamp.Set(fasttime.UnixTimestamp())
	logger.Infof("successfully reloaded relabel configs")
}

var (
	relabelConfigReloads      = metrics.NewCounter(`vmagent_relabel_config_reloads_total`)
	relabelConfigReloadErrors = metrics.NewCounter(`vmagent_relabel_config_reloads_errors_total`)
	relabelConfigSuccess      = metrics.NewGauge(`vmagent_relabel_config_last_reload_successful`, nil)
	relabelConfigTimestamp    = metrics.NewCounter(`vmagent_relabel_config_last_reload_success_timestamp_seconds`)
)

func reloadStreamAggrConfigs() {
	if len(*remoteWriteMultitenantURLs) > 0 {
		rwctxsMapLock.Lock()
		for _, rwctxs := range rwctxsMap {
			reinitStreamAggr(rwctxs)
		}
		rwctxsMapLock.Unlock()
	} else {
		reinitStreamAggr(rwctxsDefault)
	}
}

func reinitStreamAggr(rwctxs []*remoteWriteCtx) {
	for _, rwctx := range rwctxs {
		rwctx.reinitStreamAggr()
	}
}

func newRemoteWriteCtxs(at *auth.Token, urls []string) []*remoteWriteCtx {
	if len(urls) == 0 {
		logger.Panicf("BUG: urls must be non-empty")
	}

	maxInmemoryBlocks := memory.Allowed() / len(urls) / *maxRowsPerBlock / 100
	if maxInmemoryBlocks / *queues > 100 {
		// There is no much sense in keeping higher number of blocks in memory,
		// since this means that the producer outperforms consumer and the queue
		// will continue growing. It is better storing the queue to file.
		maxInmemoryBlocks = 100 * *queues
	}
	if maxInmemoryBlocks < 2 {
		maxInmemoryBlocks = 2
	}
	rwctxs := make([]*remoteWriteCtx, len(urls))
	for i, remoteWriteURLRaw := range urls {
		remoteWriteURL, err := url.Parse(remoteWriteURLRaw)
		if err != nil {
			logger.Fatalf("invalid -remoteWrite.url=%q: %s", remoteWriteURL, err)
		}
		sanitizedURL := fmt.Sprintf("%d:secret-url", i+1)
		if at != nil {
			// Construct full remote_write url for the given tenant according to https://docs.victoriametrics.com/cluster-victoriametrics/#url-format
			remoteWriteURL.Path = fmt.Sprintf("%s/insert/%d:%d/prometheus/api/v1/write", remoteWriteURL.Path, at.AccountID, at.ProjectID)
			sanitizedURL = fmt.Sprintf("%s:%d:%d", sanitizedURL, at.AccountID, at.ProjectID)
		}
		if *showRemoteWriteURL {
			sanitizedURL = fmt.Sprintf("%d:%s", i+1, remoteWriteURL)
		}
		rwctxs[i] = newRemoteWriteCtx(i, remoteWriteURL, maxInmemoryBlocks, sanitizedURL)
	}
	return rwctxs
}

var configReloaderStopCh = make(chan struct{})
var configReloaderWG sync.WaitGroup

// StartIngestionRateLimiter starts ingestion rate limiter.
//
// Ingestion rate limiter must be started before Init() call.
//
// StopIngestionRateLimiter must be called before Stop() call in order to unblock all the callers
// to ingestion rate limiter. Otherwise deadlock may occur at Stop() call.
func StartIngestionRateLimiter() {
	if *maxIngestionRate <= 0 {
		return
	}
	ingestionRateLimitReached := metrics.NewCounter(`vmagent_max_ingestion_rate_limit_reached_total`)
	ingestionRateLimiterStopCh = make(chan struct{})
	ingestionRateLimiter = ratelimiter.New(int64(*maxIngestionRate), ingestionRateLimitReached, ingestionRateLimiterStopCh)
}

// StopIngestionRateLimiter stops ingestion rate limiter.
func StopIngestionRateLimiter() {
	if ingestionRateLimiterStopCh == nil {
		return
	}
	close(ingestionRateLimiterStopCh)
	ingestionRateLimiterStopCh = nil
}

var (
	ingestionRateLimiter       *ratelimiter.RateLimiter
	ingestionRateLimiterStopCh chan struct{}
)

// Stop stops remotewrite.
//
// It is expected that nobody calls TryPush during and after the call to this func.
func Stop() {
	close(configReloaderStopCh)
	configReloaderWG.Wait()

	for _, rwctx := range rwctxsDefault {
		rwctx.MustStop()
	}
	rwctxsDefault = nil

	// There is no need in locking rwctxsMapLock here, since nobody should call TryPush during the Stop call.
	for _, rwctxs := range rwctxsMap {
		for _, rwctx := range rwctxs {
			rwctx.MustStop()
		}
	}
	rwctxsMap = nil

	if sl := hourlySeriesLimiter; sl != nil {
		sl.MustStop()
	}
	if sl := dailySeriesLimiter; sl != nil {
		sl.MustStop()
	}
}

// PushDropSamplesOnFailure pushes wr to the configured remote storage systems set via -remoteWrite.url and -remoteWrite.multitenantURL
//
// If at is nil, then the data is pushed to the configured -remoteWrite.url.
// If at isn't nil, the data is pushed to the configured -remoteWrite.multitenantURL.
//
// PushDropSamplesOnFailure can modify wr contents.
func PushDropSamplesOnFailure(at *auth.Token, wr *prompbmarshal.WriteRequest) {
	_ = tryPush(at, wr, true)
}

// TryPush tries sending wr to the configured remote storage systems set via -remoteWrite.url and -remoteWrite.multitenantURL
//
// If at is nil, then the data is pushed to the configured -remoteWrite.url.
// If at isn't nil, the data is pushed to the configured -remoteWrite.multitenantURL.
//
// TryPush can modify wr contents, so the caller must re-initialize wr before calling TryPush() after unsuccessful attempt.
// TryPush may send partial data from wr on unsuccessful attempt, so repeated call for the same wr may send the data multiple times.
//
// The caller must return ErrQueueFullHTTPRetry to the client, which sends wr, if TryPush returns false.
func TryPush(at *auth.Token, wr *prompbmarshal.WriteRequest) bool {
	return tryPush(at, wr, *dropSamplesOnOverload)
}

func tryPush(at *auth.Token, wr *prompbmarshal.WriteRequest, dropSamplesOnFailure bool) bool {
	tss := wr.Timeseries

	if at == nil && MultitenancyEnabled() {
		// Write data to default tenant if at isn't set when multitenancy is enabled.
		at = defaultAuthToken
	}

	var tenantRctx *relabelCtx
	var rwctxs []*remoteWriteCtx
	if at == nil {
		rwctxs = rwctxsDefault
	} else if len(*remoteWriteMultitenantURLs) == 0 {
		// Convert at to (vm_account_id, vm_project_id) labels.
		tenantRctx = getRelabelCtx()
		defer putRelabelCtx(tenantRctx)
		rwctxs = rwctxsDefault
	} else {
		rwctxsMapLock.Lock()
		tenantID := tenantmetrics.TenantID{
			AccountID: at.AccountID,
			ProjectID: at.ProjectID,
		}
		rwctxs = rwctxsMap[tenantID]
		if rwctxs == nil {
			rwctxs = newRemoteWriteCtxs(at, *remoteWriteMultitenantURLs)
			rwctxsMap[tenantID] = rwctxs
		}
		rwctxsMapLock.Unlock()
	}

	rowsCount := getRowsCount(tss)

	if *disableOnDiskQueue {
		// Quick check whether writes to configured remote storage systems are blocked.
		// This allows saving CPU time spent on relabeling and block compression
		// if some of remote storage systems cannot keep up with the data ingestion rate.
		for _, rwctx := range rwctxs {
			if rwctx.fq.IsWriteBlocked() {
				pushFailures.Inc()
				if dropSamplesOnFailure {
					// Just drop samples
					samplesDropped.Add(rowsCount)
					return true
				}
				return false
			}
		}
	}

	var rctx *relabelCtx
	rcs := allRelabelConfigs.Load()
	pcsGlobal := rcs.global
	if pcsGlobal.Len() > 0 {
		rctx = getRelabelCtx()
		defer putRelabelCtx(rctx)
	}
	globalRowsPushedBeforeRelabel.Add(rowsCount)
	maxSamplesPerBlock := *maxRowsPerBlock
	// Allow up to 10x of labels per each block on average.
	maxLabelsPerBlock := 10 * maxSamplesPerBlock

	for len(tss) > 0 {
		// Process big tss in smaller blocks in order to reduce the maximum memory usage
		samplesCount := 0
		labelsCount := 0
		i := 0
		for i < len(tss) {
			samplesCount += len(tss[i].Samples)
			labelsCount += len(tss[i].Samples) * len(tss[i].Labels)
			i++
			if samplesCount >= maxSamplesPerBlock || labelsCount >= maxLabelsPerBlock {
				break
			}
		}

		ingestionRateLimiter.Register(samplesCount)

		tssBlock := tss
		if i < len(tss) {
			tssBlock = tss[:i]
			tss = tss[i:]
		} else {
			tss = nil
		}
		if tenantRctx != nil {
			tenantRctx.tenantToLabels(tssBlock, at.AccountID, at.ProjectID)
		}
		if rctx != nil {
			rowsCountBeforeRelabel := getRowsCount(tssBlock)
			tssBlock = rctx.applyRelabeling(tssBlock, pcsGlobal)
			rowsCountAfterRelabel := getRowsCount(tssBlock)
			rowsDroppedByGlobalRelabel.Add(rowsCountBeforeRelabel - rowsCountAfterRelabel)
		}
		sortLabelsIfNeeded(tssBlock)
		tssBlock = limitSeriesCardinality(tssBlock)
		if !tryPushBlockToRemoteStorages(rwctxs, tssBlock) {
			if !*disableOnDiskQueue {
				logger.Panicf("BUG: tryPushBlockToRemoteStorages must return true if -remoteWrite.disableOnDiskQueue isn't set")
			}
			pushFailures.Inc()
			if dropSamplesOnFailure {
				samplesDropped.Add(rowsCount)
				return true
			}
			return false
		}
	}
	return true
}

var (
	samplesDropped = metrics.NewCounter(`vmagent_remotewrite_samples_dropped_total`)
	pushFailures   = metrics.NewCounter(`vmagent_remotewrite_push_failures_total`)
)

func tryPushBlockToRemoteStorages(rwctxs []*remoteWriteCtx, tssBlock []prompbmarshal.TimeSeries) bool {
	if len(tssBlock) == 0 {
		// Nothing to push
		return true
	}

	if len(rwctxs) == 1 {
		// Fast path - just push data to the configured single remote storage
		return rwctxs[0].TryPush(tssBlock)
	}

	// We need to push tssBlock to multiple remote storages.
	// This is either sharding or replication depending on -remoteWrite.shardByURL command-line flag value.
	if *shardByURL && *shardByURLReplicas < len(rwctxs) {
		// Shard tssBlock samples among rwctxs.
		replicas := *shardByURLReplicas
		if replicas <= 0 {
			replicas = 1
		}
		return tryShardingBlockAmongRemoteStorages(rwctxs, tssBlock, replicas)
	}

	// Replicate tssBlock samples among rwctxs.
	// Push tssBlock to remote storage systems in parallel in order to reduce
	// the time needed for sending the data to multiple remote storage systems.
	var wg sync.WaitGroup
	wg.Add(len(rwctxs))
	var anyPushFailed atomic.Bool
	for _, rwctx := range rwctxs {
		go func(rwctx *remoteWriteCtx) {
			defer wg.Done()
			if !rwctx.TryPush(tssBlock) {
				anyPushFailed.Store(true)
			}
		}(rwctx)
	}
	wg.Wait()
	return !anyPushFailed.Load()
}

func tryShardingBlockAmongRemoteStorages(rwctxs []*remoteWriteCtx, tssBlock []prompbmarshal.TimeSeries, replicas int) bool {
	x := getTSSShards(len(rwctxs))
	defer putTSSShards(x)

	shards := x.shards
	tmpLabels := promutils.GetLabels()
	for _, ts := range tssBlock {
		hashLabels := ts.Labels
		if len(shardByURLLabelsMap) > 0 {
			hashLabels = tmpLabels.Labels[:0]
			for _, label := range ts.Labels {
				if _, ok := shardByURLLabelsMap[label.Name]; ok {
					hashLabels = append(hashLabels, label)
				}
			}
			tmpLabels.Labels = hashLabels
		} else if len(shardByURLIgnoreLabelsMap) > 0 {
			hashLabels = tmpLabels.Labels[:0]
			for _, label := range ts.Labels {
				if _, ok := shardByURLIgnoreLabelsMap[label.Name]; !ok {
					hashLabels = append(hashLabels, label)
				}
			}
			tmpLabels.Labels = hashLabels
		}
		h := getLabelsHash(hashLabels)
		idx := h % uint64(len(shards))
		i := 0
		for {
			shards[idx] = append(shards[idx], ts)
			i++
			if i >= replicas {
				break
			}
			idx++
			if idx >= uint64(len(shards)) {
				idx = 0
			}
		}
	}
	promutils.PutLabels(tmpLabels)

	// Push sharded samples to remote storage systems in parallel in order to reduce
	// the time needed for sending the data to multiple remote storage systems.
	var wg sync.WaitGroup
	var anyPushFailed atomic.Bool
	for i, rwctx := range rwctxs {
		shard := shards[i]
		if len(shard) == 0 {
			continue
		}
		wg.Add(1)
		go func(rwctx *remoteWriteCtx, tss []prompbmarshal.TimeSeries) {
			defer wg.Done()
			if !rwctx.TryPush(tss) {
				anyPushFailed.Store(true)
			}
		}(rwctx, shard)
	}
	wg.Wait()
	return !anyPushFailed.Load()
}

type tssShards struct {
	shards [][]prompbmarshal.TimeSeries
}

func getTSSShards(n int) *tssShards {
	v := tssShardsPool.Get()
	if v == nil {
		v = &tssShards{}
	}
	x := v.(*tssShards)
	if cap(x.shards) < n {
		x.shards = make([][]prompbmarshal.TimeSeries, n)
	}
	x.shards = x.shards[:n]
	return x
}

func putTSSShards(x *tssShards) {
	shards := x.shards
	for i := range shards {
		clear(shards[i])
		shards[i] = shards[i][:0]
	}
	tssShardsPool.Put(x)
}

var tssShardsPool sync.Pool

// sortLabelsIfNeeded sorts labels if -sortLabels command-line flag is set.
func sortLabelsIfNeeded(tss []prompbmarshal.TimeSeries) {
	if !*sortLabels {
		return
	}
	for i := range tss {
		promrelabel.SortLabels(tss[i].Labels)
	}
}

func limitSeriesCardinality(tss []prompbmarshal.TimeSeries) []prompbmarshal.TimeSeries {
	if hourlySeriesLimiter == nil && dailySeriesLimiter == nil {
		return tss
	}
	dst := make([]prompbmarshal.TimeSeries, 0, len(tss))
	for i := range tss {
		labels := tss[i].Labels
		h := getLabelsHash(labels)
		if hourlySeriesLimiter != nil && !hourlySeriesLimiter.Add(h) {
			hourlySeriesLimitRowsDropped.Add(len(tss[i].Samples))
			logSkippedSeries(labels, "-remoteWrite.maxHourlySeries", hourlySeriesLimiter.MaxItems())
			continue
		}
		if dailySeriesLimiter != nil && !dailySeriesLimiter.Add(h) {
			dailySeriesLimitRowsDropped.Add(len(tss[i].Samples))
			logSkippedSeries(labels, "-remoteWrite.maxDailySeries", dailySeriesLimiter.MaxItems())
			continue
		}
		dst = append(dst, tss[i])
	}
	return dst
}

var (
	hourlySeriesLimiter *bloomfilter.Limiter
	dailySeriesLimiter  *bloomfilter.Limiter

	hourlySeriesLimitRowsDropped = metrics.NewCounter(`vmagent_hourly_series_limit_rows_dropped_total`)
	dailySeriesLimitRowsDropped  = metrics.NewCounter(`vmagent_daily_series_limit_rows_dropped_total`)
)

func getLabelsHash(labels []prompbmarshal.Label) uint64 {
	bb := labelsHashBufPool.Get()
	b := bb.B[:0]
	for _, label := range labels {
		b = append(b, label.Name...)
		b = append(b, label.Value...)
	}
	h := xxhash.Sum64(b)
	bb.B = b
	labelsHashBufPool.Put(bb)
	return h
}

var labelsHashBufPool bytesutil.ByteBufferPool

func logSkippedSeries(labels []prompbmarshal.Label, flagName string, flagValue int) {
	select {
	case <-logSkippedSeriesTicker.C:
		// Do not use logger.WithThrottler() here, since this will increase CPU usage
		// because every call to logSkippedSeries will result to a call to labelsToString.
		logger.Warnf("skip series %s because %s=%d reached", labelsToString(labels), flagName, flagValue)
	default:
	}
}

var logSkippedSeriesTicker = time.NewTicker(5 * time.Second)

func labelsToString(labels []prompbmarshal.Label) string {
	var b []byte
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
	return string(b)
}

var (
	globalRowsPushedBeforeRelabel = metrics.NewCounter("vmagent_remotewrite_global_rows_pushed_before_relabel_total")
	rowsDroppedByGlobalRelabel    = metrics.NewCounter("vmagent_remotewrite_global_relabel_metrics_dropped_total")
)

type remoteWriteCtx struct {
	idx int
	fq  *persistentqueue.FastQueue
	c   *client

	sas          atomic.Pointer[streamaggr.Aggregators]
	deduplicator *streamaggr.Deduplicator

	streamAggrKeepInput bool
	streamAggrDropInput bool

	pss        []*pendingSeries
	pssNextIdx atomic.Uint64

	rowsPushedAfterRelabel *metrics.Counter
	rowsDroppedByRelabel   *metrics.Counter
}

func newRemoteWriteCtx(argIdx int, remoteWriteURL *url.URL, maxInmemoryBlocks int, sanitizedURL string) *remoteWriteCtx {
	// strip query params, otherwise changing params resets pq
	pqURL := *remoteWriteURL
	pqURL.RawQuery = ""
	pqURL.Fragment = ""
	h := xxhash.Sum64([]byte(pqURL.String()))
	queuePath := filepath.Join(*tmpDataPath, persistentQueueDirname, fmt.Sprintf("%d_%016X", argIdx+1, h))
	maxPendingBytes := maxPendingBytesPerURL.GetOptionalArg(argIdx)
	if maxPendingBytes != 0 && maxPendingBytes < persistentqueue.DefaultChunkFileSize {
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4195
		logger.Warnf("rounding the -remoteWrite.maxDiskUsagePerURL=%d to the minimum supported value: %d", maxPendingBytes, persistentqueue.DefaultChunkFileSize)
		maxPendingBytes = persistentqueue.DefaultChunkFileSize
	}
	fq := persistentqueue.MustOpenFastQueue(queuePath, sanitizedURL, maxInmemoryBlocks, maxPendingBytes, *disableOnDiskQueue)
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vmagent_remotewrite_pending_data_bytes{path=%q, url=%q}`, queuePath, sanitizedURL), func() float64 {
		return float64(fq.GetPendingBytes())
	})
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vmagent_remotewrite_pending_inmemory_blocks{path=%q, url=%q}`, queuePath, sanitizedURL), func() float64 {
		return float64(fq.GetInmemoryQueueLen())
	})
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vmagent_remotewrite_queue_blocked{path=%q, url=%q}`, queuePath, sanitizedURL), func() float64 {
		if fq.IsWriteBlocked() {
			return 1
		}
		return 0
	})

	var c *client
	switch remoteWriteURL.Scheme {
	case "http", "https":
		c = newHTTPClient(argIdx, remoteWriteURL.String(), sanitizedURL, fq, *queues)
	default:
		logger.Fatalf("unsupported scheme: %s for remoteWriteURL: %s, want `http`, `https`", remoteWriteURL.Scheme, sanitizedURL)
	}
	c.init(argIdx, *queues, sanitizedURL)

	// Initialize pss
	sf := significantFigures.GetOptionalArg(argIdx)
	rd := roundDigits.GetOptionalArg(argIdx)
	pssLen := *queues
	if n := cgroup.AvailableCPUs(); pssLen > n {
		// There is no sense in running more than availableCPUs concurrent pendingSeries,
		// since every pendingSeries can saturate up to a single CPU.
		pssLen = n
	}
	pss := make([]*pendingSeries, pssLen)
	for i := range pss {
		pss[i] = newPendingSeries(fq, c.useVMProto, sf, rd)
	}

	rwctx := &remoteWriteCtx{
		idx: argIdx,
		fq:  fq,
		c:   c,
		pss: pss,

		rowsPushedAfterRelabel: metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_rows_pushed_after_relabel_total{path=%q, url=%q}`, queuePath, sanitizedURL)),
		rowsDroppedByRelabel:   metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_relabel_metrics_dropped_total{path=%q, url=%q}`, queuePath, sanitizedURL)),
	}

	// Initialize sas
	sasFile := streamAggrConfig.GetOptionalArg(argIdx)
	dedupInterval := streamAggrDedupInterval.GetOptionalArg(argIdx)
	ignoreOldSamples := streamAggrIgnoreOldSamples.GetOptionalArg(argIdx)
	if sasFile != "" {
		opts := &streamaggr.Options{
			DedupInterval:        dedupInterval,
			DropInputLabels:      *streamAggrDropInputLabels,
			IgnoreOldSamples:     ignoreOldSamples,
			IgnoreFirstIntervals: *streamAggrIgnoreFirstIntervals,
		}
		sas, err := streamaggr.LoadFromFile(sasFile, rwctx.pushInternalTrackDropped, opts)
		if err != nil {
			logger.Fatalf("cannot initialize stream aggregators from -remoteWrite.streamAggr.config=%q: %s", sasFile, err)
		}
		rwctx.sas.Store(sas)
		rwctx.streamAggrKeepInput = streamAggrKeepInput.GetOptionalArg(argIdx)
		rwctx.streamAggrDropInput = streamAggrDropInput.GetOptionalArg(argIdx)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_successful{path=%q}`, sasFile)).Set(1)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_success_timestamp_seconds{path=%q}`, sasFile)).Set(fasttime.UnixTimestamp())
	} else if dedupInterval > 0 {
		rwctx.deduplicator = streamaggr.NewDeduplicator(rwctx.pushInternalTrackDropped, dedupInterval, *streamAggrDropInputLabels)
	}

	return rwctx
}

func (rwctx *remoteWriteCtx) MustStop() {
	// sas and deduplicator must be stopped before rwctx is closed
	// because sas can write pending series to rwctx.pss if there are any
	sas := rwctx.sas.Swap(nil)
	sas.MustStop()

	if rwctx.deduplicator != nil {
		rwctx.deduplicator.MustStop()
		rwctx.deduplicator = nil
	}

	for _, ps := range rwctx.pss {
		ps.MustStop()
	}
	rwctx.idx = 0
	rwctx.pss = nil
	rwctx.fq.UnblockAllReaders()
	rwctx.c.MustStop()
	rwctx.c = nil

	rwctx.fq.MustClose()
	rwctx.fq = nil

	rwctx.rowsPushedAfterRelabel = nil
	rwctx.rowsDroppedByRelabel = nil
}

func (rwctx *remoteWriteCtx) TryPush(tss []prompbmarshal.TimeSeries) bool {
	// Apply relabeling
	var rctx *relabelCtx
	var v *[]prompbmarshal.TimeSeries
	rcs := allRelabelConfigs.Load()
	pcs := rcs.perURL[rwctx.idx]
	if pcs.Len() > 0 {
		rctx = getRelabelCtx()
		// Make a copy of tss before applying relabeling in order to prevent
		// from affecting time series for other remoteWrite.url configs.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/467
		// and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/599
		v = tssPool.Get().(*[]prompbmarshal.TimeSeries)
		tss = append(*v, tss...)
		rowsCountBeforeRelabel := getRowsCount(tss)
		tss = rctx.applyRelabeling(tss, pcs)
		rowsCountAfterRelabel := getRowsCount(tss)
		rwctx.rowsDroppedByRelabel.Add(rowsCountBeforeRelabel - rowsCountAfterRelabel)
	}
	rowsCount := getRowsCount(tss)
	rwctx.rowsPushedAfterRelabel.Add(rowsCount)

	// Apply stream aggregation or deduplication if they are configured
	sas := rwctx.sas.Load()
	if sas != nil {
		matchIdxs := matchIdxsPool.Get()
		matchIdxs.B = sas.Push(tss, matchIdxs.B)
		if !rwctx.streamAggrKeepInput {
			if rctx == nil {
				rctx = getRelabelCtx()
				// Make a copy of tss before dropping aggregated series
				v = tssPool.Get().(*[]prompbmarshal.TimeSeries)
				tss = append(*v, tss...)
			}
			tss = dropAggregatedSeries(tss, matchIdxs.B, rwctx.streamAggrDropInput)
		}
		matchIdxsPool.Put(matchIdxs)
	} else if rwctx.deduplicator != nil {
		rwctx.deduplicator.Push(tss)
		tss = tss[:0]
	}

	// Try pushing the data to remote storage
	ok := rwctx.tryPushInternal(tss)

	// Return back relabeling contexts to the pool
	if rctx != nil {
		*v = prompbmarshal.ResetTimeSeries(tss)
		tssPool.Put(v)
		putRelabelCtx(rctx)
	}

	return ok
}

var matchIdxsPool bytesutil.ByteBufferPool

func dropAggregatedSeries(src []prompbmarshal.TimeSeries, matchIdxs []byte, dropInput bool) []prompbmarshal.TimeSeries {
	dst := src[:0]
	if !dropInput {
		for i, match := range matchIdxs {
			if match == 1 {
				continue
			}
			dst = append(dst, src[i])
		}
	}
	tail := src[len(dst):]
	clear(tail)
	return dst
}

func (rwctx *remoteWriteCtx) pushInternalTrackDropped(tss []prompbmarshal.TimeSeries) {
	if rwctx.tryPushInternal(tss) {
		return
	}
	if !*disableOnDiskQueue {
		logger.Panicf("BUG: tryPushInternal must return true if -remoteWrite.disableOnDiskQueue isn't set")
	}
	pushFailures.Inc()
	if *dropSamplesOnOverload {
		rowsCount := getRowsCount(tss)
		samplesDropped.Add(rowsCount)
	}
}

func (rwctx *remoteWriteCtx) tryPushInternal(tss []prompbmarshal.TimeSeries) bool {
	var rctx *relabelCtx
	var v *[]prompbmarshal.TimeSeries
	if len(labelsGlobal) > 0 {
		// Make a copy of tss before adding extra labels in order to prevent
		// from affecting time series for other remoteWrite.url configs.
		rctx = getRelabelCtx()
		v = tssPool.Get().(*[]prompbmarshal.TimeSeries)
		tss = append(*v, tss...)
		rctx.appendExtraLabels(tss, labelsGlobal)
	}

	pss := rwctx.pss
	idx := rwctx.pssNextIdx.Add(1) % uint64(len(pss))

	ok := pss[idx].TryPush(tss)

	if rctx != nil {
		*v = prompbmarshal.ResetTimeSeries(tss)
		tssPool.Put(v)
		putRelabelCtx(rctx)
	}

	return ok
}

func (rwctx *remoteWriteCtx) reinitStreamAggr() {
	sasFile := streamAggrConfig.GetOptionalArg(rwctx.idx)
	if sasFile == "" {
		// There is no stream aggregation for rwctx
		return
	}

	logger.Infof("reloading stream aggregation configs pointed by -remoteWrite.streamAggr.config=%q", sasFile)
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reloads_total{path=%q}`, sasFile)).Inc()
	opts := &streamaggr.Options{
		DedupInterval:    streamAggrDedupInterval.GetOptionalArg(rwctx.idx),
		DropInputLabels:  *streamAggrDropInputLabels,
		IgnoreOldSamples: streamAggrIgnoreOldSamples.GetOptionalArg(rwctx.idx),
	}
	sasNew, err := streamaggr.LoadFromFile(sasFile, rwctx.pushInternalTrackDropped, opts)
	if err != nil {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reloads_errors_total{path=%q}`, sasFile)).Inc()
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_successful{path=%q}`, sasFile)).Set(0)
		logger.Errorf("cannot reload stream aggregation config from -remoteWrite.streamAggr.config=%q; continue using the previously loaded config; error: %s", sasFile, err)
		return
	}
	sas := rwctx.sas.Load()
	if !sasNew.Equal(sas) {
		sasOld := rwctx.sas.Swap(sasNew)
		sasOld.MustStop()
		logger.Infof("successfully reloaded stream aggregation configs at -remoteWrite.streamAggr.config=%q", sasFile)
	} else {
		sasNew.MustStop()
		logger.Infof("the config at -remoteWrite.streamAggr.config=%q wasn't changed", sasFile)
	}
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_successful{path=%q}`, sasFile)).Set(1)
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_success_timestamp_seconds{path=%q}`, sasFile)).Set(fasttime.UnixTimestamp())
}

var tssPool = &sync.Pool{
	New: func() interface{} {
		a := []prompbmarshal.TimeSeries{}
		return &a
	},
}

func getRowsCount(tss []prompbmarshal.TimeSeries) int {
	rowsCount := 0
	for _, ts := range tss {
		rowsCount += len(ts.Samples)
	}
	return rowsCount
}

// CheckStreamAggrConfigs checks configs pointed by -remoteWrite.streamAggr.config
func CheckStreamAggrConfigs() error {
	pushNoop := func(_ []prompbmarshal.TimeSeries) {}
	for idx, sasFile := range *streamAggrConfig {
		if sasFile == "" {
			continue
		}
		opts := &streamaggr.Options{
			DedupInterval:    streamAggrDedupInterval.GetOptionalArg(idx),
			DropInputLabels:  *streamAggrDropInputLabels,
			IgnoreOldSamples: streamAggrIgnoreOldSamples.GetOptionalArg(idx),
		}
		sas, err := streamaggr.LoadFromFile(sasFile, pushNoop, opts)
		if err != nil {
			return fmt.Errorf("cannot load -remoteWrite.streamAggr.config=%q: %w", sasFile, err)
		}
		sas.MustStop()
	}
	return nil
}

func newMapFromStrings(a []string) map[string]struct{} {
	m := make(map[string]struct{}, len(a))
	for _, s := range a {
		m[s] = struct{}{}
	}
	return m
}
