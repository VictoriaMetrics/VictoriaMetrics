package remotewrite

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metrics"
	xxhash "github.com/cespare/xxhash/v2"
)

var (
	remoteWriteURLs = flagutil.NewArray("remoteWrite.url", "Remote storage URL to write data to. It must support Prometheus remote_write API. "+
		"It is recommended using VictoriaMetrics as remote storage. Example url: http://<victoriametrics-host>:8428/api/v1/write . "+
		"Pass multiple -remoteWrite.url flags in order to write data concurrently to multiple remote storage systems")
	tmpDataPath = flag.String("remoteWrite.tmpDataPath", "vmagent-remotewrite-data", "Path to directory where temporary data for remote write component is stored")
	queues      = flag.Int("remoteWrite.queues", 1, "The number of concurrent queues to each -remoteWrite.url. Set more queues if a single queue "+
		"isn't enough for sending high volume of collected data to remote storage")
	showRemoteWriteURL = flag.Bool("remoteWrite.showURL", false, "Whether to show -remoteWrite.url in the exported metrics. "+
		"It is hidden by default, since it can contain sensistive auth info")
	maxPendingBytesPerURL = flag.Int("remoteWrite.maxDiskUsagePerURL", 0, "The maximum file-based buffer size in bytes at -remoteWrite.tmpDataPath "+
		"for each -remoteWrite.url. When buffer size reaches the configured maximum, then old data is dropped when adding new data to the buffer. "+
		"Buffered data is stored in ~500MB chunks, so the minimum practical value for this flag is 500000000. "+
		"Disk usage is unlimited if the value is set to 0")
)

var rwctxs []*remoteWriteCtx

// Contains the current relabelConfigs.
var allRelabelConfigs atomic.Value

// Init initializes remotewrite.
//
// It must be called after flag.Parse().
//
// Stop must be called for graceful shutdown.
func Init() {
	if len(*remoteWriteURLs) == 0 {
		logger.Fatalf("at least one `-remoteWrite.url` must be set")
	}

	if !*showRemoteWriteURL {
		// remoteWrite.url can contain authentication codes, so hide it at `/metrics` output.
		httpserver.RegisterSecretFlag("remoteWrite.url")
	}
	initLabelsGlobal()
	rcs, err := loadRelabelConfigs()
	if err != nil {
		logger.Fatalf("cannot load relabel configs: %s", err)
	}
	allRelabelConfigs.Store(rcs)

	maxInmemoryBlocks := memory.Allowed() / len(*remoteWriteURLs) / maxRowsPerBlock / 100
	if maxInmemoryBlocks > 200 {
		// There is no much sense in keeping higher number of blocks in memory,
		// since this means that the producer outperforms consumer and the queue
		// will continue growing. It is better storing the queue to file.
		maxInmemoryBlocks = 200
	}
	if maxInmemoryBlocks < 2 {
		maxInmemoryBlocks = 2
	}
	for i, remoteWriteURL := range *remoteWriteURLs {
		urlLabelValue := fmt.Sprintf("secret-url-%d", i+1)
		if *showRemoteWriteURL {
			urlLabelValue = remoteWriteURL
		}
		rwctx := newRemoteWriteCtx(i, remoteWriteURL, maxInmemoryBlocks, urlLabelValue)
		rwctxs = append(rwctxs, rwctx)
	}

	// Start config reloader.
	sighupCh := procutil.NewSighupChan()
	configReloaderWG.Add(1)
	go func() {
		defer configReloaderWG.Done()
		for {
			select {
			case <-sighupCh:
			case <-stopCh:
				return
			}
			logger.Infof("SIGHUP received; reloading relabel configs pointed by -remoteWrite.relabelConfig and -remoteWrite.urlRelabelConfig")
			rcs, err := loadRelabelConfigs()
			if err != nil {
				logger.Errorf("cannot reload relabel configs; preserving the previous configs; error: %s", err)
				continue
			}
			allRelabelConfigs.Store(rcs)
			logger.Infof("Successfully reloaded relabel configs")
		}
	}()
}

var stopCh = make(chan struct{})
var configReloaderWG sync.WaitGroup

// Stop stops remotewrite.
//
// It is expected that nobody calls Push during and after the call to this func.
func Stop() {
	close(stopCh)
	configReloaderWG.Wait()

	for _, rwctx := range rwctxs {
		rwctx.MustStop()
	}
	rwctxs = nil
}

