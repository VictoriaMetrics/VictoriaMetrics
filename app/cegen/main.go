// cegen generates a stream of Prometheus remote write metrics with a configurable cardinality
// and sends them to a remote write endpoint.
//
// Usage:
//
//	cegen -url http://localhost:8490/api/v1/write -cardinality 1000 -template 'foo{bar="bar$i",baz="baz$i"}'
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/metricsql"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

var (
	targetURL = flag.String("url", "http://localhost:8490/api/v1/write", "Remote write URL to send metrics to")
	cardI     = flag.Int("cardI", 1000, "Number of unique time series to generate")
	cardY     = flag.Int("cardY", 1, "Number of unique time series to generate")
	batchSize = flag.Int("batchSize", 500, "Number of time series to send per request")
	interval  = flag.Duration("interval", 5*time.Second, "Interval between send rounds")
	template  = flag.String("template", `cegen_demo{id="[cardI]"}`, ``)
)

func main() {
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Parse()

	if *cardI <= 0 {
		fmt.Fprintln(os.Stderr, "-cardinality must be positive")
		os.Exit(1)
	}
	if *batchSize <= 0 {
		fmt.Fprintln(os.Stderr, "-batchSize must be positive")
		os.Exit(1)
	}

	tss, err := buildTimeSeries(*template, *cardI, *cardY)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -template: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("cegen: sending %d unique time series to %s every %s (batch size: %d)\n",
		*cardI, *targetURL, *interval, *batchSize)

	client := &http.Client{Timeout: 30 * time.Second}

	for round := 1; ; round++ {
		sent, failed := sendAll(client, tss, *batchSize)
		fmt.Printf("round %d: sent %d time series (%d failed)\n", round, sent, failed)
		time.Sleep(*interval)
	}
}

// buildTimeSeries pre-generates cardinality unique time series from the given metric template.
// The template is a MetricsQL metric selector such as `foo{bar="bar$i",baz="baz$i"}`.
// The placeholder $i is replaced with the zero-based series index in every label name and value.
func buildTimeSeries(tmpl string, cardI, cardY int) ([]prompb.TimeSeries, error) {
	labelTmpls, err := parseTemplate(tmpl)
	if err != nil {
		return nil, err
	}

	log.Printf("%+v", labelTmpls)

	tss := make([]prompb.TimeSeries, 0, cardI*cardY)
	for i := 0; i < cardI; i++ {
		for y := 0; y < cardY; y++ {
			iStr := fmt.Sprintf("%d", i)
			yStr := fmt.Sprintf("%d", y)
			labels := make([]prompb.Label, len(labelTmpls))
			for j, lt := range labelTmpls {
				labels[j] = prompb.Label{
					Name: lt.Name,
					Value: strings.ReplaceAll(
						strings.ReplaceAll(lt.Value, "[cardY]", yStr),
						"[cardI]",
						iStr,
					),
				}
			}
			tss = append(tss, prompb.TimeSeries{Labels: labels})
		}
	}
	return tss, nil
}

// parseTemplate parses a MetricsQL metric selector and returns its labels as prompb.Label slice.
// The metric name is represented as a label with name "__name__".
func parseTemplate(tmpl string) ([]prompb.Label, error) {
	expr, err := metricsql.Parse(tmpl)
	if err != nil {
		return nil, err
	}
	me, ok := expr.(*metricsql.MetricExpr)
	if !ok || len(me.LabelFilterss) == 0 {
		return nil, fmt.Errorf("template must be a metric selector, got %T", expr)
	}
	lfs := me.LabelFilterss[0]
	labels := make([]prompb.Label, len(lfs))
	for i, lf := range lfs {
		labels[i] = prompb.Label{Name: lf.Label, Value: lf.Value}
	}
	return labels, nil
}

// sendAll sends all time series in batches. Returns the count of successfully sent series and failed ones.
func sendAll(client *http.Client, tss []prompb.TimeSeries, batchSize int) (sent, failed int) {
	now := time.Now().UnixMilli()

	for start := 0; start < len(tss); start += batchSize {
		end := start + batchSize
		if end > len(tss) {
			end = len(tss)
		}
		batch := tss[start:end]

		// Stamp each series with the current timestamp.
		for i := range batch {
			batch[i].Samples = []prompb.Sample{
				{Value: 1, Timestamp: now},
			}
		}

		if err := sendBatch(client, batch); err != nil {
			fmt.Fprintf(os.Stderr, "send error: %s\n", err)
			failed += len(batch)
		} else {
			sent += len(batch)
		}
	}
	return sent, failed
}

// sendBatch encodes and sends a single batch of time series via Prometheus remote write.
func sendBatch(client *http.Client, tss []prompb.TimeSeries) error {
	wr := &prompb.WriteRequest{Timeseries: tss}

	// Marshal to protobuf.
	pbData := wr.MarshalProtobuf(nil)

	// Compress with snappy (standard Prometheus remote write encoding).
	compressed := snappy.Encode(nil, pbData)

	req, err := http.NewRequest(http.MethodPost, *targetURL, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}
