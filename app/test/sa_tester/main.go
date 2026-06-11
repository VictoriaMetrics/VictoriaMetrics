// sa_tester is a tool for testing stream aggregation rules.
//
// It does two things:
//  1. Serves a stream aggregation config YAML on GET /sa-config so that
//     vmagent can pull it with -streamAggr.config=http://host/sa-config.
//  2. On POST /start it calls vmagent's /-/reload (recording T=now),
//     then writes synthetic samples to the same vmagent.
//
// Usage:
//
//	go run ./app/test/sa_tester -config app/test/sa_tester/config.yaml
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/promremotewrite/stream"
	"gopkg.in/yaml.v2"
)

type AppConfig struct {
	// SARules holds the stream aggregation rules.  It is marshalled back to
	// YAML and served verbatim on GET /sa-config.
	SARules interface{} `yaml:"saRules"`

	// Interval between consecutive sample slots, e.g. "10s".
	Interval string `yaml:"interval"`

	// InputSeries defines the time series to generate.
	InputSeries []InputSeries `yaml:"input_series"`

	// VmagentAddress is the vmagent that loads SA rules and accepts raw samples.
	// POST /start calls <VmagentAddress>/-/reload, then writes samples to it.
	// Default: http://localhost:8429
	VmagentAddress string `yaml:"vmagent_address"`

	// ListenAddress is the HTTP listen address for this tester.
	// Default: :8080
	ListenAddress string `yaml:"listen_address"`
}

// InputSeries describes how to generate one time series.
type InputSeries struct {
	// Series is a Prometheus-style selector, e.g. 'test1{env="prod",instance="a"}'.
	Series string `yaml:"series"`

	// Values lists the sample values for consecutive slots (1-indexed).
	// A null entry means the slot is skipped or handled by Delays.
	Values []*float64 `yaml:"values"`

	// Delays is a list of [originalSlot, sendAtSlot, value] triples (1-indexed).
	// The sample is timestamped at T+(originalSlot-1)*interval
	// but sent to vmagent at T+(sendAtSlot-1)*interval.
	//
	// Example: [4, 6, 4] means a sample with value=4 whose logical timestamp
	// is slot 4 is actually delivered at slot 6 together with the slot-6 sample.
	Delays [][]float64 `yaml:"delays"`
}

// scheduledSample is a single data point waiting to be sent.
type scheduledSample struct {
	timestamp int64 // sample timestamp in milliseconds
	value     float64
}

// vmImportLine is one line of the VictoriaMetrics /api/v1/import NDJSON format.
type vmImportLine struct {
	Metric     map[string]string `json:"metric"`
	Values     []float64         `json:"values"`
	Timestamps []int64           `json:"timestamps"`
}

// --- report data types -------------------------------------------------------

// sentDataPoint is one data point in the sent-series chart.
type sentDataPoint struct {
	TsSec     float64 `json:"x"` // sample unix timestamp in seconds
	Value     float64 `json:"y"`
	SentAtSec float64 `json:"sentAt"` // wall-clock send time in seconds; equals TsSec if not delayed
	Delayed   bool    `json:"delayed"`
}

// sentSeriesData holds all sent data points for one configured input series.
type sentSeriesData struct {
	Name   string          `json:"name"`
	Points []sentDataPoint `json:"points"`
}

// recvDataPoint is one sample received on /api/v1/write.
type recvDataPoint struct {
	TsSec float64 `json:"x"` // sample unix timestamp in seconds
	Value float64 `json:"y"`
}

// recvSeriesData holds all received samples for one metric series.
type recvSeriesData struct {
	Name   string          `json:"name"`
	Points []recvDataPoint `json:"points"`
}

var (
	cfg        *AppConfig
	configPath string // path of the config file, stored at startup for hot-reload
	saYAML     []byte // SA config YAML to serve

	mu      sync.Mutex
	started bool

	reportMu     sync.RWMutex
	reportT      time.Time
	reportJitter time.Duration
	reportSent   []sentSeriesData
	reportRecv   = make(map[string]*recvSeriesData)
)

