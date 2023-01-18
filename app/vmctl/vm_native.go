package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/cheggaaa/pb/v3"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/limiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

var (
	vmNativeFilterMatch = flag.String("vm-native-filter-match", `{__name__!=""}`, "Time series selector to match series for export. For example, select {instance!=\"localhost\"} will "+
		"match all series with \"instance\" label different to \"localhost\".\n"+
		" See more details here https://github.com/VictoriaMetrics/VictoriaMetrics#how-to-export-data-in-native-format")
	vmNativeFilterTimeStart = flag.String("vm-native-filter-time-start", "", "The time filter may contain either unix timestamp in seconds or RFC3339 values. E.g. '2020-01-01T20:07:00Z'")
	vmNativeFilterTimeEnd   = flag.String("vm-native-filter-time-end", "", "The time filter may contain either unix timestamp in seconds or RFC3339 values. E.g. '2020-01-01T20:07:00Z'")
	vmNativeStepInterval    = flag.String("vm-native-step-interval", "", fmt.Sprintf("Split export data into chunks. Requires setting --%s. Valid values are '%s','%s','%s','%s'.", "-vm-native-filter-time-start", stepper.StepMonth, stepper.StepDay, stepper.StepHour, stepper.StepMinute))

	vmNativeSrcAddr = flag.String("vm-native-src-addr", "", "VictoriaMetrics address to perform export from. \n"+
		" Should be the same as --httpListenAddr value for single-node version or vmselect component."+
		" If exporting from cluster version see https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format")
	vmNativeSrcUser     = flag.String("vm-native-src-user", "", "VictoriaMetrics username for basic auth")
	vmNativeSrcPassword = flag.String("vm-native-src-password", "", "VictoriaMetrics password for basic auth")

	vmNativeDstAddr = flag.String("vm-native-dst-addr", "", "VictoriaMetrics address to perform import to. \n"+
		" Should be the same as --httpListenAddr value for single-node version or vminsert component."+
		" If importing into cluster version see https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format")
	vmNativeDstUser     = flag.String("vm-native-dst-user", "", "VictoriaMetrics username for basic auth")
	vmNativeDstPassword = flag.String("vm-native-dst-password", "", "VictoriaMetrics password for basic auth")
)

type vmNativeProcessor struct {
	filter    filter
	rateLimit int64

	dst          *vmNativeClient
	src          *vmNativeClient
	interCluster bool
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

	nativeBarTpl = `Total: {{counters . }} {{ cycle . "↖" "↗" "↘" "↙" }} Speed: {{speed . }} {{string . "suffix"}}`
)

func (p *vmNativeProcessor) run(ctx context.Context) error {
	if p.filter.chunk == "" {
		return p.runWithFilter(ctx, p.filter)
	}

	startOfRange, err := time.Parse(time.RFC3339, p.filter.timeStart)
	if err != nil {
		return fmt.Errorf("failed to parse %s, provided: %s, expected format: %s, error: %v", *vmNativeFilterTimeStart, p.filter.timeStart, time.RFC3339, err)
	}

	var endOfRange time.Time
	if p.filter.timeEnd != "" {
		endOfRange, err = time.Parse(time.RFC3339, p.filter.timeEnd)
		if err != nil {
			return fmt.Errorf("failed to parse %s, provided: %s, expected format: %s, error: %v", *vmNativeFilterTimeEnd, p.filter.timeEnd, time.RFC3339, err)
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
		err := p.runWithFilter(ctx, f)

		if err != nil {
			log.Printf("processing failed for range %d/%d: %s - %s \n", rangeIdx+1, len(ranges), formattedStartTime, formattedEndTime)
			return err
		}
	}
	return nil
}

func (p *vmNativeProcessor) runWithFilter(ctx context.Context, f filter) error {
	nativeImportAddr, err := vm.AddExtraLabelsToImportPath(nativeImportAddr, p.dst.extraLabels)

	if err != nil {
		return fmt.Errorf("failed to add labels to import path: %s", err)
	}

	if !p.interCluster {
		srcURL := fmt.Sprintf("%s/%s", p.src.addr, nativeExportAddr)
		dstURL := fmt.Sprintf("%s/%s", p.dst.addr, nativeImportAddr)

		return p.runSingle(ctx, f, srcURL, dstURL)
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

		if err := p.runSingle(ctx, f, srcURL, dstURL); err != nil {
			return fmt.Errorf("failed to migrate data for tenant %q: %s", tenant, err)
		}
	}

	return nil
}

func (p *vmNativeProcessor) runSingle(ctx context.Context, f filter, srcURL, dstURL string) error {
	log.Printf("Initing export pipe from %q with filters: %s\n", srcURL, f)

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

	fmt.Printf("Initing import process to %q:\n", dstURL)
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

func nativeImport(ctx context.Context) flagutil.Action {
	return func(args []string) {
		fmt.Println("VictoriaMetrics Native import mode")

		if *vmNativeSrcAddr == "" {
			logger.Fatalf("flag --vm-native-src-addr cannot be empty")
		}

		if *vmNativeDstAddr == "" {
			logger.Fatalf("flag --vm-native-dst-addr cannot be empty")
		}

		err := flagutil.SetFlagsFromEnvironment()
		if err != nil {
			logger.Fatalf("error set flags from environment variables: %s", err)
		}

		if *vmNativeFilterMatch == "" {
			logger.Fatalf("flag %q can't be empty", *vmNativeFilterMatch)
		}

		p := vmNativeProcessor{
			rateLimit:    *vmRateLimit,
			interCluster: *vmInterCluster,
			filter: filter{
				match:     *vmNativeFilterMatch,
				timeStart: *vmNativeFilterTimeStart,
				timeEnd:   *vmNativeFilterTimeEnd,
				chunk:     *vmNativeStepInterval,
			},
			src: &vmNativeClient{
				addr:     strings.Trim(*vmNativeSrcAddr, "/"),
				user:     *vmNativeSrcUser,
				password: *vmNativeSrcPassword,
			},
			dst: &vmNativeClient{
				addr:        strings.Trim(*vmNativeDstAddr, "/"),
				user:        *vmNativeDstUser,
				password:    *vmNativeDstPassword,
				extraLabels: *vmExtraLabel,
			},
		}
		if err := p.run(ctx); err != nil {
			logger.Fatalf("error run vm-native process: %s", err)
		}
	}
}
