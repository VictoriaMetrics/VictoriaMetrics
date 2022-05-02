package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/limiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

type vmNativeProcessor struct {
	filter    filter
	rateLimit int64

	dst     *vmNativeClient
	src     *vmNativeClient
	syncErr chan error
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

func (p *vmNativeProcessor) Close() {
	p.syncErr <- fmt.Errorf("process aborted")
}

func (p *vmNativeProcessor) run() error {
	pr, pw := io.Pipe()

	fmt.Printf("Initing export pipe from %q with filters: %s\n", p.src.addr, p.filter)
	exportReader, err := p.exportPipe()
	if err != nil {
		return fmt.Errorf("failed to init export pipe: %s", err)
	}

	nativeImportAddr, err := vm.AddExtraLabelsToImportPath(nativeImportAddr, p.dst.extraLabels)
	if err != nil {
		return err
	}

	go p.prepareImport(nativeImportAddr, pr)

	fmt.Printf("Initing import process to %q:\n", p.dst.addr)
	bar := barpool.AddWithTemplate(nativeBarTpl, 0)
	barReader := bar.NewProxyReader(exportReader)
	if err := barpool.Start(); err != nil {
		log.Printf("error start process bars pool: %s", err)
		return err
	}

	w := io.Writer(pw)
	if p.rateLimit > 0 {
		rl := limiter.NewLimiter(p.rateLimit)
		w = limiter.NewWriteLimiter(pw, rl)
	}

	go p.copyData(w, barReader, pw)

	for err := range p.syncErr {
		if err != nil {
			return err
		}
	}

	barpool.Stop()
	return nil
}

func (p *vmNativeProcessor) exportPipe() (io.ReadCloser, error) {
	u := fmt.Sprintf("%s/%s", p.src.addr, nativeExportAddr)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request to %q: %s", p.src.addr, err)
	}

	params := req.URL.Query()
	params.Set("match[]", p.filter.match)
	if p.filter.timeStart != "" {
		params.Set("start", p.filter.timeStart)
	}
	if p.filter.timeEnd != "" {
		params.Set("end", p.filter.timeEnd)
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

func (p *vmNativeProcessor) prepareImport(nativeImportAddr string, reader io.Reader) {
	u := fmt.Sprintf("%s/%s", p.dst.addr, nativeImportAddr)
	req, err := http.NewRequest("POST", u, reader)
	if err != nil {
		p.syncErr <- fmt.Errorf("cannot create import request to %q: %s", p.dst.addr, err)
	}
	importResp, err := p.dst.do(req, http.StatusNoContent)
	if err != nil {
		p.syncErr <- fmt.Errorf("import request failed: %s", err)
	} else if err := importResp.Body.Close(); err != nil {
		p.syncErr <- fmt.Errorf("cannot close import response body: %s", err)
	}
}

func (p *vmNativeProcessor) copyData(dst io.Writer, src io.Reader, writer *io.PipeWriter) {
	// io.Copy blocks, so we need close channel when all data is copied
	defer func() { close(p.syncErr) }()
	_, err := io.Copy(dst, src)
	if err != nil {
		p.syncErr <- fmt.Errorf("failed to write into %q: %s", p.dst.addr, err)
	}
	if err := writer.Close(); err != nil {
		p.syncErr <- err
	}
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
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body for status code %d: %s", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("unexpected response code %d: %s", resp.StatusCode, string(body))
	}
	return resp, err
}
