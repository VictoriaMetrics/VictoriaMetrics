package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"

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
	isSilent     bool
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

	start, err := utils.GetTime(p.filter.TimeStart)
	if err != nil {
		return fmt.Errorf("failed to parse %s, provided: %s, error: %w", vmNativeFilterTimeStart, p.filter.TimeStart, err)
	}

	end := time.Now().In(start.Location())
	if p.filter.TimeEnd != "" {
		end, err = utils.GetTime(p.filter.TimeEnd)
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
		if !p.isSilent && !prompt(question) {
			return nil
		}
	}

	for _, tenantID := range tenants {
		err := p.runBackfilling(ctx, tenantID, ranges, p.isSilent)
		if err != nil {
			return fmt.Errorf("migration failed: %s", err)
		}
	}

	log.Println("Import finished!")
	log.Print(p.s)

	return nil
}

func (p *vmNativeProcessor) do(ctx context.Context, f native.Filter, srcURL, dstURL string, bar *pb.ProgressBar) error {

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

func (p *vmNativeProcessor) runSingle(ctx context.Context, f native.Filter, srcURL, dstURL string, bar *pb.ProgressBar) error {
	reader, err := p.src.ExportPipe(ctx, srcURL, f)
	if err != nil {
		return fmt.Errorf("failed to init export pipe: %w", err)
	}

	if p.disablePerMetricRequests && bar != nil {
		fmt.Printf("Continue import process with filter %s:\n", f.String())
		reader = bar.NewProxyReader(reader)
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

func (p *vmNativeProcessor) runBackfilling(ctx context.Context, tenantID string, ranges [][]time.Time, silent bool) error {
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

	barPrefix := "Requests to make"
	initMessage := "Initing import process from %q to %q with filter %s"
	initParams := []interface{}{srcURL, dstURL, p.filter.String()}
	if p.interCluster {
		barPrefix = fmt.Sprintf("Requests to make for tenant %s", tenantID)
		initMessage = "Initing import process from %q to %q with filter %s for tenant %s"
		initParams = []interface{}{srcURL, dstURL, p.filter.String(), tenantID}
	}

	fmt.Println("") // extra line for better output formatting
	log.Printf(initMessage, initParams...)

	var foundSeriesMsg string

	metrics := []string{p.filter.Match}
	if !p.disablePerMetricRequests {
		log.Printf("Exploring metrics...")
		metrics, err = p.src.Explore(ctx, p.filter, tenantID)
		if err != nil {
			return fmt.Errorf("cannot get metrics from source %s: %w", p.src.Addr, err)
		}

		if len(metrics) == 0 {
			errMsg := "no metrics found"
			if tenantID != "" {
				errMsg = fmt.Sprintf("%s for tenant id: %s", errMsg, tenantID)
			}
			log.Println(errMsg)
			return nil
		}
		foundSeriesMsg = fmt.Sprintf("Found %d metrics to import", len(metrics))
	}

	if !p.interCluster {
		// do not prompt for intercluster because there could be many tenants,
		// and we don't want to interrupt the process when moving to the next tenant.
		question := foundSeriesMsg + ". Continue?"
		if !silent && !prompt(question) {
			return nil
		}
	} else {
		log.Print(foundSeriesMsg)
	}

	processingMsg := fmt.Sprintf("Requests to make: %d", len(metrics)*len(ranges))
	if len(ranges) > 1 {
		processingMsg = fmt.Sprintf("Selected time range will be split into %d ranges according to %q step. %s", len(ranges), p.filter.Chunk, processingMsg)
	}
	log.Print(processingMsg)

	var bar *pb.ProgressBar
	if !silent {
		bar = barpool.NewSingleProgress(fmt.Sprintf(nativeWithBackoffTpl, barPrefix), len(metrics)*len(ranges))
		if p.disablePerMetricRequests {
			bar = barpool.NewSingleProgress(nativeSingleProcessTpl, 0)
		}
		bar.Start()
		defer bar.Finish()
	}

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
					if bar != nil {
						bar.Increment()
					}
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
	for _, s := range metrics {

		match, err := buildMatchWithFilter(p.filter.Match, s)
		if err != nil {
			logger.Errorf("failed to build export filters: %s", err)
			continue
		}

		for _, times := range ranges {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context canceled")
			case infErr := <-errCh:
				return fmt.Errorf("native error: %s", infErr)
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
