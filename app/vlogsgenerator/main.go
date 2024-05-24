package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

var (
	addr    = flag.String("addr", "stdout", "HTTP address to push the generated logs to; if it is set to stdout, then logs are generated to stdout")
	workers = flag.Int("workers", 1, "The number of workers to use to push logs to -addr")

	start         = newTimeFlag("start", "-1d", "Generated logs start from this time; see https://docs.victoriametrics.com/#timestamp-formats")
	end           = newTimeFlag("end", "0s", "Generated logs end at this time; see https://docs.victoriametrics.com/#timestamp-formats")
	activeStreams = flag.Int("activeStreams", 100, "The number of active log streams to generate; see https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields")
	totalStreams  = flag.Int("totalStreams", 0, "The number of total log streams; if -totalStreams > -activeStreams, then some active streams are substituted with new streams "+
		"during data generation")
	logsPerStream     = flag.Int64("logsPerStream", 1_000, "The number of log entries to generate per each log stream. Log entries are evenly distributed between -start and -end")
	constFieldsPerLog = flag.Int("constFieldsPerLog", 3, "The number of fields with constaint values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")
	varFieldsPerLog = flag.Int("varFieldsPerLog", 1, "The number of fields with variable values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")
	dictFieldsPerLog = flag.Int("dictFieldsPerLog", 2, "The number of fields with up to 8 different values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")
	u8FieldsPerLog = flag.Int("u8FieldsPerLog", 1, "The number of fields with uint8 values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")
	u16FieldsPerLog = flag.Int("u16FieldsPerLog", 1, "The number of fields with uint16 values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")
	u32FieldsPerLog = flag.Int("u32FieldsPerLog", 1, "The number of fields with uint32 values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")
	u64FieldsPerLog = flag.Int("u64FieldsPerLog", 1, "The number of fields with uint64 values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")
	floatFieldsPerLog = flag.Int("floatFieldsPerLog", 1, "The number of fields with float64 values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")
	ipFieldsPerLog = flag.Int("ipFieldsPerLog", 1, "The number of fields with IPv4 values to generate per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")
	timestampFieldsPerLog = flag.Int("timestampFieldsPerLog", 1, "The number of fields with ISO8601 timestamps per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")
	jsonFieldsPerLog = flag.Int("jsonFieldsPerLog", 1, "The number of JSON fields to generate per each log entry; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model")

	statInterval = flag.Duration("statInterval", 10*time.Second, "The interval between publishing the stats")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	var remoteWriteURL *url.URL
	if *addr != "stdout" {
		urlParsed, err := url.Parse(*addr)
		if err != nil {
			logger.Fatalf("cannot parse -addr=%q: %s", *addr, err)
		}
		qs, err := url.ParseQuery(urlParsed.RawQuery)
		if err != nil {
			logger.Fatalf("cannot parse query string in -addr=%q: %w", *addr, err)
		}
		qs.Set("_stream_fields", "host,worker_id")
		urlParsed.RawQuery = qs.Encode()
		remoteWriteURL = urlParsed
	}

	if start.nsec >= end.nsec {
		logger.Fatalf("-start=%s must be smaller than -end=%s", start, end)
	}
	if *activeStreams <= 0 {
		logger.Fatalf("-activeStreams must be bigger than 0; got %d", *activeStreams)
	}
	if *logsPerStream <= 0 {
		logger.Fatalf("-logsPerStream must be bigger than 0; got %d", *logsPerStream)
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

	if cfg.url == nil {
		_, err := io.Copy(os.Stdout, pr)
		if err != nil {
			logger.Fatalf("unexpected error when writing logs to stdout: %s", err)
		}
		return
	}

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
		generateLogsAtTimestamp(bw, workerID, currNsec, firstStreamID, activeStreams)
		currNsec += step
	}
}

var runID = toUUID(rand.Uint64(), rand.Uint64())

