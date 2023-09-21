package common

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
)

// TestReloadStreamAggrConfigReload supposed to test concurrent
// execution of stream configuration update.
// Should be executed with -race flag
func TestReloadStreamAggrConfigReload(t *testing.T) {
	tssInput := mustParsePromMetrics(`foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8`)

	streamAggrConfigs := []string{`
- interval: 1m
  outputs: [max]
`, `
- interval: 1m
  outputs: [max]
- interval: 2m
  outputs: [max]
- interval: 3m
  outputs: [max]
`, `
- interval: 3m
  outputs: [max]
- interval: 2m
  outputs: [max]
- interval: 1m
  outputs: [max]
`, `
- interval: 1m
  outputs: [last]
- interval: 1m
  outputs: [max]
`}

	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}

	*streamAggrConfig = f.Name()
	defer func() {
		*streamAggrConfig = ""
		_ = os.Remove(f.Name())
	}()

	logger.SetOutputForTests(&bytes.Buffer{})

	writeToFile(t, f.Name(), streamAggrConfigs[0])
	reloadStreamAggrConfig(true)

	syncCh := make(chan struct{})
	go func() {
		r := rand.New(rand.NewSource(1))
		for {
			select {
			case <-syncCh:
				return
			default:
				n := r.Intn(len(streamAggrConfigs))
				writeToFile(t, f.Name(), streamAggrConfigs[n])
				reloadStreamAggrConfig(true)
				time.Sleep(time.Millisecond * 50)
			}
		}
	}()

	for i := 0; i < 1e4; i++ {
		sasGlobal.Load().Push(tssInput, nil)
	}
	close(syncCh)
}

func writeToFile(t *testing.T, file, b string) {
	t.Helper()
	err := os.WriteFile(file, []byte(b), 0644)
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}
}

func mustParsePromMetrics(s string) []prompbmarshal.TimeSeries {
	var rows prometheus.Rows
	errLogger := func(s string) {
		panic(fmt.Errorf("unexpected error when parsing Prometheus metrics: %s", s))
	}
	rows.UnmarshalWithErrLogger(s, errLogger)
	var tss []prompbmarshal.TimeSeries
	samples := make([]prompbmarshal.Sample, 0, len(rows.Rows))
	for _, row := range rows.Rows {
		labels := make([]prompbmarshal.Label, 0, len(row.Tags)+1)
		labels = append(labels, prompbmarshal.Label{
			Name:  "__name__",
			Value: row.Metric,
		})
		for _, tag := range row.Tags {
			labels = append(labels, prompbmarshal.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		samples = append(samples, prompbmarshal.Sample{
			Value:     row.Value,
			Timestamp: row.Timestamp,
		})
		ts := prompbmarshal.TimeSeries{
			Labels:  labels,
			Samples: samples[len(samples)-1:],
		}
		tss = append(tss, ts)
	}
	return tss
}
