package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/limiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/cheggaaa/pb/v3"
)

type vmNativeProcessor struct {
	filter    filter
	rateLimit int64

	dst          *vmNativeClient
	src          *vmNativeClient
	interCluster bool
	backoff      *backoff.Backoff

	s             *stats
	cc            int
	requestsLimit int
}

type stats struct {
	sync.Mutex
	bytes        uint64
	requests     uint64
	retries      uint64
	startTime    time.Time
	idleDuration time.Duration
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
		"  idle duration: %v;\n"+
		"  time spent while importing: %v;\n"+
		"  total bytes: %s;\n"+
		"  bytes/s: %s;\n"+
		"  import requests: %d;\n"+
		"  import requests retries: %d;",
		s.idleDuration, totalImportDuration,
		byteCountSI(int64(s.bytes)), bytesPerS,
		s.requests, s.retries)
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

type vmNativeClient struct {
	addr        string
	user        string
	password    string
	extraLabels []string
}

type filter struct {
	match     string
	timeStart string
	timeEnd   string
	chunk     string
}

func (f filter) String() string {
	s := fmt.Sprintf("\n\tfilter: match[]=%s", f.match)
	if f.timeStart != "" {
		s += fmt.Sprintf("\n\tstart: %s", f.timeStart)
	}
	if f.timeEnd != "" {
		s += fmt.Sprintf("\n\tend: %s", f.timeEnd)
	}
	return s
}

const (
	nativeExportAddr  = "api/v1/export/native"
	nativeImportAddr  = "api/v1/import/native"
	nativeTenantsAddr = "admin/tenants"
	nativeSeriesAddr  = "api/v1/series"
	nativeBarTpl      = `{{ blue "%s:" }} {{ counters . }} {{ bar . "[" "█" (cycle . "█") "▒" "]" }} {{ percent . }}`
)

func (p *vmNativeProcessor) run(ctx context.Context, silent bool) error {
	if p.cc == 0 {
		p.cc = 1
	}
	if p.requestsLimit == 0 {
		p.requestsLimit = 1
	}

	fmt.Printf("Init series discovery process on time range %s - %s \n", p.filter.timeStart, p.filter.timeEnd)
	series, err := p.explore(ctx, p.filter)
	if err != nil {
		return fmt.Errorf("cannot get series from source %s database: %s", p.src.addr, err)
	}
	fmt.Printf("Discovered %d series \n", len(series))

	startOfRange, err := time.Parse(time.RFC3339, p.filter.timeStart)
	if err != nil {
		return fmt.Errorf("failed to parse %s, provided: %s, expected format: %s, error: %v", vmNativeFilterTimeStart, p.filter.timeStart, time.RFC3339, err)
	}

	endOfRange := time.Now().In(startOfRange.Location())
	if p.filter.timeEnd != "" {
		endOfRange, err = time.Parse(time.RFC3339, p.filter.timeEnd)
		if err != nil {
			return fmt.Errorf("failed to parse %s, provided: %s, expected format: %s, error: %v", vmNativeFilterTimeEnd, p.filter.timeEnd, time.RFC3339, err)
		}
	}

	ranges := [][]time.Time{{startOfRange, endOfRange}}
	if p.filter.chunk != "" {
		r, err := stepper.SplitDateRange(startOfRange, endOfRange, p.filter.chunk)
		if err != nil {
			return fmt.Errorf("failed to create date ranges for the given time filters: %v", err)
		}
		ranges = ranges[:0]
		ranges = append(ranges, r...)
	}

	fmt.Printf("Initing import process from %q to %q on time period %s - %s \n", p.src.addr, p.dst.addr, startOfRange, endOfRange)
	var bar *pb.ProgressBar
	if !silent {
		bar = barpool.AddWithTemplate(fmt.Sprintf(nativeBarTpl, "Processing series"), len(series))
		if err := barpool.Start(); err != nil {
			return err
		}
	}

	p.ResetStats()

	filterCh := make(chan filter)
	errCh := make(chan error)
	requestLimitC := make(chan struct{}, p.requestsLimit)

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
				<-requestLimitC
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
				<-requestLimitC
				return fmt.Errorf("native error: %s", infErr)
			case requestLimitC <- struct{}{}:
				filterCh <- filter{
					match:     s,
					timeStart: times[0].Format(time.RFC3339),
					timeEnd:   times[1].Format(time.RFC3339),
				}
			}
		}
	}

	close(filterCh)
	wg.Wait()
	close(requestLimitC)
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

