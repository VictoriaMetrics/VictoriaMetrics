package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/limiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/cheggaaa/pb/v3"
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
}

// ResetStats resets im stats.
func (p *vmNativeProcessor) ResetStats() {
	p.s = &stats{
		startTime: time.Now(),
	}
}

// Stats returns im stats.
func (p *vmNativeProcessor) Stats() string {
	return p.s.String()
}

const (
	nativeExportAddr = "api/v1/export/native"
	nativeImportAddr = "api/v1/import/native"
	nativeBarTpl     = `{{ blue "%s:" }} {{ counters . }} {{ bar . "[" "█" (cycle . "█") "▒" "]" }} {{ percent . }}`
)

func (p *vmNativeProcessor) run(ctx context.Context, silent bool) error {
	if p.cc == 0 {
		p.cc = 1
	}

	startOfRange, err := time.Parse(time.RFC3339, p.filter.TimeStart)
	if err != nil {
		return fmt.Errorf("failed to parse %s, provided: %s, expected format: %s, error: %v", vmNativeFilterTimeStart, p.filter.TimeStart, time.RFC3339, err)
	}

	endOfRange := time.Now().In(startOfRange.Location())
	if p.filter.TimeEnd != "" {
		endOfRange, err = time.Parse(time.RFC3339, p.filter.TimeEnd)
		if err != nil {
			return fmt.Errorf("failed to parse %s, provided: %s, expected format: %s, error: %v", vmNativeFilterTimeEnd, p.filter.TimeEnd, time.RFC3339, err)
		}
	}

	tenants := []string{""}
	if p.interCluster {
		log.Printf("Init search for cluster tenants")
		sourceTenants, err := p.src.GetSourceTenants(ctx, p.filter)
		if err != nil {
			return fmt.Errorf("failed to get tenants: %s", err)

		}
		log.Printf("Discovered %s tenants", sourceTenants)
		tenants = tenants[:0]
		tenants = append(tenants, sourceTenants...)
	}

	ranges := [][]time.Time{{startOfRange, endOfRange}}
	if p.filter.Chunk != "" {
		r, err := stepper.SplitDateRange(startOfRange, endOfRange, p.filter.Chunk)
		if err != nil {
			return fmt.Errorf("failed to create date ranges for the given time filters: %v", err)
		}
		ranges = ranges[:0]
		ranges = append(ranges, r...)
	}

	p.ResetStats()

	for _, tenantID := range tenants {
		err := p.runBackfilling(ctx, tenantID, ranges, silent)
		if err != nil {
			log.Printf("error run migration process on tenant %s: %s", tenantID, err)
			continue
		}
	}

	log.Println("Import finished!")
	log.Print(p.Stats())

	return nil
}

func (p *vmNativeProcessor) do(ctx context.Context, f native.Filter, srcURL, dstURL string) error {

	retryableFunc := func() error { return p.runSingle(ctx, f, srcURL, dstURL) }
	attempts, err := p.backoff.Retry(ctx, retryableFunc)
	p.s.Lock()
	p.s.retries += attempts
	p.s.Unlock()
	if err != nil {
		return fmt.Errorf("failed to migrate %s from %s to %s: %s, after attempts: %d", f.String(), srcURL, dstURL, err, attempts)
	}

	return nil
}

func (p *vmNativeProcessor) runSingle(ctx context.Context, f native.Filter, srcURL, dstURL string) error {

	exportReader, err := p.src.ExportPipe(ctx, srcURL, f)
	if err != nil {
		return fmt.Errorf("failed to init export pipe: %s", err)
	}

	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer func() { close(done) }()
		if err := p.dst.ImportPipe(ctx, dstURL, pr); err != nil {
			logger.Errorf("error initialize import pipe: %s", err)
			return
		}
	}()

	w := io.Writer(pw)
	if p.rateLimit > 0 {
		rl := limiter.NewLimiter(p.rateLimit)
		w = limiter.NewWriteLimiter(pw, rl)
	}

	written, err := io.Copy(w, exportReader)
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
	<-done

	return nil
}

func (p *vmNativeProcessor) runBackfilling(ctx context.Context, tenantID string, ranges [][]time.Time, silent bool) error {

	nativeImportAddr, err := vm.AddExtraLabelsToImportPath(nativeImportAddr, p.dst.ExtraLabels)

	if err != nil {
		return fmt.Errorf("failed to add labels to import path: %s", err)
	}

	srcURL := fmt.Sprintf("%s/%s", p.src.Addr, nativeExportAddr)
	dstURL := fmt.Sprintf("%s/%s", p.dst.Addr, nativeImportAddr)
	if p.interCluster {
		srcURL = fmt.Sprintf("%s/select/%s/prometheus/%s", p.src.Addr, tenantID, nativeExportAddr)
		dstURL = fmt.Sprintf("%s/insert/%s/prometheus/%s", p.dst.Addr, tenantID, nativeImportAddr)
	}

	barPrefix := "Processing part of work:"
	initMessage := "Initing import process from %q to %q with filter %s \n"
	initParams := []interface{}{srcURL, dstURL, p.filter.String()}
	if p.interCluster {
		barPrefix = fmt.Sprintf("Processing part of work for tenant %s", tenantID)
		initMessage = "Initing import process from %q to %q with filter %s for tenant %s \n"
		initParams = []interface{}{srcURL, dstURL, p.filter.String(), tenantID}
	}

	log.Printf(initMessage, initParams...)

	log.Printf("Init explore series")
	series, err := p.src.Explore(ctx, p.filter, tenantID)
	if err != nil {
		return fmt.Errorf("cannot get series from source %s database: %s", p.src.Addr, err)
	}

	if len(series) == 0 {
		return fmt.Errorf("series not found")
	}

	foundSeriesMsg := fmt.Sprintf("Found %d timeseries to import. \n", len(series))
	rangesMsg := fmt.Sprintf("Selected time range will be split into %d ranges according to %q step. \n", len(ranges), p.filter.Chunk)
	processingPartsMsg := fmt.Sprintf("Export requests to make: %d \n", len(series)*len(ranges))
	log.Printf("%s. %s. %s.", foundSeriesMsg, rangesMsg, processingPartsMsg)

	var bar *pb.ProgressBar
	if !silent {
		bar = pb.ProgressBarTemplate(fmt.Sprintf(nativeBarTpl, barPrefix)).New(len(series) * len(ranges))
		bar.Start()
		defer bar.Finish()
	}

	filterCh := make(chan native.Filter)
	errCh := make(chan error)

	var wg sync.WaitGroup
	for i := 0; i < p.cc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range filterCh {
				if err := p.do(ctx, f, srcURL, dstURL); err != nil {
					errCh <- fmt.Errorf("request failed for: %s", err)
					return
				}
				if bar != nil {
					bar.Increment()
				}
			}
		}()
	}

	// any error breaks the import
	for s := range series {
		for _, times := range ranges {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context canceled")
			case infErr := <-errCh:
				return fmt.Errorf("native error: %s", infErr)
			case filterCh <- native.Filter{
				Match:     fmt.Sprintf("{%s=%q}", nameLabel, s),
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
