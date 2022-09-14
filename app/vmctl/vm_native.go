package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/dmitryk-dk/pb/v3"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/limiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

type vmNativeProcessor struct {
	filter    filter
	rateLimit int64

	dst *vmNativeClient
	src *vmNativeClient
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
	nativeExportAddr = "api/v1/export/native"
	nativeImportAddr = "api/v1/import/native"

	nativeBarTpl = `Total: {{counters . }} {{ cycle . "↖" "↗" "↘" "↙" }} Speed: {{speed . }} {{string . "suffix"}}`
)

func (p *vmNativeProcessor) run(ctx context.Context) error {
	if p.filter.chunk == "" {
		return p.runSingle(ctx, p.filter)
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
		formattedStartTime := r[0].Format(time.RFC3339)
		formattedEndTime := r[1].Format(time.RFC3339)
		log.Printf("Processing range %d/%d: %s - %s \n", rangeIdx+1, len(ranges), formattedStartTime, formattedEndTime)
		f := filter{
			match:     p.filter.match,
			timeStart: formattedStartTime,
			timeEnd:   formattedEndTime,
		}
		err := p.runSingle(ctx, f)

		if err != nil {
			log.Printf("processing failed for range %d/%d: %s - %s \n", rangeIdx+1, len(ranges), formattedStartTime, formattedEndTime)
			return err
		}
	}
	return nil
}

func (p *vmNativeProcessor) runSingle(ctx context.Context, f filter) error {
	pr, pw := io.Pipe()

	log.Printf("Initing export pipe from %q with filters: %s\n", p.src.addr, f)
	exportReader, err := p.exportPipe(ctx, f)
	if err != nil {
		return fmt.Errorf("failed to init export pipe: %s", err)
	}

	nativeImportAddr, err := vm.AddExtraLabelsToImportPath(nativeImportAddr, p.dst.extraLabels)
	if err != nil {
		return err
	}

	sync := make(chan struct{})
	go func() {
		defer func() { close(sync) }()
		u := fmt.Sprintf("%s/%s", p.dst.addr, nativeImportAddr)
		req, err := http.NewRequestWithContext(ctx, "POST", u, pr)
		if err != nil {
			log.Fatalf("cannot create import request to %q: %s", p.dst.addr, err)
		}
		importResp, err := p.dst.do(req, http.StatusNoContent)
		if err != nil {
			log.Fatalf("import request failed: %s", err)
		}
		if err := importResp.Body.Close(); err != nil {
			log.Fatalf("cannot close import response body: %s", err)
		}
	}()

	fmt.Printf("Initing import process to %q:\n", p.dst.addr)
	pool := pb.NewPool()
	bar := pb.ProgressBarTemplate(nativeBarTpl).New(0)
	pool.Add(bar)
	barReader := bar.NewProxyReader(exportReader)
	if err := pool.Start(); err != nil {
		log.Printf("error start process bars pool: %s", err)
		return err
	}
	defer func() {
		bar.Finish()
		if err := pool.Stop(); err != nil {
			fmt.Printf("failed to stop barpool: %+v\n", err)
		}
	}()

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

	log.Println("Import finished!")
	return nil
}

func (p *vmNativeProcessor) exportPipe(ctx context.Context, f filter) (io.ReadCloser, error) {
	u := fmt.Sprintf("%s/%s", p.src.addr, nativeExportAddr)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
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
