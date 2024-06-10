package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/limiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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

	start, err := utils.ParseTime(p.filter.TimeStart)
	if err != nil {
		return fmt.Errorf("failed to parse %s, provided: %s, error: %w", vmNativeFilterTimeStart, p.filter.TimeStart, err)
	}

	end := time.Now().In(start.Location())
	if p.filter.TimeEnd != "" {
		end, err = utils.ParseTime(p.filter.TimeEnd)
		if err != nil {
			return fmt.Errorf("failed to parse %s, provided: %s, error: %w", vmNativeFilterTimeEnd, p.filter.TimeEnd, err)
		}
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
		if !prompt(question) {
			return nil
		}
	}

	for _, tenantID := range tenants {
		err := p.runBackfilling(ctx, tenantID, ranges)
		if err != nil {
			return fmt.Errorf("migration failed: %s", err)
		}
	}

	log.Println("Import finished!")
	log.Print(p.s)

	return nil
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
	reader, err := p.src.ExportPipe(ctx, srcURL, f)
	if err != nil {
		return fmt.Errorf("failed to init export pipe: %w", err)
	}

	if p.disablePerMetricRequests {
		pr := bar.NewProxyReader(reader)
		if pr != nil {
			reader = bar.NewProxyReader(reader)
			fmt.Printf("Continue import process with filter %s:\n", f.String())
		}
	}

	pr, pw := io.Pipe()
	importCh := make(chan error)
	go func() {
		importCh <- p.dst.ImportPipe(ctx, dstURL, pr)
		close(importCh)
	}()

	w := io.Writer(pw)
	if p.rateLimit > 0 {
		rl := limiter.NewLimiter(p.rateLimit)
		w = limiter.NewWriteLimiter(pw, rl)
	}

	written, err := io.Copy(w, reader)
	if err != nil {
		return fmt.Errorf("failed to write into %q: %s", p.dst.Addr, err)
	}

	p.s.Lock()
	p.s.bytes += uint64(written)
	p.s.requests++
	p.s.Unlock()

	if err := pw.Close(); err != nil {
		return err
	}

	return <-importCh
}

func (p *vmNativeProcessor) runBackfilling(ctx context.Context, tenantID string, ranges [][]time.Time) error {
	exportAddr := nativeExportAddr
	importAddr := nativeImportAddr
	if p.isNative {
		exportAddr += "/native"
		importAddr += "/native"
	}
	srcURL := fmt.Sprintf("%s/%s", p.src.Addr, exportAddr)

	importAddr, err := vm.AddExtraLabelsToImportPath(importAddr, p.dst.ExtraLabels)
	if err != nil {
		return fmt.Errorf("failed to add labels to import path: %s", err)
	}
	dstURL := fmt.Sprintf("%s/%s", p.dst.Addr, importAddr)

	if p.interCluster {
		srcURL = fmt.Sprintf("%s/select/%s/prometheus/%s", p.src.Addr, tenantID, exportAddr)
		dstURL = fmt.Sprintf("%s/insert/%s/prometheus/%s", p.dst.Addr, tenantID, importAddr)
	}

	initMessage := "Initing import process from %q to %q with filter %s"
	initParams := []interface{}{srcURL, dstURL, p.filter.String()}
	if p.interCluster {
		initMessage = "Initing import process from %q to %q with filter %s for tenant %s"
		initParams = []interface{}{srcURL, dstURL, p.filter.String(), tenantID}
	}

	fmt.Println("") // extra line for better output formatting
	log.Printf(initMessage, initParams...)
	if len(ranges) > 1 {
		log.Printf("Selected time range will be split into %d ranges according to %q step", len(ranges), p.filter.Chunk)
	}

	var foundSeriesMsg string
	var requestsToMake int
	var metrics = map[string][][]time.Time{
		"": ranges,
	}
	if !p.disablePerMetricRequests {
		metrics, err = p.explore(ctx, p.src, tenantID, ranges)
		if err != nil {
			return fmt.Errorf("failed to explore metric names: %s", err)
		}
		if len(metrics) == 0 {
			errMsg := "no metrics found"
			if tenantID != "" {
				errMsg = fmt.Sprintf("%s for tenant id: %s", errMsg, tenantID)
			}
			log.Println(errMsg)
			return nil
		}
		for _, m := range metrics {
			requestsToMake += len(m)
		}
		foundSeriesMsg = fmt.Sprintf("Found %d unique metric names to import. Total import/export requests to make %d", len(metrics), requestsToMake)
	}

	if !p.interCluster {
		// do not prompt for intercluster because there could be many tenants,
		// and we don't want to interrupt the process when moving to the next tenant.
		question := foundSeriesMsg + ". Continue?"
		if !prompt(question) {
			return nil
		}
	} else {
		log.Print(foundSeriesMsg)
	}

	barPrefix := "Requests to make"
	if p.interCluster {
		barPrefix = fmt.Sprintf("Requests to make for tenant %s", tenantID)
	}

	bar := barpool.NewSingleProgress(fmt.Sprintf(nativeWithBackoffTpl, barPrefix), requestsToMake)
	if p.disablePerMetricRequests {
		bar = barpool.NewSingleProgress(nativeSingleProcessTpl, 0)
	}
	bar.Start()
	defer bar.Finish()

	filterCh := make(chan native.Filter)
	errCh := make(chan error, p.cc)

	var wg sync.WaitGroup
	for i := 0; i < p.cc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range filterCh {
				if !p.disablePerMetricRequests {
					if err := p.do(ctx, f, srcURL, dstURL, nil); err != nil {
						errCh <- err
						return
					}
					bar.Increment()
				} else {
					if err := p.runSingle(ctx, f, srcURL, dstURL, bar); err != nil {
						errCh <- err
						return
					}
				}
			}
		}()
	}

	// any error breaks the import
	for mName, mRanges := range metrics {
		match, err := buildMatchWithFilter(p.filter.Match, mName)
		if err != nil {
			logger.Errorf("failed to build filter %q for metric name %q: %s", p.filter.Match, mName, err)
			continue
		}

		for _, times := range mRanges {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context canceled")
			case infErr := <-errCh:
				return fmt.Errorf("export/import error: %s", infErr)
			case filterCh <- native.Filter{
				Match:     match,
				TimeStart: times[0].Format(time.RFC3339),
				TimeEnd:   times[1].Format(time.RFC3339),
			}:
			}
		}
	}

	close(filterCh)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		return fmt.Errorf("import process failed: %s", err)
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
	if filter == metricName {
		return filter, nil
	}
	nameFilter := fmt.Sprintf("__name__=%q", metricName)

	tfss, err := searchutils.ParseMetricSelector(filter)
	if err != nil {
		return "", err
	}

	var filters []string
	for _, tfs := range tfss {
		var a []string
		for _, tf := range tfs {
			if len(tf.Key) == 0 {
				continue
			}
			a = append(a, tf.String())
		}
		a = append(a, nameFilter)
		filters = append(filters, strings.Join(a, ","))
	}

	match := "{" + strings.Join(filters, " or ") + "}"
	return match, nil
}
