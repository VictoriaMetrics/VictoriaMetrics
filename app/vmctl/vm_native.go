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

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/limiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/cheggaaa/pb/v3"
)

type vmNativeProcessor struct {
	filter    filter
	rateLimit int64

	dst          *vmNativeClient
	src          *vmNativeClient
	interCluster bool
	retry        *utils.Retry

	cc            int
	requestsLimit int
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

	nativeBarTpl = `Total: {{counters . }} {{ cycle . "↖" "↗" "↘" "↙" }} Speed: {{speed . }} {{string . "suffix"}}`
)

func (p *vmNativeProcessor) run(ctx context.Context) error {
	if p.cc == 0 {
		p.cc = 1
	}

	fmt.Printf("Init series discovery process on time range %s - %s \n", p.filter.timeStart, p.filter.timeEnd)
	series, err := p.explore(ctx, p.filter)
	if err != nil {
		return fmt.Errorf("cannot get series from source %s database: %s", p.src.addr, err)
	}
	fmt.Printf("Discovered %d series \n", len(series))

	pool := pb.NewPool()
	if err := pool.Start(); err != nil {
		log.Printf("error start process bars pool: %s", err)
		return err
	}
	bar := pb.ProgressBarTemplate(nativeBarTpl).New(0)
	pool.Add(bar)
	defer func() {
		bar.Finish()
		if err := pool.Stop(); err != nil {
			fmt.Printf("failed to stop barpool: %+v\n", err)
		}
	}()

	requestLimitC := make(chan struct{}, p.requestsLimit)
	var wg sync.WaitGroup

	if p.filter.chunk == "" {
		for s := range series {
			wg.Add(p.cc)
			f := filter{match: s, timeStart: p.filter.timeStart, timeEnd: p.filter.timeEnd}
			for i := 0; i < p.cc; i++ {
				requestLimitC <- struct{}{}
				go func() {
					defer func() {
						wg.Done()
						<-requestLimitC
					}()
					fmt.Printf("Initing export pipe from %q with filters: %s \n", p.src.addr, f)
					retryableFunc := func() error { return p.runWithFilter(ctx, bar, f) }
					attempts, err := p.retry.Do(ctx, retryableFunc)
					if err != nil {
						log.Fatalf("processing failed: %s - %s after %d attempts \n with error: %s", p.filter.timeStart, p.filter.timeEnd, attempts, err)
						return
					}
				}()
			}
		}
	}

	startOfRange, err := time.Parse(time.RFC3339, p.filter.timeStart)
	if err != nil {
		return fmt.Errorf("failed to parse %s, provided: %s, expected format: %s, error: %v", vmNativeFilterTimeStart, p.filter.timeStart, time.RFC3339, err)
	}

	var endOfRange time.Time
	if p.filter.timeEnd != "" {
		endOfRange, err = time.Parse(time.RFC3339, p.filter.timeEnd)
		if err != nil {
			return fmt.Errorf("failed to parse %s, provided: %s, expected format: %s, error: %v", vmNativeFilterTimeEnd, p.filter.timeEnd, time.RFC3339, err)
		}
	} else {
		endOfRange = time.Now()
	}

	ranges, err := stepper.SplitDateRange(startOfRange, endOfRange, p.filter.chunk)
	if err != nil {
		return fmt.Errorf("failed to create date ranges for the given time filters: %v", err)
	}

	for rangeIdx, r := range ranges {
		for s := range series {
			formattedStartTime := r[0].Format(time.RFC3339)
			formattedEndTime := r[1].Format(time.RFC3339)
			log.Printf("Processing range %d/%d: %s - %s for series: %s \n", rangeIdx+1, len(ranges), formattedStartTime, formattedEndTime, s)

			f := filter{match: s, timeStart: formattedStartTime, timeEnd: formattedEndTime}
			fmt.Printf("Initing export pipe from %q with filters: %s \n", p.src.addr, f)
			wg.Add(p.cc)
			for i := 0; i < p.cc; i++ {
				requestLimitC <- struct{}{}
				go func(f filter, i int) {
					defer func() {
						wg.Done()
						<-requestLimitC
						fmt.Printf("Import finished for: %s \n", f.String())
					}()
					retryableFunc := func() error { return p.runWithFilter(ctx, bar, f) }
					attempts, err := p.retry.Do(ctx, retryableFunc)
					if err != nil {
						log.Fatalf("processing failed for range %d/%d: %s - %s after %d attempts \n with error: %s", rangeIdx+1, len(ranges), f.timeStart, f.timeEnd, attempts, err)
						return
					}
				}(f, i)
			}
		}
	}

	wg.Wait()
	close(requestLimitC)

	return nil
}

func (p *vmNativeProcessor) runWithFilter(ctx context.Context, bar *pb.ProgressBar, f filter) error {
	nativeImportAddr, err := vm.AddExtraLabelsToImportPath(nativeImportAddr, p.dst.extraLabels)

	if err != nil {
		return fmt.Errorf("failed to add labels to import path: %s", err)
	}

	if !p.interCluster {
		srcURL := fmt.Sprintf("%s/%s", p.src.addr, nativeExportAddr)
		dstURL := fmt.Sprintf("%s/%s", p.dst.addr, nativeImportAddr)

		return p.runSingle(ctx, bar, f, srcURL, dstURL)
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

		if err := p.runSingle(ctx, bar, f, srcURL, dstURL); err != nil {
			return fmt.Errorf("failed to migrate data for tenant %q: %s", tenant, err)
		}
	}

	return nil
}

func (p *vmNativeProcessor) runSingle(ctx context.Context, bar *pb.ProgressBar, f filter, srcURL, dstURL string) error {

	exportReader, err := p.exportPipe(ctx, srcURL, f)
	if err != nil {
		return fmt.Errorf("failed to init export pipe: %s", err)
	}

	pr, pw := io.Pipe()
	sync := make(chan struct{})
	go func() {
		defer func() { close(sync) }()
		req, err := http.NewRequestWithContext(ctx, "POST", dstURL, pr)
		if err != nil {
			log.Printf("cannot create import request to %q: %s", p.dst.addr, err)
			return
		}
		importResp, err := p.dst.do(req, http.StatusNoContent)
		if err != nil {
			log.Printf("import request failed: %s", err)
			return
		}
		if err := importResp.Body.Close(); err != nil {
			log.Printf("cannot close import response body: %s", err)
			return
		}
	}()

	barReader := bar.NewProxyReader(exportReader)

	w := io.Writer(pw)
	if p.rateLimit > 0 {
		rl := limiter.NewLimiter(p.rateLimit)
		w = limiter.NewWriteLimiter(pw, rl)
	}

	_, err = io.Copy(w, barReader)
	if err != nil {
		return fmt.Errorf("failed to write into %q: %s", p.dst.addr, err)
	}

	if err := pw.Close(); err != nil {
		return err
	}
	<-sync

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
	u := fmt.Sprintf("%s/%s", p.src.addr, nativeSeriesAddr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
			if labelName == "__name__" {
				name := fmt.Sprintf("{__name__=%q}", labelValue)
				names[name] = struct{}{}
			}
		}
	}
	return names, nil
}
