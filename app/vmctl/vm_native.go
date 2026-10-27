package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/limiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/migrationstate"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vmctlutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

type vmNativeProcessor struct {
	filter native.Filter

	dst     *native.Client
	src     *native.Client
	backoff *backoff.Backoff

	s            *stats
	rateLimit    int64
	interCluster bool
	cc           int
	isNative     bool

	disablePerMetricRequests bool

	// stateDir is the directory for the migration state file. The current working directory is used if empty.
	stateDir string
	// srcHeaders and dstHeaders are --vm-native-src-headers and --vm-native-dst-headers values.
	// They identify the migration, since headers may select the tenant to read from or write to.
	srcHeaders string
	dstHeaders string
}

const (
	nativeExportAddr       = "api/v1/export"
	nativeImportAddr       = "api/v1/import"
	nativeWithBackoffTpl   = `{{ blue "%s:" }} {{ counters . }} {{ bar . "[" "█" (cycle . "█") "▒" "]" }} {{ percent . }}`
	nativeSingleProcessTpl = `Total: {{counters . }} {{ cycle . "↖" "↗" "↘" "↙" }} Speed: {{speed . }} {{string . "suffix"}}`
)

func (p *vmNativeProcessor) run(ctx context.Context) error {
	if p.cc == 0 {
		p.cc = 1
	}
	p.s = &stats{
		startTime: time.Now(),
	}

	start, err := vmctlutil.ParseTime(p.filter.TimeStart)
	if err != nil {
		return fmt.Errorf("failed to parse %s, provided: %s: %w", vmNativeFilterTimeStart, p.filter.TimeStart, err)
	}
	end, err := vmctlutil.ParseTime(p.filter.TimeEnd)
	if err != nil {
		return fmt.Errorf("failed to parse %s, provided: %s: %w", vmNativeFilterTimeEnd, p.filter.TimeEnd, err)
	}

	// The state isn't tracked without per-metric requests, since there is no distinct metric names to track.
	var st *migrationstate.State
	if !p.disablePerMetricRequests {
		st, err = p.loadState()
		if err != nil {
			return err
		}
		defer st.Close()
	}

	ranges := [][]time.Time{{start, end}}
	if p.filter.Chunk != "" {
		ranges, err = stepper.SplitDateRange(start, end, p.filter.Chunk, p.filter.TimeReverse)
		if err != nil {
			return fmt.Errorf("failed to create date ranges for the given time filters: %w", err)
		}
	}
	tenants := []string{""}
	if p.interCluster {
		log.Printf("Discovering tenants...")
		tenants, err = p.src.GetSourceTenants(ctx, p.filter)
		if err != nil {
			return fmt.Errorf("failed to get tenants: %w", err)
		}
		question := fmt.Sprintf("The following tenants were discovered: %s.\n Continue?", tenants)
		if !prompt(ctx, question) {
			return nil
		}
		migrationTenantsTotal.Set(uint64(len(tenants)))
	}

	for _, tenantID := range tenants {
		err := p.runBackfilling(ctx, st, tenantID, ranges)
		if err != nil {
			migrationErrorsTotal.Inc()
			return fmt.Errorf("migration failed: %w", err)
		}

		if p.interCluster {
			migrationTenantsProcessed.Inc()
		}
	}

	log.Println("Import finished!")
	log.Print(p.s)

	return nil
}

// nativeStateParams contains vm-native flag values, which identify a migration.
type nativeStateParams struct {
	SrcAddr           string   `json:"srcAddr"`
	DstAddr           string   `json:"dstAddr"`
	FilterMatch       string   `json:"filterMatch"`
	FilterTimeStart   string   `json:"filterTimeStart"`
	FilterTimeEnd     string   `json:"filterTimeEnd"`
	StepInterval      string   `json:"stepInterval"`
	FilterTimeReverse bool     `json:"filterTimeReverse"`
	InterCluster      bool     `json:"interCluster"`
	SrcExtraLabels    []string `json:"srcExtraLabels"`
	DstExtraLabels    []string `json:"dstExtraLabels"`
	// SrcHeadersHash and DstHeadersHash are hashes of the headers, since headers may contain secrets.
	SrcHeadersHash string `json:"srcHeadersHash"`
	DstHeadersHash string `json:"dstHeadersHash"`
}

