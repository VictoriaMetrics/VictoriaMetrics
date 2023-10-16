package unittest

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	testutil "github.com/VictoriaMetrics/VictoriaMetrics/app/victoria-metrics/test"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/metricsql"
)

// series holds input_series defined in the test file
type series struct {
	Series string `yaml:"series"`
	Values string `yaml:"values"`
}

// sequenceValue is an omittable value in a sequence of time series values.
type sequenceValue struct {
	Value   float64
	Omitted bool
}

func httpWrite(address string, r io.Reader) {
	resp, err := http.Post(address, "", r)
	if err != nil {
		logger.Fatalf("failed to send to storage: %v", err)
	}
	resp.Body.Close()
}

// writeInputSeries send input series to vmstorage and flush them
func writeInputSeries(input []series, interval *promutils.Duration, startStamp time.Time, dst string) error {
	r := testutil.WriteRequest{}
	for _, data := range input {
		expr, err := metricsql.Parse(data.Series)
		if err != nil {
			return fmt.Errorf("failed to parse series %s: %v", data.Series, err)
		}
		promvals, err := parseInputValue(data.Values, true)
		if err != nil {
			return fmt.Errorf("failed to parse input series value %s: %v", data.Values, err)
		}
		metricExpr, ok := expr.(*metricsql.MetricExpr)
		if !ok {
			return fmt.Errorf("failed to parse series %s to metric expr: %v", data.Series, err)
		}
		samples := make([]testutil.Sample, 0, len(promvals))
		ts := startStamp
		for _, v := range promvals {
			if !v.Omitted {
				samples = append(samples, testutil.Sample{
					Timestamp: ts.UnixMilli(),
					Value:     v.Value,
				})
			}
			ts = ts.Add(interval.Duration())
		}
		var ls []testutil.Label
		for _, filter := range metricExpr.LabelFilterss[0] {
			ls = append(ls, testutil.Label{Name: filter.Label, Value: filter.Value})
		}
		r.Timeseries = append(r.Timeseries, testutil.TimeSeries{Labels: ls, Samples: samples})
	}

	data, err := testutil.Compress(r)
	if err != nil {
		return fmt.Errorf("failed to compress data: %v", err)
	}
	// write input series to vm
	httpWrite(dst, bytes.NewBuffer(data))
	vmstorage.Storage.DebugFlush()
	return nil
}

// parseInputValue support input like "1", "1+1x1 _ -4 3+20x1", see more examples in test.
func parseInputValue(input string, origin bool) ([]sequenceValue, error) {
	var res []sequenceValue
	items := strings.Split(input, " ")
	reg := regexp.MustCompile(`\D?\d*\D?`)
	for _, item := range items {
		if item == "stale" {
			res = append(res, sequenceValue{Value: decimal.StaleNaN})
			continue
		}
		vals := reg.FindAllString(item, -1)
		switch len(vals) {
		case 1:
			if vals[0] == "_" {
				res = append(res, sequenceValue{Omitted: true})
				continue
			}
			v, err := strconv.ParseFloat(vals[0], 64)
			if err != nil {
				return nil, err
			}
			res = append(res, sequenceValue{Value: v})
			continue
		case 2:
			p1 := vals[0][:len(vals[0])-1]
			v2, err := strconv.ParseInt(vals[1], 10, 64)
			if err != nil {
				return nil, err
			}
			option := vals[0][len(vals[0])-1]
			switch option {
			case '+':
				v1, err := strconv.ParseFloat(p1, 64)
				if err != nil {
					return nil, err
				}
				res = append(res, sequenceValue{Value: v1 + float64(v2)})
			case 'x':
				for i := int64(0); i <= v2; i++ {
					if p1 == "_" {
						if i == 0 {
							i = 1
						}
						res = append(res, sequenceValue{Omitted: true})
						continue
					}
					v1, err := strconv.ParseFloat(p1, 64)
					if err != nil {
						return nil, err
					}
					if !origin || v1 == 0 {
						res = append(res, sequenceValue{Value: v1 * float64(i)})
						continue
					}
					newVal := fmt.Sprintf("%s+0x%s", p1, vals[1])
					newRes, err := parseInputValue(newVal, false)
					if err != nil {
						return nil, err
					}
					res = append(res, newRes...)
					break
				}

			default:
				return nil, fmt.Errorf("got invalid operation %b", option)
			}
		case 3:
			r1, err := parseInputValue(fmt.Sprintf("%s%s", vals[1], vals[2]), false)
			if err != nil {
				return nil, err
			}
			p1 := vals[0][:len(vals[0])-1]
			v1, err := strconv.ParseFloat(p1, 64)
			if err != nil {
				return nil, err
			}
			option := vals[0][len(vals[0])-1]
			var isAdd bool
			if option == '+' {
				isAdd = true
			}
			for _, r := range r1 {
				if isAdd {
					res = append(res, sequenceValue{
						Value: r.Value + v1,
					})
				} else {
					res = append(res, sequenceValue{
						Value: v1 - r.Value,
					})
				}
			}
		default:
			return nil, fmt.Errorf("unsupported input %s", input)
		}
	}
	return res, nil
}
