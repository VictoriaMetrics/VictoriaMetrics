package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

var (
	addr    = flag.String("addr", "http://localhost:9428/insert/jsonline", "HTTP address to push the generated logs to")
	workers = flag.Int("workers", 1, "The number of workers to use to push logs to -addr")

	start         = newTimeFlag("start", "-1d", "Generated logs start from this time; see https://docs.victoriametrics.com/#timestamp-formats")
	end           = newTimeFlag("end", "0s", "Generated logs end at this time; see https://docs.victoriametrics.com/#timestamp-formats")
	activeStreams = flag.Int("activeStreams", 1_000, "The number of active log streams to generate; see https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields")
	totalStreams  = flag.Int("totalStreams", 2_000, "The number of total log streams; if -totalStreams > -activeStreams, then some active streams are substituted with new streams "+
		"during data generation")
	logsPerStream   = flag.Int64("logsPerStream", 1_000, "The number of log entries to generate per each log stream. Log entries are evenly distributed between -start and -end")
	varFieldsPerLog = flag.Int("varFieldsPerLog", 3, "The number of additional fields with variable values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model")
	constFieldsPerLog = flag.Int("constFieldsPerLog", 3, "The number of additional fields with constaint values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#data-model")

	statInterval = flag.Duration("statInterval", 10*time.Second, "The interval between publishing the stats")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	remoteWriteURL, err := url.Parse(*addr)
	if err != nil {
		logger.Fatalf("cannot parse -addr=%q: %s", *addr, err)
	}
	qs, err := url.ParseQuery(remoteWriteURL.RawQuery)
	if err != nil {
		logger.Fatalf("cannot parse query string in -addr=%q: %w", *addr, err)
	}
	qs.Set("_stream_fields", "host,worker_id")
	remoteWriteURL.RawQuery = qs.Encode()

	if start.nsec >= end.nsec {
		logger.Fatalf("-start=%s must be smaller than -end=%s", start, end)
	}
	if *activeStreams <= 0 {
		logger.Fatalf("-activeStreams must be bigger than 0; got %d", *activeStreams)
	}
	if *logsPerStream <= 0 {
		logger.Fatalf("-logsPerStream must be bigger than 0; got %d", *logsPerStream)
	}
	if *varFieldsPerLog <= 0 {
		logger.Fatalf("-varFieldsPerLog must be bigger than 0; got %d", *varFieldsPerLog)
	}
	if *constFieldsPerLog <= 0 {
		logger.Fatalf("-constFieldsPerLog must be bigger than 0; got %d", *constFieldsPerLog)
	}
	if *totalStreams < *activeStreams {
		*totalStreams = *activeStreams
	}

	cfg := &workerConfig{
		url:           remoteWriteURL,
		activeStreams: *activeStreams,
		totalStreams:  *totalStreams,
	}

	// divide total and active streams among workers
	if *workers <= 0 {
		logger.Fatalf("-workers must be bigger than 0; got %d", *workers)
	}
	if *workers > *activeStreams {
		logger.Fatalf("-workers=%d cannot exceed -activeStreams=%d", *workers, *activeStreams)
	}
	cfg.activeStreams /= *workers
	cfg.totalStreams /= *workers

	logger.Infof("start -workers=%d workers for ingesting -logsPerStream=%d log entries per each -totalStreams=%d (-activeStreams=%d) on a time range -start=%s, -end=%s to -addr=%s",
		*workers, *logsPerStream, *totalStreams, *activeStreams, toRFC3339(start.nsec), toRFC3339(end.nsec), *addr)

	startTime := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			generateAndPushLogs(cfg, workerID)
		}(i)
	}

	go func() {
		prevEntries := uint64(0)
		prevBytes := uint64(0)
		ticker := time.NewTicker(*statInterval)
		for range ticker.C {
			currEntries := logEntriesCount.Load()
			deltaEntries := currEntries - prevEntries
			rateEntries := float64(deltaEntries) / statInterval.Seconds()

			currBytes := bytesGenerated.Load()
			deltaBytes := currBytes - prevBytes
			rateBytes := float64(deltaBytes) / statInterval.Seconds()
			logger.Infof("generated %dK log entries (%dK total) at %.0fK entries/sec, %dMB (%dMB total) at %.0fMB/sec",
				deltaEntries/1e3, currEntries/1e3, rateEntries/1e3, deltaBytes/1e6, currBytes/1e6, rateBytes/1e6)

			prevEntries = currEntries
			prevBytes = currBytes
		}
	}()

	wg.Wait()

	dSecs := time.Since(startTime).Seconds()
	currEntries := logEntriesCount.Load()
	currBytes := bytesGenerated.Load()
	rateEntries := float64(currEntries) / dSecs
	rateBytes := float64(currBytes) / dSecs
	logger.Infof("ingested %dK log entries (%dMB) in %.3f seconds; avg ingestion rate: %.0fK entries/sec, %.0fMB/sec", currEntries/1e3, currBytes/1e6, dSecs, rateEntries/1e3, rateBytes/1e6)
}

