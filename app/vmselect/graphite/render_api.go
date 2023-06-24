package graphite

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bufferedwriter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	storageStep = flag.Duration("search.graphiteStorageStep", 10*time.Second, "The interval between datapoints stored in the database. "+
		"It is used at Graphite Render API handler for normalizing the interval between datapoints in case it isn't normalized. "+
		"It can be overridden by sending 'storage_step' query arg to /render API or "+
		"by sending the desired interval via 'Storage-Step' http header during querying /render API")
	maxPointsPerSeries = flag.Int("search.graphiteMaxPointsPerSeries", 1e6, "The maximum number of points per series Graphite render API can return")
)

// RenderHandler implements /render endpoint from Graphite Render API.
//
// See https://graphite.readthedocs.io/en/stable/render_api.html
func RenderHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	format := r.FormValue("format")
	if format != "json" {
		return fmt.Errorf("unsupported format=%q; supported values: json", format)
	}
	xFilesFactor := float64(0)
	if xff := r.FormValue("xFilesFactor"); len(xff) > 0 {
		f, err := strconv.ParseFloat(xff, 64)
		if err != nil {
			return fmt.Errorf("cannot parse xFilesFactor=%q: %w", xff, err)
		}
		xFilesFactor = f
	}
	from := r.FormValue("from")
	fromTime := startTime.UnixNano()/1e6 - 24*3600*1000
	if len(from) != 0 {
		fv, err := parseTime(startTime, from)
		if err != nil {
			return fmt.Errorf("cannot parse from=%q: %w", from, err)
		}
		fromTime = fv
	}
	until := r.FormValue("until")
	untilTime := startTime.UnixNano() / 1e6
	if len(until) != 0 {
		uv, err := parseTime(startTime, until)
		if err != nil {
			return fmt.Errorf("cannot parse until=%q: %w", until, err)
		}
		untilTime = uv
	}
	storageStep, err := getStorageStep(r)
	if err != nil {
		return err
	}
	fromAlign := fromTime % storageStep
	fromTime -= fromAlign
	if fromAlign > 0 {
		fromTime += storageStep
	}
	untilAlign := untilTime % storageStep
	untilTime -= untilAlign
	if untilAlign > 0 {
		untilTime += storageStep
	}
	if untilTime < fromTime {
		return fmt.Errorf("from=%s cannot exceed until=%s", from, until)
	}
	pointsPerSeries := (untilTime - fromTime) / storageStep
	if pointsPerSeries > int64(*maxPointsPerSeries) {
		return fmt.Errorf("too many points per series must be returned on the given [from=%s ... until=%s] time range and the given storageStep=%d: %d; "+
			"either reduce the time range or increase -search.graphiteMaxPointsPerSeries=%d", from, until, storageStep, pointsPerSeries, *maxPointsPerSeries)
	}
	maxDataPoints := 0
	if s := r.FormValue("maxDataPoints"); len(s) > 0 {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("cannot parse maxDataPoints=%q: %w", maxDataPoints, err)
		}
		if n <= 0 {
			return fmt.Errorf("maxDataPoints must be greater than 0; got %f", n)
		}
		maxDataPoints = int(n)
	}
	etfs, err := searchutils.GetExtraTagFilters(r)
	if err != nil {
		return fmt.Errorf("cannot setup tag filters: %w", err)
	}
	var nextSeriess []nextSeriesFunc
	targets := r.Form["target"]
	for _, target := range targets {
		ec := &evalConfig{
			startTime:     fromTime,
			endTime:       untilTime,
			storageStep:   storageStep,
			deadline:      deadline,
			currentTime:   startTime,
			xFilesFactor:  xFilesFactor,
			etfs:          etfs,
			originalQuery: target,
		}
		nextSeries, err := execExpr(ec, target)
		if err != nil {
			for _, f := range nextSeriess {
				_, _ = drainAllSeries(f)
			}
			return fmt.Errorf("cannot eval target=%q: %w", target, err)
		}
		// do not use nextSeriesConcurrentWrapper here in order to preserve series order.
		if maxDataPoints > 0 {
			step := (ec.endTime - ec.startTime) / int64(maxDataPoints)
			nextSeries = nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
				aggrFunc := s.consolidateFunc
				if aggrFunc == nil {
					aggrFunc = aggrAvg
				}
				xFilesFactor := s.xFilesFactor
				if s.xFilesFactor <= 0 {
					xFilesFactor = ec.xFilesFactor
				}
				if len(s.Values) > maxDataPoints {
					s.summarize(aggrFunc, ec.startTime, ec.endTime, step, xFilesFactor)
				}
				return s, nil
			})
		}
		nextSeriess = append(nextSeriess, nextSeries)
	}
	f := nextSeriesGroup(nextSeriess, nil)
	jsonp := r.FormValue("jsonp")
	contentType := getContentType(jsonp)
	w.Header().Set("Content-Type", contentType)
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteRenderJSONResponse(bw, f, jsonp)
	if err := bw.Flush(); err != nil {
		return err
	}
	renderDuration.UpdateDuration(startTime)
	return nil
}

var renderDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/render"}`)

const msecsPerDay = 24 * 3600 * 1000

// parseTime parses Graphite time in s.
//
// If the time in s is relative, then it is relative to startTime.
func parseTime(startTime time.Time, s string) (int64, error) {
	switch s {
	case "now":
		return startTime.UnixNano() / 1e6, nil
	case "today":
		ts := startTime.UnixNano() / 1e6
		return ts - ts%msecsPerDay, nil
	case "yesterday":
		ts := startTime.UnixNano() / 1e6
		return ts - (ts % msecsPerDay) - msecsPerDay, nil
	}
	// Attempt to parse RFC3339 (YYYY-MM-DDTHH:mm:SSZTZ:00)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UnixNano() / 1e6, nil
	}
	// Attempt to parse HH:MM_YYYYMMDD
	if t, err := time.Parse("15:04_20060102", s); err == nil {
		return t.UnixNano() / 1e6, nil
	}
	// Attempt to parse HH:MMYYYYMMDD
	if t, err := time.Parse("15:0420060102", s); err == nil {
		return t.UnixNano() / 1e6, nil
	}
	// Attempt to parse YYYYMMDD
	if t, err := time.Parse("20060102", s); err == nil {
		return t.UnixNano() / 1e6, nil
	}
	// Attempt to parse HH:MM YYYYMMDD
	if t, err := time.Parse("15:04 20060102", s); err == nil {
		return t.UnixNano() / 1e6, nil
	}
	// Attempt to parse YYYY-MM-DD
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UnixNano() / 1e6, nil
	}
	// Attempt to parse MM/DD/YY
	if t, err := time.Parse("01/02/06", s); err == nil {
		return t.UnixNano() / 1e6, nil
	}

	// Attempt to parse time as unix timestamp
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n * 1000, nil
	}
	// Attempt to parse interval
	if interval, err := parseInterval(s); err == nil {
		return startTime.UnixNano()/1e6 + interval, nil
	}
	return 0, fmt.Errorf("unsupported time %q", s)
}

func parseInterval(s string) (int64, error) {
	s = strings.TrimSpace(s)
	prefix := s
	var suffix string
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch != '-' && ch != '+' && ch != '.' && (ch < '0' || ch > '9') {
			prefix = s[:i]
			suffix = s[i:]
			break
		}
	}
	n, err := strconv.ParseFloat(prefix, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse interval %q: %w", s, err)
	}
	suffix = strings.TrimSpace(suffix)
	if len(suffix) == 0 {
		return 0, fmt.Errorf("missing suffix for interval %q; expecting s, min, h, d, w, mon or y suffix", s)
	}
	var m float64
	switch {
	case strings.HasPrefix(suffix, "ms"):
		m = 1
	case strings.HasPrefix(suffix, "s"):
		m = 1000
	case strings.HasPrefix(suffix, "mi"),
		strings.HasPrefix(suffix, "m") && !strings.HasPrefix(suffix, "mo"):
		m = 60 * 1000
	case strings.HasPrefix(suffix, "h"):
		m = 3600 * 1000
	case strings.HasPrefix(suffix, "d"):
		m = 24 * 3600 * 1000
	case strings.HasPrefix(suffix, "w"):
		m = 7 * 24 * 3600 * 1000
	case strings.HasPrefix(suffix, "mo"):
		m = 30 * 24 * 3600 * 1000
	case strings.HasPrefix(suffix, "y"):
		m = 365 * 24 * 3600 * 1000
	default:
		return 0, fmt.Errorf("unsupported interval %q", s)
	}
	return int64(n * m), nil
}

func getStorageStep(r *http.Request) (int64, error) {
	s := r.FormValue("storage_step")
	if len(s) == 0 {
		s = r.Header.Get("Storage-Step")
	}
	if len(s) == 0 {
		step := int64(storageStep.Seconds() * 1000)
		if step <= 0 {
			return 0, fmt.Errorf("the `-search.graphiteStorageStep` command-line flag value must be positive; got %s", storageStep.String())
		}
		return step, nil
	}
	step, err := parseInterval(s)
	if err != nil {
		return 0, fmt.Errorf("cannot parse datapoints interval %s: %w", s, err)
	}
	if step <= 0 {
		return 0, fmt.Errorf("storage_step cannot be negative; got %s", s)
	}
	return step, nil
}
