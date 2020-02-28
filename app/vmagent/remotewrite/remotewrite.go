package remotewrite

import (
	"flag"
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
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
)

// Init initializes remotewrite.
//
// It must be called after flag.Parse().
//
// Stop must be called for graceful shutdown.
func Init() {
	if len(*remoteWriteURLs) == 0 {
		logger.Panicf("FATAL: at least one `-remoteWrite.url` must be set")
	}

	if !*showRemoteWriteURL {
		// remoteWrite.url can contain authentication codes, so hide it at `/metrics` output.
		httpserver.RegisterSecretFlag("remoteWrite.url")
	}
	initRelabel()

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
		h := xxhash.Sum64([]byte(remoteWriteURL))
		path := fmt.Sprintf("%s/persistent-queue/%016X", *tmpDataPath, h)
		fq := persistentqueue.MustOpenFastQueue(path, remoteWriteURL, maxInmemoryBlocks)
		urlLabelValue := fmt.Sprintf("secret-url-%d", i+1)
		if *showRemoteWriteURL {
			urlLabelValue = remoteWriteURL
		}
		_ = metrics.NewGauge(fmt.Sprintf(`vmagent_remotewrite_pending_data_bytes{url=%q, hash="%016X"}`, urlLabelValue, h), func() float64 {
			return float64(fq.GetPendingBytes())
		})
		_ = metrics.NewGauge(fmt.Sprintf(`vmagent_remotewrite_pending_inmemory_blocks{url=%q}`, urlLabelValue), func() float64 {
			return float64(fq.GetInmemoryQueueLen())
		})
		c := newClient(remoteWriteURL, urlLabelValue, fq)
		fqs = append(fqs, fq)
		cs = append(cs, c)
	}

	pss = make([]*pendingSeries, *queues)
	for i := range pss {
		pss[i] = newPendingSeries(pushBlockToPersistentQueues)
	}
}

// Stop stops remotewrite.
//
// It is expected that nobody calls Push during and after the call to this func.
func Stop() {
	for _, ps := range pss {
		ps.MustStop()
	}

	// Close all the persistent queues. This should unblock clients waiting in MustReadBlock.
	for _, fq := range fqs {
		fq.MustClose()
	}
	fqs = nil

	// Stop all the clients
	for _, c := range cs {
		c.MustStop()
	}
	cs = nil

	resetRelabel()
}

// Push sends wr to remote storage systems set via `-remoteWrite.url`.
//
// Each timeseries in wr.Timeseries must contain one sample.
func Push(wr *prompbmarshal.WriteRequest) {
	rctx := relabelCtxPool.Get().(*relabelCtx)
	tss := wr.Timeseries
	for len(tss) > 0 {
		// Process big tss in smaller blocks in order to reduce maxmimum memory usage
		tssBlock := tss
		if len(tssBlock) > maxRowsPerBlock {
			tssBlock = tss[:maxRowsPerBlock]
			tss = tss[maxRowsPerBlock:]
		}
		tssBlock = rctx.applyRelabeling(tssBlock)
		idx := atomic.AddUint64(&pssNextIdx, 1) % uint64(len(pss))
		pss[idx].Push(tssBlock)
	}
	rctx.reset()
	relabelCtxPool.Put(rctx)
}

func pushBlockToPersistentQueues(block []byte) {
	for _, fq := range fqs {
		fq.MustWriteBlock(block)
	}
}

var fqs []*persistentqueue.FastQueue
var cs []*client

var pssNextIdx uint64
var pss []*pendingSeries