var logEntriesCount atomic.Uint64

var bytesGenerated atomic.Uint64

type workerConfig struct {
	url           *url.URL
	activeStreams int
	totalStreams  int
}

type statWriter struct {
	w io.Writer
}

func (sw *statWriter) Write(p []byte) (int, error) {
	bytesGenerated.Add(uint64(len(p)))
	return sw.w.Write(p)
}

func generateAndPushLogs(cfg *workerConfig, workerID int) {
	pr, pw := io.Pipe()
	sw := &statWriter{
		w: pw,
	}
	bw := bufio.NewWriter(sw)
	doneCh := make(chan struct{})
	go func() {
		generateLogs(bw, workerID, cfg.activeStreams, cfg.totalStreams)
		_ = bw.Flush()
		_ = pw.Close()
		close(doneCh)
	}()

	req, err := http.NewRequest("POST", cfg.url.String(), pr)
	if err != nil {
		logger.Fatalf("cannot create request to %q: %s", cfg.url, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Fatalf("cannot perform request to %q: %s", cfg.url, err)
	}
	if resp.StatusCode/100 != 2 {
		logger.Fatalf("unexpected status code got from %q: %d; want 2xx", cfg.url, err)
	}

	// Wait until all the generateLogs goroutine is finished.
	<-doneCh
}

func generateLogs(bw *bufio.Writer, workerID, activeStreams, totalStreams int) {
	streamLifetime := int64(float64(end.nsec-start.nsec) * (float64(activeStreams) / float64(totalStreams)))
	streamStep := int64(float64(end.nsec-start.nsec) / float64(totalStreams-activeStreams+1))
	step := streamLifetime / (*logsPerStream - 1)

	currNsec := start.nsec
	for currNsec < end.nsec {
		firstStreamID := int((currNsec - start.nsec) / streamStep)
		generateLogsAtTimestamp(bw, workerID, currNsec, firstStreamID, activeStreams, *varFieldsPerLog, *constFieldsPerLog)
		currNsec += step
	}
}

func generateLogsAtTimestamp(bw *bufio.Writer, workerID int, ts int64, firstStreamID, activeStreams, varFieldsPerEntry, constFieldsPerEntry int) {
	streamID := firstStreamID
	timeStr := toRFC3339(ts)
	for i := 0; i < activeStreams; i++ {
		fmt.Fprintf(bw, `{"_time":%q,"_msg":"message #%d (%d) for the stream %d and worker %d; some foo bar baz error warn 1.2.3.4","host":"host_%d","worker_id":"%d"`,
			timeStr, ts, i, streamID, workerID, streamID, workerID)
		for j := 0; j < varFieldsPerEntry; j++ {
			fmt.Fprintf(bw, `,"var_field_%d":"value_%d_%d_%d"`, j, i, j, streamID)
		}
		for j := 0; j < constFieldsPerEntry; j++ {
			fmt.Fprintf(bw, `,"const_field_%d":"value_%d_%d"`, j, j, streamID)
		}
		fmt.Fprintf(bw, "}\n")

		logEntriesCount.Add(1)
	}
}

func newTimeFlag(name, defaultValue, description string) *timeFlag {
	var tf timeFlag
	if err := tf.Set(defaultValue); err != nil {
		logger.Panicf("invalid defaultValue=%q for flag %q: %w", defaultValue, name, err)
	}
	flag.Var(&tf, name, description)
	return &tf
}

type timeFlag struct {
	s    string
	nsec int64
}

func (tf *timeFlag) Set(s string) error {
	msec, err := promutils.ParseTimeMsec(s)
	if err != nil {
		return fmt.Errorf("cannot parse time from %q: %w", s, err)
	}
	tf.s = s
	tf.nsec = msec * 1e6
	return nil
}

func (tf *timeFlag) String() string {
	return tf.s
}

func toRFC3339(nsec int64) string {
	return time.Unix(nsec/1e9, nsec%1e9).UTC().Format(time.RFC3339Nano)
}