func main() {
	configFile := flag.String("config", "config.yaml", "path to config YAML file")
	flag.Parse()

	configPath = *configFile
	if err := loadConfig(); err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sa-config", handleSAConfig)
	mux.HandleFunc("/start", handleStart)
	mux.HandleFunc("/reset", handleReset)
	mux.HandleFunc("/api/v1/write", handleRemoteWrite)
	mux.HandleFunc("/report", handleReport)

	log.Printf("HTTP server listening on %s", cfg.ListenAddress)
	log.Printf("Endpoints:")
	log.Printf("  GET  /sa-config       — serve SA rules YAML for vmagent")
	log.Printf("  POST /start           — call vmagent /-/reload then write samples")
	log.Printf("  POST /reset           — reload config, clear 'started' flag")
	log.Printf("  POST /api/v1/write    — receive Prometheus remote-write from vmagent SA output")
	log.Printf("  GET  /report          — HTML report with sent/received series charts")
	log.Fatalf("server stopped: %v", http.ListenAndServe(cfg.ListenAddress, mux))
}

// handleSAConfig serves the SA rules YAML so that vmagent can fetch it.
func handleSAConfig(w http.ResponseWriter, r *http.Request) {
	log.Printf("[sa-config] request from %s", r.RemoteAddr)
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	if _, err := w.Write(saYAML); err != nil {
		log.Printf("[sa-config] write error: %v", err)
	}
}

// handleStart triggers vmagent reload and starts the sample writer goroutine.
func handleStart(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	if started {
		mu.Unlock()
		http.Error(w, "test already running; POST /reset to allow re-running", http.StatusConflict)
		return
	}
	started = true
	mu.Unlock()

	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid interval %q: %v", cfg.Interval, err), http.StatusBadRequest)
		mu.Lock()
		started = false
		mu.Unlock()
		return
	}

	// Call vmagent /-/reload so it picks up the SA config from /sa-config.
	reloadURL := cfg.VmagentAddress + "/-/reload"
	log.Printf("[start] calling vmagent reload: POST %s", reloadURL)
	reloadResp, err := http.Post(reloadURL, "application/json", nil) //nolint:noctx
	if err != nil {
		log.Printf("[start] reload request failed: %v", err)
		http.Error(w, fmt.Sprintf("vmagent reload failed: %v", err), http.StatusBadGateway)
		mu.Lock()
		started = false
		mu.Unlock()
		return
	}
	reloadBody, _ := io.ReadAll(reloadResp.Body)
	reloadResp.Body.Close()
	log.Printf("[start] reload response: status=%d body=%q", reloadResp.StatusCode, reloadBody)

	T := time.Now()
	jitter := time.Duration(rand.Int63n(int64(interval / 2))) //nolint:gosec
	log.Printf("[start] T=%s  jitter=%v  first-sample-at T+%v",
		T.Format(time.RFC3339Nano), jitter, jitter)

	reportMu.Lock()
	reportT = T
	reportJitter = jitter
	reportSent = nil
	reportRecv = make(map[string]*recvSeriesData)
	reportMu.Unlock()

	go runTest(T, interval, jitter)

	fmt.Fprintf(w, "test started\nT=%s\njitter=%v\n", T.Format(time.RFC3339Nano), jitter)
}

