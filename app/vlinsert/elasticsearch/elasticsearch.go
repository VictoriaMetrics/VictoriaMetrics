package elasticsearch

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

var (
	elasticsearchVersion = flag.String("elasticsearch.version", "8.9.0", "Elasticsearch version to report to client")
)

// RequestHandler processes Elasticsearch insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	w.Header().Add("Content-Type", "application/json")
	// This header is needed for Logstash
	w.Header().Set("X-Elastic-Product", "Elasticsearch")

	if strings.HasPrefix(path, "/_ilm/policy") {
		// Return fake response for Elasticsearch ilm request.
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/_index_template") {
		// Return fake response for Elasticsearch index template request.
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/_ingest") {
		// Return fake response for Elasticsearch ingest pipeline request.
		// See: https://www.elastic.co/guide/en/elasticsearch/reference/8.8/put-pipeline-api.html
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/_nodes") {
		// Return fake response for Elasticsearch nodes discovery request.
		// See: https://www.elastic.co/guide/en/elasticsearch/reference/8.8/cluster.html
		fmt.Fprintf(w, `{}`)
		return true
	}
	if strings.HasPrefix(path, "/logstash") || strings.HasPrefix(path, "/_logstash") {
		// Return fake response for Logstash APIs requests.
		// See: https://www.elastic.co/guide/en/elasticsearch/reference/8.8/logstash-apis.html
		fmt.Fprintf(w, `{}`)
		return true
	}
	switch path {
	case "/", "":
		switch r.Method {
		case http.MethodGet:
			// Return fake response for Elasticsearch ping request.
			// See the latest available version for Elasticsearch at https://github.com/elastic/elasticsearch/releases
			fmt.Fprintf(w, `{
			"version": {
				"number": %q
			}
		}`, *elasticsearchVersion)
		case http.MethodHead:
			// Return empty response for Logstash ping request.
		}

		return true
	case "/_license":
		// Return fake response for Elasticsearch license request.
		fmt.Fprintf(w, `{
			"license": {
				"uid": "cbff45e7-c553-41f7-ae4f-9205eabd80xx",
				"type": "oss",
				"status": "active",
				"expiry_date_in_millis" : 4000000000000
			}
		}`)
		return true
	case "/_bulk":
		startTime := time.Now()
		bulkRequestsTotal.Inc()

		cp, err := insertutil.GetCommonParams(r)
		if err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		if err := vlstorage.CanWriteData(); err != nil {
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		lmp := cp.NewLogMessageProcessor("elasticsearch_bulk", true)
		encoding := r.Header.Get("Content-Encoding")
		streamName := fmt.Sprintf("remoteAddr=%s, requestURI=%q", httpserver.GetQuotedRemoteAddr(r), r.RequestURI)
		n, parseErrors, err := readBulkRequest(streamName, r.Body, encoding, cp.TimeFields, cp.MsgFields, lmp)
		lmp.MustClose()
		if err != nil {
			logger.Errorf("cannot read /_bulk request: %s, stream fields: %s", err, cp.StreamFields)
			httpserver.Errorf(w, r, "cannot read /_bulk request: %s", err)
			return true
		}
		logParseErrors(parseErrors, cp.StreamFields)

		tookMs := time.Since(startTime).Milliseconds()
		bw := bufferedwriter.Get(w)
		defer bufferedwriter.Put(bw)
		// Even if there were parsing errors and not a single document could be parsed,
		// we must still return a 200 OK status to match Elasticsearch's behavior.
		WriteBulkResponse(bw, n, parseErrors, tookMs)
		_ = bw.Flush()

		// update bulkRequestDuration only for successfully parsed requests
		// There is no need in updating bulkRequestDuration for request errors,
		// since their timings are usually much smaller than the timing for successful request parsing.
		bulkRequestDuration.UpdateDuration(startTime)

		return true
	default:
		return false
	}
}

func logParseErrors(errs []parseError, streamFields []string) {
	if len(errs) == 0 {
		return
	}
	errorsToLog := 5
	if errorsToLog > len(errs) {
		errorsToLog = len(errs)
	}
	for _, parseErr := range errs[:errorsToLog] {
		logger.Warnf("cannot decode log message #%d in /_bulk request: %s, stream fields: %s", parseErr.pos, parseErr.err, streamFields)
	}
	skipped := len(errs[errorsToLog:])
	if skipped > 0 {
		logger.Warnf("skipped %d more parse errors in /_bulk request; stream fields: %s", skipped, streamFields)
	}
}