// loadState loads the state of the migration, so the interrupted migration can be resumed.
func (p *vmNativeProcessor) loadState() (*migrationstate.State, error) {
	params := nativeStateParams{
		SrcAddr:           p.src.Addr,
		DstAddr:           p.dst.Addr,
		FilterMatch:       p.filter.Match,
		FilterTimeStart:   p.filter.TimeStart,
		FilterTimeEnd:     p.filter.TimeEnd,
		StepInterval:      p.filter.Chunk,
		FilterTimeReverse: p.filter.TimeReverse,
		InterCluster:      p.interCluster,
		SrcExtraLabels:    p.src.ExtraLabels,
		DstExtraLabels:    p.dst.ExtraLabels,
		SrcHeadersHash:    getHeadersHash(p.srcHeaders),
		DstHeadersHash:    getHeadersHash(p.dstHeaders),
	}
	name, err := getNativeStateName(params)
	if err != nil {
		return nil, err
	}
	if p.stateDir != "" {
		if err := os.MkdirAll(p.stateDir, 0o755); err != nil {
			return nil, fmt.Errorf("cannot create state dir: %w", err)
		}
	}
	st, err := migrationstate.Load(filepath.Join(p.stateDir, name), params)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	if n := st.DoneCount(); n > 0 {
		log.Printf("Resuming migration: %d requests done; state is stored at %q", n, st.Path())
	} else {
		log.Printf("Migration state is stored at %q", st.Path())
	}
	return st, nil
}

// getHeadersHash returns the hash of headers or empty string if headers are empty.
func getHeadersHash(headers string) string {
	if headers == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(headers))
	return hex.EncodeToString(sum[:8])
}