// handleRemoteWrite accepts Prometheus remote-write requests (e.g. from vmagent's SA output)
// and logs each received time series in a human-readable format for result verification.
func handleRemoteWrite(w http.ResponseWriter, r *http.Request) {
	isVMRemoteWrite := r.Header.Get("Content-Encoding") == "zstd"
	err := stream.Parse(r.Body, isVMRemoteWrite, func(tss []prompb.TimeSeries, _ []prompb.MetricMetadata) error {
		for i := range tss {
			ts := &tss[i]
			var sb strings.Builder
			// Build metric name + labels string.
			var metricName string
			for _, lbl := range ts.Labels {
				if lbl.Name == "__name__" {
					metricName = lbl.Value
					break
				}
			}
			sb.WriteString(metricName)
			sb.WriteByte('{')
			first := true
			for _, lbl := range ts.Labels {
				if lbl.Name == "__name__" {
					continue
				}
				if !first {
					sb.WriteByte(',')
				}
				first = false
				sb.WriteString(lbl.Name)
				sb.WriteString(`="`)
				sb.WriteString(lbl.Value)
				sb.WriteByte('"')
			}
			sb.WriteByte('}')
			metricStr := sb.String()

			// Log each sample on its own line for easy reading.
			for _, s := range ts.Samples {
				t := time.UnixMilli(s.Timestamp)
				log.Printf("[recv] %-60s  value=%-12g  ts= %v, ts_human=%s",
					metricStr, s.Value, t.UnixMilli(), t.UTC().Format(time.RFC3339Nano))
			}

			// Record for /report.
			reportMu.Lock()
			if reportRecv == nil {
				reportRecv = make(map[string]*recvSeriesData)
			}
			rd := reportRecv[metricStr]
			if rd == nil {
				rd = &recvSeriesData{Name: metricStr}
				reportRecv[metricStr] = rd
			}
			for _, s := range ts.Samples {
				rd.Points = append(rd.Points, recvDataPoint{
					TsSec: float64(s.Timestamp) / 1000.0,
					Value: s.Value,
				})
			}
			reportMu.Unlock()
		}
		return nil
	})
	if err != nil {
		log.Printf("[recv] parse error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleReset re-reads the config file from disk, updates the SA config and
// input_series, and clears the started flag so the test can be triggered again.
func handleReset(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	if err := loadConfig(); err != nil {
		mu.Unlock()
		log.Printf("[reset] failed to reload config: %v", err)
		http.Error(w, fmt.Sprintf("config reload failed: %v", err), http.StatusInternalServerError)
		return
	}
	started = false
	mu.Unlock()

	reportMu.Lock()
	reportSent = nil
	reportRecv = make(map[string]*recvSeriesData)
	reportT = time.Time{}
	reportMu.Unlock()

	log.Printf("[reset] config reloaded, started flag cleared")
	fmt.Fprintln(w, "reset ok")
}

// loadConfig reads configPath from disk, parses it into cfg, and rebuilds saYAML.
// Callers that need thread safety must hold mu.
func loadConfig() error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("cannot read config file %q: %w", configPath, err)
	}

	newCfg := &AppConfig{
		VmagentAddress: "http://localhost:8429",
		ListenAddress:  ":8080",
	}
	if err := yaml.Unmarshal(data, newCfg); err != nil {
		return fmt.Errorf("cannot parse config: %w", err)
	}

	var newSAYAML []byte
	if newCfg.SARules != nil {
		newSAYAML, err = yaml.Marshal(newCfg.SARules)
		if err != nil {
			return fmt.Errorf("cannot re-marshal saRules to YAML: %w", err)
		}
	} else {
		newSAYAML = []byte("[]\n")
	}

	cfg = newCfg
	saYAML = newSAYAML

	log.Printf("[config] loaded from %q", configPath)
	log.Printf("[config] interval          : %s", cfg.Interval)
	log.Printf("[config] vmagent           : %s", cfg.VmagentAddress)
	log.Printf("[config] listen            : %s", cfg.ListenAddress)
	log.Printf("[config] input_series count: %d", len(cfg.InputSeries))
	log.Printf("[config] SA config:\n---\n%s---", saYAML)
	return nil
}

// --- label parsing -----------------------------------------------------------

// seriesRe matches "metricname" or "metricname{k="v",...}".
var seriesRe = regexp.MustCompile(`^([^{,\s]+?)(?:\{([^}]*)\})?$`)
var labelRe = regexp.MustCompile(`(\w+)="([^"]*)"`)

// parseLabels converts a Prometheus-style selector string into a flat label map.
func parseLabels(series string) (map[string]string, error) {
	series = strings.TrimSpace(series)
	m := seriesRe.FindStringSubmatch(series)
	if m == nil {
		return nil, fmt.Errorf("cannot parse series selector %q", series)
	}
	labels := map[string]string{"__name__": m[1]}
	if m[2] != "" {
		for _, pair := range labelRe.FindAllStringSubmatch(m[2], -1) {
			labels[pair[1]] = pair[2]
		}
	}
	return labels, nil
}

// --- schedule building -------------------------------------------------------

// buildSchedule constructs a map of sendAtMs → []scheduledSample for a series.
//
// Slot numbering is 1-indexed:
//   - slot i has sample timestamp T+jitter+(i-1)*interval
//   - non-null values[i-1] are sent at their own slot time
//   - delay entry [orig, sendAt, val] sends a sample timestamped at slot orig
//     but delivered to vmagent at slot sendAt
func buildSchedule(is InputSeries, T time.Time, interval, jitter time.Duration) (map[int64][]scheduledSample, error) {
	type delayEntry struct {
		sendAtSlot int
		value      float64
	}
	delayMap := make(map[int]delayEntry, len(is.Delays))
	for _, d := range is.Delays {
		if len(d) != 3 {
			return nil, fmt.Errorf("each delay must be [originalSlot, sendAtSlot, value]; got %v", d)
		}
		delayMap[int(d[0])] = delayEntry{sendAtSlot: int(d[1]), value: d[2]}
	}

	schedule := make(map[int64][]scheduledSample)

	// Regular (non-null) values — sent at their natural slot time.
	for i, v := range is.Values {
		if v == nil {
			continue // null → slot is handled by delays or intentionally absent
		}
		sampleTime := T.Add(jitter + time.Duration(i)*interval)
		sendAtMs := sampleTime.UnixMilli()
		schedule[sendAtMs] = append(schedule[sendAtMs], scheduledSample{
			timestamp: sampleTime.UnixMilli(),
			value:     *v,
		})
	}

	// Delayed values — timestamped at originalSlot, sent at sendAtSlot.
	for origSlot, de := range delayMap {
		sampleTime := T.Add(jitter + time.Duration(origSlot-1)*interval)
		sendAt := T.Add(jitter + time.Duration(de.sendAtSlot-1)*interval)
		sendAtMs := sendAt.UnixMilli()
		schedule[sendAtMs] = append(schedule[sendAtMs], scheduledSample{
			timestamp: sampleTime.UnixMilli(),
			value:     de.value,
		})
	}

	// Sort samples within each slot by timestamp for deterministic ordering.
	for k, s := range schedule {
		sort.Slice(s, func(i, j int) bool { return s[i].timestamp < s[j].timestamp })
		schedule[k] = s
	}

	return schedule, nil
}

// --- test runner -------------------------------------------------------------

type seriesEvent struct {
	seriesName string // original series selector string from config
	labels     map[string]string
	samples    []scheduledSample
}

func runTest(T time.Time, interval, jitter time.Duration) {
	defer func() {
		mu.Lock()
		started = false
		mu.Unlock()
		log.Printf("[write] finished")
	}()

	// Collect all send events across every configured series.
	allEvents := make(map[int64][]seriesEvent)

	for _, is := range cfg.InputSeries {
		labels, err := parseLabels(is.Series)
		if err != nil {
			log.Printf("[test] cannot parse series %q: %v — skipping", is.Series, err)
			continue
		}
		schedule, err := buildSchedule(is, T, interval, jitter)
		if err != nil {
			log.Printf("[test] cannot build schedule for %q: %v — skipping", is.Series, err)
			continue
		}
		log.Printf("[test] series %q: %d distinct send-time slots", is.Series, len(schedule))
		for sendAtMs, samples := range schedule {
			allEvents[sendAtMs] = append(allEvents[sendAtMs], seriesEvent{
				seriesName: is.Series,
				labels:     labels,
				samples:    samples,
			})
		}
	}

	// Sort the unique send times into chronological order.
	sendTimes := make([]int64, 0, len(allEvents))
	for t := range allEvents {
		sendTimes = append(sendTimes, t)
	}
	sort.Slice(sendTimes, func(i, j int) bool { return sendTimes[i] < sendTimes[j] })

	log.Printf("[test] starting: %d distinct send times across %d series",
		len(sendTimes), len(cfg.InputSeries))

	for _, sendAtMs := range sendTimes {
		sendAt := time.UnixMilli(sendAtMs)
		if now := time.Now(); sendAt.After(now) {
			sleep := sendAt.Sub(now)
			time.Sleep(sleep)
		}

		for _, ev := range allEvents[sendAtMs] {
			recordSent(ev.seriesName, ev.samples, sendAtMs)
			if err := writeSamples(cfg.VmagentAddress, ev.labels, ev.samples); err != nil {
				log.Printf("[test] write error for %v: %v", ev.labels, err)
			}
		}
	}
}

// --- vmagent writer ----------------------------------------------------------

// writeSamples POSTs one NDJSON line to vmagent's /api/v1/import endpoint.
// All samples for the same metric series are batched into a single request.
func writeSamples(addr string, labels map[string]string, samples []scheduledSample) error {
	values := make([]float64, len(samples))
	timestamps := make([]int64, len(samples))
	for i, s := range samples {
		values[i] = s.value
		timestamps[i] = s.timestamp
	}

	line := vmImportLine{
		Metric:     labels,
		Values:     values,
		Timestamps: timestamps,
	}
	data, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')

	url := addr + "/api/v1/import"
	log.Printf("[write] metric=%v values=%v timestamps_ms=%v",
		labels, values, timestamps)

	resp, err := http.Post(url, "application/json", bytes.NewReader(data)) //nolint:noctx
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("POST %s: status=%d body=%q", url, resp.StatusCode, body)
	}
	return nil
}

