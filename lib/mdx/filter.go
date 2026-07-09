package mdx

import (
	"flag"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

var (
	vmLabel = flag.String("mdx.label", "", "Optional label value in the form 'name=value' used to identify VictoriaMetrics metrics for MDX. "+
		"Metrics containing the specified label are forwarded to `-remoteWrite.url` endpoints configured with `-remoteWrite.mdx.enable=true`.")
)

const (
	vmAppLabelName         = "victoriametrics_app"
	vmAppLabelValue        = "true"
	vmAppVersionMetricName = "vm_app_version"
)

// Ctx defines filtering context
type Ctx struct {
	// labels hold modified timeseries labels
	// valid until PutContext call
	labels []prompb.Label

	buf                  []byte
	hasVMAppLabel        bool
	hasVMAppVersionLabel bool
	hasFilterLabelValue  bool
	jobLabelValue        string
	instanceLabelValue   string
}

func (ctx *Ctx) reset() {
	// do not reset labels intentionally
	// it must live until PutContext call

	ctx.buf = ctx.buf[:0]
	ctx.hasVMAppLabel = false
	ctx.hasVMAppVersionLabel = false
	ctx.hasFilterLabelValue = false
	ctx.jobLabelValue = ""
	ctx.instanceLabelValue = ""
}

var ctxPool = &sync.Pool{
	New: func() any {
		return &Ctx{}
	},
}

// GetContext returns filtering context
func GetContext() *Ctx {
	return ctxPool.Get().(*Ctx)
}

// PutContext resets context
func PutContext(ctx *Ctx) {
	clear(ctx.labels)
	ctx.labels = ctx.labels[:0]
	ctx.reset()
	ctxPool.Put(ctx)
}

// Filter manages the list of VictoriaMetrics instances grouped by job:instance labels.
// job and instance must present at timeseries.
//
// Filter keeps timeseries with any of the following conditions:
// * vm_app_version present
// * victoriametrics_app=true label present at timeseries
// * if labels has label value defined with flag `-mdx.label`
//
// Filter track entries with TTL of 1 hour
type Filter struct {
	tracker           *instanceTracker
	filterByLabelName string
	label             string
}

// NewFilter returns new Filter instance
func NewFilter() *Filter {
	filter := &Filter{
		tracker: newInstanceTracker(),
	}
	if len(*vmLabel) > 0 {
		n := strings.IndexByte(*vmLabel, '=')
		if n < 0 {
			logger.Fatalf("missing '=' in `-mdx.label`. It must contain label in the form `name=value`; got %q", *vmLabel)
		}
		filter.filterByLabelName = (*vmLabel)[:n]
		filter.label = (*vmLabel)[n+1:]
		if len(filter.filterByLabelName) == 0 || len(filter.label) == 0 {
			logger.Fatalf("label name and value cannot be empty in `-mdx.label`. It must contain label in the form `name=value`; got %q", *vmLabel)
		}
	}

	return filter
}

// VMInstancesCount returns amount of currently tracked instances
func (filter *Filter) VMInstancesCount() int {
	return filter.tracker.len()
}

// MustStop stops filter instance
func (filter *Filter) MustStop() {
	filter.tracker.mustStop()
}

// Filter filters provided timeseries with given context.
//
// Returned timeseries is valid as long as Ctx is valid
func (filter *Filter) Filter(ctx *Ctx, tss []prompb.TimeSeries) []prompb.TimeSeries {
	dstTss := tss[:0]
	for _, ts := range tss {
		ctx.prepare(ts.Labels, filter.filterByLabelName, filter.label)
		key := ctx.formatTimeSeriesKey()
		if len(key) == 0 {
			// metrics with empty job or instance labels must be always dropped
			// despite any other conditions
			continue
		}
		if ctx.hasVMAppLabel {
			filter.trackInstance(key)
			dstTss = append(dstTss, ts)
			continue
		}
		if ctx.hasFilterLabelValue || ctx.hasVMAppVersionLabel {
			ts.Labels = ctx.addVMAppLabel(ts.Labels)
			filter.trackInstance(key)
			dstTss = append(dstTss, ts)
			continue
		}
		ok := filter.tracker.has(key)
		if ok {
			ts.Labels = ctx.addVMAppLabel(ts.Labels)
			dstTss = append(dstTss, ts)
		}
	}
	return dstTss
}

func (filter *Filter) trackInstance(key string) {
	if filter.tracker.has(key) {
		return
	}
	key = strings.Clone(key)
	filter.tracker.register(key)
}

func (ctx *Ctx) prepare(labels []prompb.Label, filterByLabelName, label string) {
	ctx.reset()

	// always use the last label=value pair
	// because in case of possible label duplicates,
	// the last added label must win
	for _, l := range labels {
		switch l.Name {
		case "job":
			ctx.jobLabelValue = l.Value
		case "instance":
			ctx.instanceLabelValue = l.Value
		case vmAppLabelName:
			if l.Value == vmAppLabelValue {
				ctx.hasVMAppLabel = true
			}
		case "__name__":
			if l.Value == vmAppVersionMetricName {
				ctx.hasVMAppVersionLabel = true
			}
		}
		if len(filterByLabelName) > 0 {
			if l.Name == filterByLabelName && l.Value == label {
				ctx.hasFilterLabelValue = true
			}
		}
	}
}

// formatTimeSeriesKey returns timeseries key after ctx.prepare call
// if it catched job and instances labels
//
// returned string is valid until next ctx.prepare
func (ctx *Ctx) formatTimeSeriesKey() string {
	if len(ctx.jobLabelValue) == 0 || len(ctx.instanceLabelValue) == 0 {
		return ""
	}
	buf := ctx.buf[:0]
	buf = strconv.AppendQuote(buf, ctx.jobLabelValue)
	buf = append(buf, ':')
	buf = strconv.AppendQuote(buf, ctx.instanceLabelValue)
	ctx.buf = buf
	return bytesutil.ToUnsafeString(buf)
}

func (ctx *Ctx) addVMAppLabel(labels []prompb.Label) []prompb.Label {
	// unconditionally add vmAppLabelValue at the end of labels list
	// it will overwrite any exist vmAppLabelName labels with a value different to vmAppLabelValue
	// it's guaranteed by VictoriaMetrics ingestion contract
	poolLabels := ctx.labels
	poolLabelsLen := len(poolLabels)
	poolLabels = append(poolLabels, labels...)
	poolLabels = append(poolLabels, prompb.Label{Name: vmAppLabelName, Value: vmAppLabelValue})
	ctx.labels = poolLabels
	return poolLabels[poolLabelsLen:len(poolLabels):len(poolLabels)]
}

type instanceTracker struct {
	mu              sync.RWMutex
	lastAccessByKey map[string]*atomic.Uint64
	wg              sync.WaitGroup
	stop            chan struct{}
}

func newInstanceTracker() *instanceTracker {
	c := &instanceTracker{
		lastAccessByKey: make(map[string]*atomic.Uint64),
		stop:            make(chan struct{}),
	}
	c.wg.Add(1)
	go c.startStaleWatcher()
	return c
}

func (it *instanceTracker) len() int {
	it.mu.RLock()
	s := len(it.lastAccessByKey)
	it.mu.RUnlock()
	return s
}

func (it *instanceTracker) has(key string) bool {
	it.mu.RLock()
	lat, ok := it.lastAccessByKey[key]
	it.mu.RUnlock()
	if ok {
		lat.Store(fasttime.UnixTimestamp())
	}
	return ok
}

func (it *instanceTracker) register(key string) {
	it.mu.Lock()
	// key could be registered by concurrent goroutine
	lat, ok := it.lastAccessByKey[key]
	if !ok {
		lat = &atomic.Uint64{}
		it.lastAccessByKey[key] = lat
	}
	it.mu.Unlock()
	lat.Store(fasttime.UnixTimestamp())
}

func (it *instanceTracker) startStaleWatcher() {
	defer it.wg.Done()

	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-it.stop:
			return
		case <-t.C:
			it.cleanStale()
		}
	}
}

var entryTTLSeconds = uint64(time.Hour.Seconds())

func (it *instanceTracker) cleanStale() {
	ct := fasttime.UnixTimestamp()
	var toDelete map[string]*atomic.Uint64

	it.mu.RLock()
	for key, lastAccessTime := range it.lastAccessByKey {
		accessedAt := lastAccessTime.Load()
		if ct > accessedAt+entryTTLSeconds {
			if toDelete == nil {
				toDelete = make(map[string]*atomic.Uint64)
			}
			toDelete[key] = lastAccessTime
		}
	}
	it.mu.RUnlock()

	if len(toDelete) > 0 {
		it.mu.Lock()
		for key, lastAccessTime := range toDelete {
			accessedAt := lastAccessTime.Load()
			// concurrent goroutine may refresh lastAccessTime
			if ct > accessedAt+entryTTLSeconds {
				delete(it.lastAccessByKey, key)
			}
		}
		it.mu.Unlock()
	}
}

func (it *instanceTracker) mustStop() {
	close(it.stop)
	it.wg.Wait()
}