func generateLogsAtTimestamp(bw *bufio.Writer, workerID int, ts int64, firstStreamID, activeStreams int) {
	streamID := firstStreamID
	timeStr := toRFC3339(ts)
	for i := 0; i < activeStreams; i++ {
		ip := toIPv4(rand.Uint32())
		uuid := toUUID(rand.Uint64(), rand.Uint64())
		fmt.Fprintf(bw, `{"_time":%q,"_msg":"message for the stream %d and worker %d; ip=%s; uuid=%s; u64=%d","host":"host_%d","worker_id":"%d"`,
			timeStr, streamID, workerID, ip, uuid, rand.Uint64(), streamID, workerID)
		fmt.Fprintf(bw, `,"run_id":"%s"`, runID)
		for j := 0; j < *constFieldsPerLog; j++ {
			fmt.Fprintf(bw, `,"const_%d":"some value %d %d"`, j, j, streamID)
		}
		for j := 0; j < *varFieldsPerLog; j++ {
			fmt.Fprintf(bw, `,"var_%d":"some value %d %d"`, j, j, rand.Uint64())
		}
		for j := 0; j < *dictFieldsPerLog; j++ {
			fmt.Fprintf(bw, `,"dict_%d":"%s"`, j, dictValues[rand.Intn(len(dictValues))])
		}
		for j := 0; j < *u8FieldsPerLog; j++ {
			fmt.Fprintf(bw, `,"u8_%d":"%d"`, j, uint8(rand.Uint32()))
		}
		for j := 0; j < *u16FieldsPerLog; j++ {
			fmt.Fprintf(bw, `,"u16_%d":"%d"`, j, uint16(rand.Uint32()))
		}
		for j := 0; j < *u32FieldsPerLog; j++ {
			fmt.Fprintf(bw, `,"u32_%d":"%d"`, j, rand.Uint32())
		}
		for j := 0; j < *u64FieldsPerLog; j++ {
			fmt.Fprintf(bw, `,"u64_%d":"%d"`, j, rand.Uint64())
		}
		for j := 0; j < *floatFieldsPerLog; j++ {
			fmt.Fprintf(bw, `,"float_%d":"%v"`, j, math.Round(10_000*rand.Float64())/1000)
		}
		for j := 0; j < *ipFieldsPerLog; j++ {
			ip := toIPv4(rand.Uint32())
			fmt.Fprintf(bw, `,"ip_%d":"%s"`, j, ip)
		}
		for j := 0; j < *timestampFieldsPerLog; j++ {
			timestamp := toISO8601(int64(rand.Uint64()))
			fmt.Fprintf(bw, `,"timestamp_%d":"%s"`, j, timestamp)
		}
		for j := 0; j < *jsonFieldsPerLog; j++ {
			fmt.Fprintf(bw, `,"json_%d":"{\"foo\":\"bar_%d\",\"baz\":{\"a\":[\"x\",\"y\"]},\"f3\":NaN,\"f4\":%d}"`, j, rand.Intn(10), rand.Intn(100))
		}
		fmt.Fprintf(bw, "}\n")

		logEntriesCount.Add(1)
		streamID++
	}
}

var dictValues = []string{
	"debug",
	"info",
	"warn",
	"error",
	"fatal",
	"ERROR",
	"FATAL",
	"INFO",
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
	return time.Unix(0, nsec).UTC().Format(time.RFC3339Nano)
}

func toISO8601(nsec int64) string {
	return time.Unix(0, nsec).UTC().Format("2006-01-02T15:04:05.000Z")
}

func toIPv4(n uint32) string {
	dst := make([]byte, 0, len("255.255.255.255"))
	dst = marshalUint64(dst, uint64(n>>24))
	dst = append(dst, '.')
	dst = marshalUint64(dst, uint64((n>>16)&0xff))
	dst = append(dst, '.')
	dst = marshalUint64(dst, uint64((n>>8)&0xff))
	dst = append(dst, '.')
	dst = marshalUint64(dst, uint64(n&0xff))
	return string(dst)
}

func toUUID(a, b uint64) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", a&(1<<32-1), (a>>32)&(1<<16-1), (a >> 48), b&(1<<16-1), b>>16)
}

// marshalUint64 appends string representation of n to dst and returns the result.
func marshalUint64(dst []byte, n uint64) []byte {
	return strconv.AppendUint(dst, n, 10)
}