// --- report ------------------------------------------------------------------

// recordSent stores sent samples into the report data store.
// It must be called without holding reportMu.
func recordSent(seriesName string, samples []scheduledSample, sendAtMs int64) {
	reportMu.Lock()
	defer reportMu.Unlock()

	var sd *sentSeriesData
	for i := range reportSent {
		if reportSent[i].Name == seriesName {
			sd = &reportSent[i]
			break
		}
	}
	if sd == nil {
		reportSent = append(reportSent, sentSeriesData{Name: seriesName})
		sd = &reportSent[len(reportSent)-1]
	}

	for _, s := range samples {
		sd.Points = append(sd.Points, sentDataPoint{
			TsSec:     float64(s.timestamp) / 1000.0,
			Value:     s.value,
			SentAtSec: float64(sendAtMs) / 1000.0,
			Delayed:   sendAtMs != s.timestamp,
		})
	}
}

// handleReport renders and serves the HTML report page with sent/received charts.
func handleReport(w http.ResponseWriter, r *http.Request) {
	// saYAML is read without mu, consistent with handleSAConfig.
	saYAMLStr := string(saYAML)

	reportMu.RLock()
	sentSnap := make([]sentSeriesData, len(reportSent))
	for i, sd := range reportSent {
		pts := make([]sentDataPoint, len(sd.Points))
		copy(pts, sd.Points)
		sentSnap[i] = sentSeriesData{Name: sd.Name, Points: pts}
	}
	recvSnap := make([]recvSeriesData, 0, len(reportRecv))
	for _, rd := range reportRecv {
		pts := make([]recvDataPoint, len(rd.Points))
		copy(pts, rd.Points)
		recvSnap = append(recvSnap, recvSeriesData{Name: rd.Name, Points: pts})
	}
	reportMu.RUnlock()

	sort.Slice(recvSnap, func(i, j int) bool { return recvSnap[i].Name < recvSnap[j].Name })

	sentJSON, _ := json.Marshal(sentSnap)
	recvJSON, _ := json.Marshal(recvSnap)

	// Single canvas for all sent series combined.
	var sentCanvas string
	if len(sentSnap) == 0 {
		sentCanvas = `<p class="no-data">No data yet — POST /start to run the test.</p>`
	} else {
		sentCanvas = `<div class="chart-wrap"><canvas id="sent-all"></canvas></div>`
	}

	var recvCharts strings.Builder
	if len(recvSnap) == 0 {
		recvCharts.WriteString(`<p class="no-data">No data received yet.</p>`)
	} else {
		for i, rd := range recvSnap {
			fmt.Fprintf(&recvCharts,
				"<h3>%s</h3><div class=\"chart-wrap\"><canvas id=\"recv-%d\"></canvas></div>\n",
				html.EscapeString(rd.Name), i)
		}
	}

	page := reportPageTemplate
	page = strings.ReplaceAll(page, "__GENERATED_AT__", time.Now().UTC().Format(time.RFC3339))
	page = strings.ReplaceAll(page, "__SA_YAML__", html.EscapeString(saYAMLStr))
	page = strings.ReplaceAll(page, "__SENT_COUNT__", strconv.Itoa(len(sentSnap)))
	page = strings.ReplaceAll(page, "__SENT_CANVAS__", sentCanvas)
	page = strings.ReplaceAll(page, "__RECV_COUNT__", strconv.Itoa(len(recvSnap)))
	page = strings.ReplaceAll(page, "__RECV_CHARTS__", recvCharts.String())
	page = strings.ReplaceAll(page, "__SENT_JSON__", string(sentJSON))
	page = strings.ReplaceAll(page, "__RECV_JSON__", string(recvJSON))
	startTs := "0"
	if !reportT.IsZero() {
		startTs = fmt.Sprintf("%f", float64(reportT.UnixMilli())/1000.0)
	}
	page = strings.ReplaceAll(page, "__START_TS__", startTs)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, page)
}