var (
	bulkRequestsTotal   = metrics.NewCounter(`vl_http_requests_total{path="/insert/elasticsearch/_bulk"}`)
	bulkRequestDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/elasticsearch/_bulk"}`)
)

type parseError struct {
	pos int
	err error
}

func readBulkRequest(streamName string, r io.Reader, encoding string, timeFields, msgFields []string, lmp insertutil.LogMessageProcessor) (int, []parseError, error) {
	// See https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-bulk.html

	reader, err := protoparserutil.GetUncompressedReader(r, encoding)
	if err != nil {
		return 0, nil, fmt.Errorf("cannot decode Elasticsearch protocol data: %w", err)
	}
	defer protoparserutil.PutUncompressedReader(reader)

	wcr := writeconcurrencylimiter.GetReader(reader)
	defer writeconcurrencylimiter.PutReader(wcr)

	lr := insertutil.NewLineReader(streamName, wcr)

	n := 0
	var parseErrors []parseError
	for ; ; n++ {
		hasMore, err := readBulkLine(lr, timeFields, msgFields, lmp)
		wcr.DecConcurrency()
		if err != nil {
			if errors.Is(err, errParseLogEntry) {
				parseErrors = append(parseErrors, parseError{pos: n, err: err})
				continue
			}
			return n, nil, err
		}
		if !hasMore {
			return n, parseErrors, nil
		}
	}
}

var errParseLogEntry = errors.New("cannot parse json-encoded log entry")

func readBulkLine(lr *insertutil.LineReader, timeFields, msgFields []string, lmp insertutil.LogMessageProcessor) (bool, error) {
	var line []byte

	// Read the command, must be "create" or "index"
	for len(line) == 0 {
		if !lr.NextLine() {
			err := lr.Err()
			return false, err
		}
		line = lr.Line
	}
	lineStr := bytesutil.ToUnsafeString(line)
	if !strings.Contains(lineStr, `"create"`) && !strings.Contains(lineStr, `"index"`) {
		return false, fmt.Errorf(`unexpected command %q; expecting "create" or "index"`, line)
	}

	// Decode log message
	if !lr.NextLine() {
		if err := lr.Err(); err != nil {
			return false, err
		}
		return false, fmt.Errorf(`missing log message after the "create" or "index" command`)
	}
	line = lr.Line
	if len(line) == 0 {
		// Special case - the line could be too long, so it was skipped.
		// Continue parsing next lines.
		return true, nil
	}
	p := logstorage.GetJSONParser()
	if err := p.ParseLogMessage(line); err != nil {
		return true, fmt.Errorf("%w: %s", errParseLogEntry, err)
	}

	ts, err := extractTimestampFromFields(timeFields, p.Fields)
	if err != nil {
		return true, fmt.Errorf("%w: %s", errParseLogEntry, err)
	}
	if ts == 0 {
		ts = time.Now().UnixNano()
	}
	logstorage.RenameField(p.Fields, msgFields, "_msg")
	lmp.AddRow(ts, p.Fields, nil)
	logstorage.PutJSONParser(p)

	return true, nil
}

func extractTimestampFromFields(timeFields []string, fields []logstorage.Field) (int64, error) {
	for _, timeField := range timeFields {
		for i := range fields {
			f := &fields[i]
			if f.Name != timeField {
				continue
			}
			timestamp, err := parseElasticsearchTimestamp(f.Value)
			if err != nil {
				return 0, err
			}
			f.Value = ""
			return timestamp, nil
		}
	}
	return 0, nil
}

func parseElasticsearchTimestamp(s string) (int64, error) {
	if s == "0" || s == "" {
		// Special case - zero or empty timestamp must be substituted
		// with the current time by the caller.
		return 0, nil
	}
	if len(s) < len("YYYY-MM-DD") || s[len("YYYY")] != '-' {
		// Try parsing timestamp in seconds or milliseconds
		return insertutil.ParseUnixTimestamp(s)
	}
	if len(s) == len("YYYY-MM-DD") {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return 0, fmt.Errorf("cannot parse date %q: %w", s, err)
		}
		return t.UnixNano(), nil
	}
	nsecs, ok := logstorage.TryParseTimestampRFC3339Nano(s)
	if !ok {
		return 0, fmt.Errorf("cannot parse timestamp %q", s)
	}
	return nsecs, nil
}