// getNativeStateName returns the state file name for params.
//
// The name is derived from params, so a run with changed flags starts a new migration.
func getNativeStateName(params nativeStateParams) (string, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("cannot marshal state params: %w", err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("vmctl-vm-native-%s.state", hex.EncodeToString(sum[:8])), nil
}

// newNativeUnit returns the unit of work for the request of metric mName of tenantID on times.
func newNativeUnit(tenantID, mName string, times []time.Time) *migrationstate.Unit {
	return &migrationstate.Unit{
		Tenant:     tenantID,
		MetricName: mName,
		TR: storage.TimeRange{
			MinTimestamp: times[0].UnixMilli(),
			MaxTimestamp: times[1].UnixMilli(),
		},
	}
}

// nativeRequest is a single export/import request.
type nativeRequest struct {
	f native.Filter
	// u is nil if the state isn't tracked.
	u *migrationstate.Unit
}

func (p *vmNativeProcessor) do(ctx context.Context, f native.Filter, srcURL, dstURL string, bar barpool.Bar) error {

	retryableFunc := func() error { return p.runSingle(ctx, f, srcURL, dstURL, bar) }
	attempts, err := p.backoff.Retry(ctx, retryableFunc)
	p.s.Lock()
	p.s.retries += attempts
	p.s.Unlock()
	if err != nil {
		return fmt.Errorf("failed to migrate from %s to %s (retry attempts: %d): %w\nwith filter %s", srcURL, dstURL, attempts, err, f)
	}

	return nil
}

func (p *vmNativeProcessor) runSingle(ctx context.Context, f native.Filter, srcURL, dstURL string, bar barpool.Bar) error {
	exportReader, err := p.src.ExportPipe(ctx, srcURL, f)
	if err != nil {
		return fmt.Errorf("failed to init export pipe: %w", err)
	}
	defer func() {
		// close the export reader on exit, so it doesn't hang at the source
		// until server-side timeout.
		_ = exportReader.Close()
	}()

	reader := io.Reader(exportReader)
	if p.disablePerMetricRequests {
		pr := bar.NewProxyReader(reader)
		if pr != nil {
			reader = pr
			fmt.Fprintf(log.Writer(), "Continue import process with filter %s:\n", f.String())
		}
	}

	pr, pw := io.Pipe()
	// make importCh buffered so goroutine won't get stuck if nothing reads from chan
	importCh := make(chan error, 1)
	go func() {
		importCh <- p.dst.ImportPipe(ctx, dstURL, pr)
		_ = pr.Close()
	}()

	w := io.Writer(pw)
	if p.rateLimit > 0 {
		rl := limiter.NewLimiter(p.rateLimit)
		w = limiter.NewWriteLimiter(pw, rl)
	}

	written, err := io.Copy(w, reader)
	if err != nil {
		// close the import writer, so the destination doesn't hang until server-side timeout
		_ = pw.CloseWithError(err)
		select {
		// check if the error happened in the ImportPipe
		case importErr := <-importCh:
			if importErr != nil {
				return fmt.Errorf("failed to import %s: %w", p.dst.Addr, importErr)
			}
		// or because vmctl has been stopped
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		return fmt.Errorf("failed to write into %q: %w", p.dst.Addr, err)
	}

	p.s.Lock()
	p.s.bytes += uint64(written)
	p.s.requests++
	p.s.Unlock()
	migrationBytesTransferredTotal.AddInt64(written)

	if err := pw.Close(); err != nil {
		return err
	}

	return <-importCh
}

// runBackfilling migrates data of tenantID on ranges. Requests already done according to st are skipped.
// st is nil if the state isn't tracked.
func (p *vmNativeProcessor) runBackfilling(ctx context.Context, st *migrationstate.State, tenantID string, ranges [][]time.Time) error {
	exportAddr := nativeExportAddr
	importAddr := nativeImportAddr
	if p.isNative {
		exportAddr += "/native"
		importAddr += "/native"
	}
	srcURL := fmt.Sprintf("%s/%s", p.src.Addr, exportAddr)

	importAddr, err := vm.AddExtraLabelsToImportPath(importAddr, p.dst.ExtraLabels)
	if err != nil {
		return fmt.Errorf("failed to add labels to import path: %w", err)
	}
	dstURL := fmt.Sprintf("%s/%s", p.dst.Addr, importAddr)

	if p.interCluster {
		srcURL = fmt.Sprintf("%s/select/%s/prometheus/%s", p.src.Addr, tenantID, exportAddr)
		dstURL = fmt.Sprintf("%s/insert/%s/prometheus/%s", p.dst.Addr, tenantID, importAddr)
	}

	initMessage := "Initing import process from %q to %q with filter %s"
	initParams := []any{srcURL, dstURL, p.filter.String()}
	if p.interCluster {
		initMessage = "Initing import process from %q to %q with filter %s for tenant %s"
		initParams = []any{srcURL, dstURL, p.filter.String(), tenantID}
	}

	fmt.Fprintln(log.Writer(), "") // extra line for better output formatting
	log.Printf(initMessage, initParams...)
	if len(ranges) > 1 {
		log.Printf("Selected time range will be split into %d ranges according to %q step", len(ranges), p.filter.Chunk)
	}

	var foundSeriesMsg string
	var requestsToMake, doneCount int
	var metricsMap = map[string][][]time.Time{
		"": ranges,
	}

	format := nativeSingleProcessTpl
	barPrefix := "Requests to make"
	if p.interCluster {
		barPrefix = fmt.Sprintf("Requests to make for tenant %s", tenantID)
	}

	if !p.disablePerMetricRequests {
		format = fmt.Sprintf(nativeWithBackoffTpl, barPrefix)
		metricsMap, err = p.explore(ctx, p.src, tenantID, ranges)
		if err != nil {
			return fmt.Errorf("failed to explore metric names: %w", err)
		}
		if len(metricsMap) == 0 {
			errMsg := "no metrics found"
			if tenantID != "" {
				errMsg = fmt.Sprintf("%s for tenant id: %s", errMsg, tenantID)
			}
			log.Println(errMsg)
			return nil
		}
		for mName, mRanges := range metricsMap {
			requestsToMake += len(mRanges)
			for _, times := range mRanges {
				if st.IsDone(newNativeUnit(tenantID, mName, times)) {
					doneCount++
				}
			}
		}
		foundSeriesMsg = fmt.Sprintf("Found %d unique metric names to import. Total import/export requests to make %d, %d already done (tracked in %q)",
			len(metricsMap), requestsToMake, doneCount, st.Path())

		migrationMetricsTotal.Add(len(metricsMap))
		if doneCount == requestsToMake {
			log.Printf("%s. Nothing to migrate", foundSeriesMsg)
			return nil
		}
	} else {
		requestsToMake = len(ranges)
	}

	if !p.interCluster {
		// do not prompt for intercluster because there could be many tenants,
		// and we don't want to interrupt the process when moving to the next tenant.
		question := foundSeriesMsg + ". Continue?"
		if !prompt(ctx, question) {
			return nil
		}
	} else {
		log.Print(foundSeriesMsg)
	}

	migrationRequestsPlanned.Add(requestsToMake - doneCount)
	bar := barpool.NewSingleProgress(format, requestsToMake)
	bar.Add(doneCount)
	bar.Start()
	defer bar.Finish()

	requestCh := make(chan nativeRequest)
	errCh := make(chan error, p.cc)

	var wg sync.WaitGroup
	for range p.cc {
		wg.Go(func() {
			for r := range requestCh {
				if !p.disablePerMetricRequests {
					if err := p.do(ctx, r.f, srcURL, dstURL, nil); err != nil {
						errCh <- err
						return
					}
					if err := st.MarkDone(r.u); err != nil {
						errCh <- err
						return
					}
					bar.Increment()
				} else {
					if err := p.runSingle(ctx, r.f, srcURL, dstURL, bar); err != nil {
						errCh <- err
						return
					}
				}
				migrationRequestsCompleted.Inc()
			}
		})
	}

	// any error breaks the import
	for mName, mRanges := range metricsMap {
		match, err := buildMatchWithFilter(p.filter.Match, mName)
		if err != nil {
			logger.Errorf("failed to build filter %q for metric name %q: %s", p.filter.Match, mName, err)
			continue
		}

		for _, times := range mRanges {
			r := nativeRequest{
				f: native.Filter{
					Match:     match,
					TimeStart: times[0].Format(time.RFC3339),
					TimeEnd:   times[1].Format(time.RFC3339),
				},
			}
			if st != nil {
				// st is nil if p.disablePerMetricRequests == true
				r.u = newNativeUnit(tenantID, mName, times)
				if st.IsDone(r.u) {
					continue
				}
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("context canceled")
			case infErr := <-errCh:
				return fmt.Errorf("export/import error: %w", infErr)
			case requestCh <- r:
			}
		}
		if !p.disablePerMetricRequests {
			migrationMetricsProcessed.Inc()
		}
	}

	close(requestCh)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		return fmt.Errorf("import process failed: %w", err)
	}

	return nil
}

func (p *vmNativeProcessor) explore(ctx context.Context, src *native.Client, tenantID string, ranges [][]time.Time) (map[string][][]time.Time, error) {
	log.Printf("Exploring metrics...")

	bar := barpool.NewSingleProgress(fmt.Sprintf(nativeWithBackoffTpl, "Explore requests to make"), len(ranges))
	bar.Start()
	defer bar.Finish()

	metrics := make(map[string][][]time.Time)
	for _, r := range ranges {
		ms, err := src.Explore(ctx, p.filter, tenantID, r[0], r[1])
		if err != nil {
			return nil, fmt.Errorf("cannot get metrics from %s on interval %v-%v: %w", src.Addr, r[0], r[1], err)
		}
		for i := range ms {
			metrics[ms[i]] = append(metrics[ms[i]], r)
		}
		bar.Increment()
	}
	return metrics, nil
}

// stats represents client statistic
// when processing data
type stats struct {
	sync.Mutex
	startTime time.Time
	bytes     uint64
	requests  uint64
	retries   uint64
}

func (s *stats) String() string {
	s.Lock()
	defer s.Unlock()

	totalImportDuration := time.Since(s.startTime)
	totalImportDurationS := totalImportDuration.Seconds()
	bytesPerS := byteCountSI(0)
	if s.bytes > 0 && totalImportDurationS > 0 {
		bytesPerS = byteCountSI(int64(float64(s.bytes) / totalImportDurationS))
	}

	return fmt.Sprintf("VictoriaMetrics importer stats:\n"+
		"  time spent while importing: %v;\n"+
		"  total bytes: %s;\n"+
		"  bytes/s: %s;\n"+
		"  requests: %d;\n"+
		"  requests retries: %d;",
		totalImportDuration,
		byteCountSI(int64(s.bytes)), bytesPerS,
		s.requests, s.retries)
}

func byteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

func buildMatchWithFilter(filter string, metricName string) (string, error) {
	tfss, err := searchutil.ParseMetricSelector(filter)
	if err != nil {
		return "", err
	}

	if filter == metricName || metricName == "" {
		return filter, nil
	}

	nameFilter := fmt.Sprintf("__name__=%q", metricName)

	var filters []string
	for _, tfs := range tfss {
		var a []string
		for _, tf := range tfs {
			if len(tf.Key) == 0 {
				continue
			}
			switch {
			case tf.IsNegative && tf.IsRegexp:
				a = append(a, fmt.Sprintf("%s!~%q", tf.Key, tf.Value))
			case tf.IsNegative:
				a = append(a, fmt.Sprintf("%s!=%q", tf.Key, tf.Value))
			case tf.IsRegexp:
				a = append(a, fmt.Sprintf("%s=~%q", tf.Key, tf.Value))
			default:
				a = append(a, fmt.Sprintf("%s=%q", tf.Key, tf.Value))
			}
		}
		a = append(a, nameFilter)
		filters = append(filters, strings.Join(a, ","))
	}

	match := "{" + strings.Join(filters, " or ") + "}"
	return match, nil
}

var (
	migrationMetricsTotal     = metrics.NewCounter(`vmctl_vm_native_migration_metrics_total`)
	migrationMetricsProcessed = metrics.NewCounter(`vmctl_vm_native_migration_metrics_processed`)

	migrationRequestsPlanned   = metrics.NewCounter(`vmctl_vm_native_migration_requests_planned`)
	migrationRequestsCompleted = metrics.NewCounter(`vmctl_vm_native_migration_requests_completed`)

	migrationErrorsTotal = metrics.NewCounter(`vmctl_vm_native_migration_errors_total`)

	migrationTenantsTotal     = metrics.NewCounter(`vmctl_vm_native_migration_tenants_total`)
	migrationTenantsProcessed = metrics.NewCounter(`vmctl_vm_native_migration_tenants_processed`)

	migrationBytesTransferredTotal = metrics.NewCounter(`vmctl_vm_native_migration_bytes_transferred_total`)
)