func (p *vmNativeProcessor) do(ctx context.Context, f filter) error {
	nativeImportAddr, err := vm.AddExtraLabelsToImportPath(nativeImportAddr, p.dst.extraLabels)

	if err != nil {
		return fmt.Errorf("failed to add labels to import path: %s", err)
	}

	if !p.interCluster {
		srcURL := fmt.Sprintf("%s/%s", p.src.addr, nativeExportAddr)
		dstURL := fmt.Sprintf("%s/%s", p.dst.addr, nativeImportAddr)

		retryableFunc := func() error { return p.runSingle(ctx, f, srcURL, dstURL) }
		attempts, err := p.backoff.Retry(ctx, retryableFunc)
		p.s.Lock()
		p.s.retries = attempts
		p.s.Unlock()
		if err != nil {
			return fmt.Errorf("failed to migrate %s from %s to %s: %s, after attempts: %d", f.String(), srcURL, dstURL, err, attempts)
		}
		return nil
	}

	tenants, err := p.getSourceTenants(ctx, f)
	if err != nil {
		return fmt.Errorf("failed to get source tenants: %s", err)
	}

	log.Printf("Discovered tenants: %v", tenants)
	for _, tenant := range tenants {
		// src and dst expected formats: http://vminsert:8480/ and http://vmselect:8481/
		srcURL := fmt.Sprintf("%s/select/%s/prometheus/%s", p.src.addr, tenant, nativeExportAddr)
		dstURL := fmt.Sprintf("%s/insert/%s/prometheus/%s", p.dst.addr, tenant, nativeImportAddr)

		log.Printf("Initing export pipe from %q with filters: %s\n", srcURL, f)
		retryableFunc := func() error { return p.runSingle(ctx, f, srcURL, dstURL) }
		attempts, err := p.backoff.Retry(ctx, retryableFunc)
		p.s.Lock()
		p.s.retries = attempts
		p.s.Unlock()
		if err != nil {
			return fmt.Errorf("failed to migrate %s for tenant %q: %s, after attempts: %d", f.String(), tenant, err, attempts)
		}
	}

	return nil
}

func (p *vmNativeProcessor) runSingle(ctx context.Context, f filter, srcURL, dstURL string) error {

	exportReader, err := p.exportPipe(ctx, srcURL, f)
	if err != nil {
		return fmt.Errorf("failed to init export pipe: %s", err)
	}

	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer func() { close(done) }()
		if err := p.importPipe(ctx, dstURL, pr); err != nil {
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
		return fmt.Errorf("failed to write into %q: %s", p.dst.addr, err)
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

func (p *vmNativeProcessor) getSourceTenants(ctx context.Context, f filter) ([]string, error) {
	u := fmt.Sprintf("%s/%s", p.src.addr, nativeTenantsAddr)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request to %q: %s", u, err)
	}

	params := req.URL.Query()
	if f.timeStart != "" {
		params.Set("start", f.timeStart)
	}
	if f.timeEnd != "" {
		params.Set("end", f.timeEnd)
	}
	req.URL.RawQuery = params.Encode()

	resp, err := p.src.do(req, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("tenants request failed: %s", err)
	}

	var r struct {
		Tenants []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("cannot decode tenants response: %s", err)
	}

	if err := resp.Body.Close(); err != nil {
		return nil, fmt.Errorf("cannot close tenants response body: %s", err)
	}

	return r.Tenants, nil
}

func (p *vmNativeProcessor) exportPipe(ctx context.Context, url string, f filter) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request to %q: %s", p.src.addr, err)
	}

	params := req.URL.Query()
	params.Set("match[]", f.match)
	if f.timeStart != "" {
		params.Set("start", f.timeStart)
	}
	if f.timeEnd != "" {
		params.Set("end", f.timeEnd)
	}
	req.URL.RawQuery = params.Encode()

	// disable compression since it is meaningless for native format
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := p.src.do(req, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("export request failed: %s", err)
	}
	return resp.Body, nil
}

func (c *vmNativeClient) do(req *http.Request, expSC int) (*http.Response, error) {
	if c.user != "" {
		req.SetBasicAuth(c.user, c.password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unexpected error when performing request: %s", err)
	}

	if resp.StatusCode != expSC {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body for status code %d: %s", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("unexpected response code %d: %s", resp.StatusCode, string(body))
	}
	return resp, err
}

type LabelValues map[string]string

type Response struct {
	Status string        `json:"status"`
	Series []LabelValues `json:"data"`
}

func (p *vmNativeProcessor) explore(ctx context.Context, f filter) (map[string]struct{}, error) {
	url := fmt.Sprintf("%s/%s", p.src.addr, nativeSeriesAddr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request to %q: %s", url, err)
	}

	params := req.URL.Query()
	if f.timeStart != "" {
		params.Set("start", f.timeStart)
	}
	if f.timeEnd != "" {
		params.Set("end", f.timeEnd)
	}
	params.Set("match[]", f.match)
	req.URL.RawQuery = params.Encode()

	resp, err := p.src.do(req, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("tenants request failed: %s", err)
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("cannot decode tenants response: %s", err)
	}

	if err := resp.Body.Close(); err != nil {
		return nil, fmt.Errorf("cannot close tenants response body: %s", err)
	}
	names := make(map[string]struct{})
	for _, series := range response.Series {
		for labelName, labelValue := range series {
			if labelName == nameLabel {
				name := fmt.Sprintf("{%s=%q}", nameLabel, labelValue)
				names[name] = struct{}{}
			}
		}
	}
	return names, nil
}

func (p *vmNativeProcessor) importPipe(ctx context.Context, dstURL string, pr *io.PipeReader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dstURL, pr)
	if err != nil {
		return fmt.Errorf("cannot create import request to %q: %s", p.dst.addr, err)
	}
	importResp, err := p.dst.do(req, http.StatusNoContent)
	if err != nil {
		return fmt.Errorf("import request failed: %s", err)
	}
	if err := importResp.Body.Close(); err != nil {
		return fmt.Errorf("cannot close import response body: %s", err)
	}
	return nil
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
