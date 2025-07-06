package remotewrite

import (
	"flag"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage/netinsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
)

var (
	remoteWriteURLs = flagutil.NewArrayString("remoteWrite.url", "Remote storage URL to write data to. It must support VictoriaLogs native protocol. "+
		"Example url: http://<victorialogs-host>:9428/internal/insert. "+
		"Pass multiple -remoteWrite.url options in order to replicate the collected data to multiple remote storage systems.")
	maxPendingBytesPerURL = flagutil.NewArrayBytes("remoteWrite.maxDiskUsagePerURL", 0, "The maximum file-based buffer size in bytes at -remoteWrite.tmpDataPath "+
		"for each -remoteWrite.url. When buffer size reaches the configured maximum, then old data is dropped when adding new data to the buffer. "+
		"Buffered data is stored in ~500MB chunks. It is recommended to set the value for this flag to a multiple of the block size 500MB. "+
		"Disk usage is unlimited if the value is set to 0")

	tmpDataPath = flag.String("remoteWrite.tmpDataPath", "vlagent-remotewrite-data", "Path to directory for storing pending data, which isn't sent to the configured -remoteWrite.url . "+
		"See also -remoteWrite.maxDiskUsagePerURL")
	queues = flag.Int("remoteWrite.queues", cgroup.AvailableCPUs()*2, "The number of concurrent queues to each -remoteWrite.url. Set more queues if default number of queues "+
		"isn't enough for sending high volume of collected data to remote storage. "+
		"Default value depends on the number of available CPU cores. It should work fine in most cases since it minimizes resource usage")

	showRemoteWriteURL = flag.Bool("remoteWrite.showURL", false, "Whether to show -remoteWrite.url in the exported metrics. "+
		"It is hidden by default, since it can contain sensitive info such as auth key")
)

// rwctxsGlobal contains statically populated entries when -remoteWrite.url is specified.
var rwctxsGlobal []*remoteWriteCtx

// Storage implements insertutil.LogRowsStorage interface
type Storage struct{}

// MustAddRows implements insertutil.LogRowsStorage interface
func (*Storage) MustAddRows(lr *logstorage.LogRows) {
	pushToRemoteStorages(lr)
}

// CanWriteData implements insertutil.LogRowsStorage interface
func (*Storage) CanWriteData() error {
	return nil
}

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

// Init initializes remotewrite.
//
// It must be called after flag.Parse().
//
// Stop must be called for graceful shutdown.
func Init() {
	if len(*remoteWriteURLs) == 0 {
		logger.Fatalf("at least one `-remoteWrite.url` command-line flag must be set")
	}
	if *queues > maxQueues {
		*queues = maxQueues
	}
	if *queues <= 0 {
		*queues = 1
	}
	initRemoteWriteCtxs(*remoteWriteURLs)
	dropDanglingQueues()
}

// Stop stops remotewrite.
//
// It is expected that nobody calls TryPush during and after the call to this func.
func Stop() {
	for _, rwctx := range rwctxsGlobal {
		rwctx.mustStop()
	}
	rwctxsGlobal = nil
}

func dropDanglingQueues() {
	// Remove dangling persistent queues, if any.
	// This is required for the case when the number of queues has been changed or URL have been changed.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4014
	//
	// In case if there were many persistent queues with identical *remoteWriteURLs
	// the queue with the last index will be dropped.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6140
	existingQueues := make(map[string]struct{}, len(rwctxsGlobal))
	for _, rwctx := range rwctxsGlobal {
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
		logger.Infof("removed %d dangling queues from %q, active queues: %d", removed, *tmpDataPath, len(rwctxsGlobal))
	}
}

func initRemoteWriteCtxs(urls []string) {
	if len(urls) == 0 {
		logger.Panicf("BUG: urls must be non-empty")
	}

	maxInmemoryBlocks := memory.Allowed() / len(urls) / 10000
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
	rwctxIdx := make([]int, len(urls))
	for i, remoteWriteURLRaw := range urls {
		remoteWriteURL, err := url.Parse(remoteWriteURLRaw)
		if err != nil {
			logger.Fatalf("invalid -remoteWrite.url=%q: %s", remoteWriteURL, err)
		}
		sanitizedURL := fmt.Sprintf("%d:secret-url", i+1)
		if *showRemoteWriteURL {
			sanitizedURL = fmt.Sprintf("%d:%s", i+1, remoteWriteURL)
		}
		rwctxs[i] = newRemoteWriteCtx(i, remoteWriteURL, maxInmemoryBlocks, sanitizedURL)
		rwctxIdx[i] = i
	}

	rwctxsGlobal = rwctxs
}

func pushToRemoteStorages(lr *logstorage.LogRows) {
	rwctxs := rwctxsGlobal
	if len(rwctxs) == 1 {
		// fast path
		rwctxs[0].push(lr)
		return
	}
	// Push samples to remote storage systems in parallel in order to reduce
	// the time needed for sending the data to multiple remote storage systems.
	var wg sync.WaitGroup
	for _, rwctx := range rwctxs {
		wg.Add(1)
		go func(rwctx *remoteWriteCtx) {
			defer wg.Done()
			rwctx.push(lr)

		}(rwctx)
	}
	wg.Wait()
}

type remoteWriteCtx struct {
	idx int
	fq  *persistentqueue.FastQueue
	c   *client

	pls        []*pendingLogs
	pssNextIdx atomic.Uint64
}

func newRemoteWriteCtx(argIdx int, remoteWriteURL *url.URL, maxInmemoryBlocks int, sanitizedURL string) *remoteWriteCtx {
	// protocol version is required by victoria-logs
	q := remoteWriteURL.Query()
	q.Set("version", netinsert.ProtocolVersion)
	remoteWriteURL.RawQuery = q.Encode()

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

	fq := persistentqueue.MustOpenFastQueue(queuePath, sanitizedURL, maxInmemoryBlocks, maxPendingBytes, false)
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vlagent_remotewrite_pending_data_bytes{path=%q, url=%q}`, queuePath, sanitizedURL), func() float64 {
		return float64(fq.GetPendingBytes())
	})
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vlagent_remotewrite_pending_inmemory_blocks{path=%q, url=%q}`, queuePath, sanitizedURL), func() float64 {
		return float64(fq.GetInmemoryQueueLen())
	})
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vlagent_remotewrite_queue_blocked{path=%q, url=%q}`, queuePath, sanitizedURL), func() float64 {
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
	plsLen := *queues
	if n := cgroup.AvailableCPUs(); plsLen > n {
		// There is no sense in running more than availableCPUs concurrent pendingLogs,
		// since every pendingLogs can saturate up to a single CPU.
		plsLen = n
	}
	pls := make([]*pendingLogs, plsLen)
	for i := range pls {
		pls[i] = newPendingLogs(fq)
	}

	rwctx := &remoteWriteCtx{
		idx: argIdx,
		fq:  fq,
		c:   c,
		pls: pls,
	}

	return rwctx
}

func (rwctx *remoteWriteCtx) push(lr *logstorage.LogRows) {
	pls := rwctx.pls
	idx := rwctx.pssNextIdx.Add(1) % uint64(len(pls))
	pls[idx].add(lr)
}

func (rwctx *remoteWriteCtx) mustStop() {
	for _, ps := range rwctx.pls {
		ps.mustStop()
	}
	rwctx.idx = 0
	rwctx.pls = nil
	rwctx.fq.UnblockAllReaders()
	rwctx.c.MustStop()
	rwctx.c = nil

	rwctx.fq.MustClose()
	rwctx.fq = nil
}
