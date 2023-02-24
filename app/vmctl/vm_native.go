package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
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

	s            *native.Stats
	rateLimit    int64
	interCluster bool
	cc           int
}

// ResetStats resets im stats.
func (p *vmNativeProcessor) ResetStats() {
	p.s = &native.Stats{
		StartTime: time.Now(),
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

	fmt.Printf("Init series discovery process on time range %s - %s \n", p.filter.TimeStart, p.filter.TimeEnd)
	series, err := p.src.Explore(ctx, p.filter)
	if err != nil {
		return fmt.Errorf("cannot get series from source %s database: %s", p.src.Addr, err)
	}
	fmt.Printf("Discovered %d series \n", len(series))

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

	ranges := [][]time.Time{{startOfRange, endOfRange}}
	if p.filter.Chunk != "" {
		r, err := stepper.SplitDateRange(startOfRange, endOfRange, p.filter.Chunk)
		if err != nil {
			return fmt.Errorf("failed to create date ranges for the given time filters: %v", err)
		}
		ranges = ranges[:0]
		ranges = append(ranges, r...)
	}

	fmt.Printf("Initing import process from %q to %q with filter %s \n", p.src.Addr, p.dst.Addr, p.filter.String())
	var bar *pb.ProgressBar
	if !silent {
		bar = barpool.AddWithTemplate(fmt.Sprintf(nativeBarTpl, "Processing series (series * time ranges)"), len(series)*len(ranges))
		if err := barpool.Start(); err != nil {
			return err
		}
	}

	p.ResetStats()

	filterCh := make(chan native.Filter)
	errCh := make(chan error)

	var wg sync.WaitGroup
	wg.Add(p.cc)
	for i := 0; i < p.cc; i++ {
		go func() {
			defer wg.Done()
			for f := range filterCh {
				if err := p.do(ctx, f); err != nil {
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
				Match:     s,
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

	if !silent {
		barpool.Stop()
	}
	log.Println("Import finished!")
	log.Print(p.Stats())

	return nil
}

func (p *vmNativeProcessor) do(ctx context.Context, f native.Filter) error {
	nativeImportAddr, err := vm.AddExtraLabelsToImportPath(nativeImportAddr, p.dst.ExtraLabels)

	if err != nil {
		return fmt.Errorf("failed to add labels to import path: %s", err)
	}

	if !p.interCluster {
		srcURL := fmt.Sprintf("%s/%s", p.src.Addr, nativeExportAddr)
		dstURL := fmt.Sprintf("%s/%s", p.dst.Addr, nativeImportAddr)

		retryableFunc := func() error { return p.runSingle(ctx, f, srcURL, dstURL) }
		attempts, err := p.backoff.Retry(ctx, retryableFunc)
		p.s.Lock()
		p.s.Retries += attempts
		p.s.Unlock()
		if err != nil {
			return fmt.Errorf("failed to migrate %s from %s to %s: %s, after attempts: %d", f.String(), srcURL, dstURL, err, attempts)
		}
		return nil
	}

	tenants, err := p.src.GetSourceTenants(ctx, f)
	if err != nil {
		return fmt.Errorf("failed to get source tenants: %s", err)
	}

	log.Printf("Discovered tenants: %v", tenants)
	for _, tenant := range tenants {
		// src and dst expected formats: http://vminsert:8480/ and http://vmselect:8481/
		srcURL := fmt.Sprintf("%s/select/%s/prometheus/%s", p.src.Addr, tenant, nativeExportAddr)
		dstURL := fmt.Sprintf("%s/insert/%s/prometheus/%s", p.dst.Addr, tenant, nativeImportAddr)

		log.Printf("Initing export pipe from %q with filters: %s\n", srcURL, f)
		retryableFunc := func() error { return p.runSingle(ctx, f, srcURL, dstURL) }
		attempts, err := p.backoff.Retry(ctx, retryableFunc)
		p.s.Lock()
		p.s.Retries += attempts
		p.s.Unlock()
		if err != nil {
			return fmt.Errorf("failed to migrate %s for tenant %q: %s, after attempts: %d", f.String(), tenant, err, attempts)
		}
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
	p.s.Bytes += uint64(written)
	p.s.Requests++
	p.s.Unlock()

	if err := pw.Close(); err != nil {
		return err
	}
	<-done

	return nil
}