// Push sends wr to remote storage systems set via `-remoteWrite.url`.
//
// Note that wr may be modified by Push due to relabeling.
func Push(wr *prompbmarshal.WriteRequest) {
	var rctx *relabelCtx
	rcs := allRelabelConfigs.Load().(*relabelConfigs)
	prcsGlobal := rcs.global
	if len(prcsGlobal) > 0 || len(labelsGlobal) > 0 {
		rctx = getRelabelCtx()
	}
	tss := wr.Timeseries
	for len(tss) > 0 {
		// Process big tss in smaller blocks in order to reduce maxmimum memory usage
		tssBlock := tss
		if len(tssBlock) > maxRowsPerBlock {
			tssBlock = tss[:maxRowsPerBlock]
			tss = tss[maxRowsPerBlock:]
		} else {
			tss = nil
		}
		if rctx != nil {
			tssBlockLen := len(tssBlock)
			tssBlock = rctx.applyRelabeling(tssBlock, labelsGlobal, prcsGlobal)
			globalRelabelMetricsDropped.Add(tssBlockLen - len(tssBlock))
		}
		for _, rwctx := range rwctxs {
			rwctx.Push(tssBlock)
		}
		if rctx != nil {
			rctx.reset()
		}
	}
	if rctx != nil {
		putRelabelCtx(rctx)
	}
}

var globalRelabelMetricsDropped = metrics.NewCounter("vmagent_remotewrite_global_relabel_metrics_dropped_total")

type remoteWriteCtx struct {
	idx        int
	fq         *persistentqueue.FastQueue
	c          *client
	pss        []*pendingSeries
	pssNextIdx uint64

	tss []prompbmarshal.TimeSeries

	relabelMetricsDropped *metrics.Counter
}

func newRemoteWriteCtx(argIdx int, remoteWriteURL string, maxInmemoryBlocks int, urlLabelValue string) *remoteWriteCtx {
	h := xxhash.Sum64([]byte(remoteWriteURL))
	path := fmt.Sprintf("%s/persistent-queue/%016X", *tmpDataPath, h)
	fq := persistentqueue.MustOpenFastQueue(path, remoteWriteURL, maxInmemoryBlocks, *maxPendingBytesPerURL)
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vmagent_remotewrite_pending_data_bytes{path=%q, url=%q}`, path, urlLabelValue), func() float64 {
		return float64(fq.GetPendingBytes())
	})
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vmagent_remotewrite_pending_inmemory_blocks{path=%q, url=%q}`, path, urlLabelValue), func() float64 {
		return float64(fq.GetInmemoryQueueLen())
	})
	c := newClient(argIdx, remoteWriteURL, urlLabelValue, fq, *queues)
	pss := make([]*pendingSeries, *queues)
	for i := range pss {
		pss[i] = newPendingSeries(fq.MustWriteBlock)
	}
	return &remoteWriteCtx{
		idx: argIdx,
		fq:  fq,
		c:   c,
		pss: pss,

		relabelMetricsDropped: metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_relabel_metrics_dropped_total{path=%q, url=%q}`, path, urlLabelValue)),
	}
}

func (rwctx *remoteWriteCtx) MustStop() {
	for _, ps := range rwctx.pss {
		ps.MustStop()
	}
	rwctx.idx = 0
	rwctx.pss = nil
	rwctx.fq.MustClose()
	rwctx.fq = nil
	rwctx.c.MustStop()
	rwctx.c = nil

	rwctx.relabelMetricsDropped = nil
}

func (rwctx *remoteWriteCtx) Push(tss []prompbmarshal.TimeSeries) {
	var rctx *relabelCtx
	rcs := allRelabelConfigs.Load().(*relabelConfigs)
	prcs := rcs.perURL[rwctx.idx]
	if len(prcs) > 0 {
		// Make a copy of tss before applying relabeling in order to prevent
		// from affecting time series for other remoteWrite.url configs.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/467 for details.
		rwctx.tss = append(rwctx.tss[:0], tss...)
		tss = rwctx.tss
		rctx = getRelabelCtx()
		tssLen := len(tss)
		tss = rctx.applyRelabeling(tss, nil, prcs)
		rwctx.relabelMetricsDropped.Add(tssLen - len(tss))
	}
	pss := rwctx.pss
	idx := atomic.AddUint64(&rwctx.pssNextIdx, 1) % uint64(len(pss))
	pss[idx].Push(tss)
	if rctx != nil {
		putRelabelCtx(rctx)
		// Zero rwctx.tss in order to free up GC references.
		rwctx.tss = prompbmarshal.ResetTimeSeries(rwctx.tss)
	}
}
