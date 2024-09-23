package promql

import (
	"math"
	"testing"
	"time"

	"github.com/VictoriaMetrics/metricsql"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestEscapeDots(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		result := escapeDots(s)
		if result != resultExpected {
			t.Fatalf("unexpected result for escapeDots(%q); got\n%s\nwant\n%s", s, result, resultExpected)
		}
	}
	f("", "")
	f("a", "a")
	f("foobar", "foobar")
	f(".", `\.`)
	f(".*", `.*`)
	f(".+", `.+`)
	f("..", `\.\.`)
	f("foo.b.{2}ar..+baz.*", `foo\.b.{2}ar\..+baz.*`)
}

func TestEscapeDotsInRegexpLabelFilters(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		e, err := metricsql.Parse(s)
		if err != nil {
			t.Fatalf("unexpected error in metricsql.Parse(%q): %s", s, err)
		}
		e = escapeDotsInRegexpLabelFilters(e)
		result := e.AppendString(nil)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result for escapeDotsInRegexpLabelFilters(%q);\ngot\n%s\nwant\n%s", s, result, resultExpected)
		}
	}
	f("2", "2")
	f(`foo.bar + 123`, `foo.bar + 123`)
	f(`foo{bar=~"baz.xx.yyy"}`, `foo{bar=~"baz\\.xx\\.yyy"}`)
	f(`sum(a.b{c="d.e",x=~"a.b.+[.a]",y!~"aaa.bb|cc.dd"}) + avg_over_time(1,sum({x=~"aa.bb"}))`, `sum(a.b{c="d.e",x=~"a\\.b.+[\\.a]",y!~"aaa\\.bb|cc\\.dd"}) + avg_over_time(1, sum({x=~"aa\\.bb"}))`)
}

func TestExecSuccess(t *testing.T) {
	start := int64(1000e3)
	end := int64(2000e3)
	step := int64(200e3)
	timestampsExpected := []int64{1000e3, 1200e3, 1400e3, 1600e3, 1800e3, 2000e3}
	metricNameExpected := storage.MetricName{}

	f := func(q string, resultExpected []netstorage.Result) {
		t.Helper()
		ec := &EvalConfig{
			Start:              start,
			End:                end,
			Step:               step,
			MaxPointsPerSeries: 1e4,
			MaxSeries:          1000,
			Deadline:           searchutils.NewDeadline(time.Now(), time.Minute, ""),
			RoundDigits:        100,
		}
		for i := 0; i < 5; i++ {
			result, err := Exec(nil, ec, q, false)
			if err != nil {
				t.Fatalf(`unexpected error when executing %q: %s`, q, err)
			}
			testResultsEqual(t, result, resultExpected)
		}
	}

	t.Run("simple-number", func(t *testing.T) {
		t.Parallel()
		q := `123`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 123, 123, 123},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("int_with_underscores", func(t *testing.T) {
		t.Parallel()
		q := `123_456_789`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123456789, 123456789, 123456789, 123456789, 123456789, 123456789},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("float_with_underscores", func(t *testing.T) {
		t.Parallel()
		q := `1_2.3_456_789`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{12.3456789, 12.3456789, 12.3456789, 12.3456789, 12.3456789, 12.3456789},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("duration-constant", func(t *testing.T) {
		t.Parallel()
		q := `1h23m5S`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4985, 4985, 4985, 4985, 4985, 4985},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("num-with-suffix-1", func(t *testing.T) {
		t.Parallel()
		q := `123M`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123e6, 123e6, 123e6, 123e6, 123e6, 123e6},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("num-with-suffix-2", func(t *testing.T) {
		t.Parallel()
		q := `1.23TB`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1.23e12, 1.23e12, 1.23e12, 1.23e12, 1.23e12, 1.23e12},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("num-with-suffix-3", func(t *testing.T) {
		t.Parallel()
		q := `1.23Mib`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1.23 * (1 << 20), 1.23 * (1 << 20), 1.23 * (1 << 20), 1.23 * (1 << 20), 1.23 * (1 << 20), 1.23 * (1 << 20)},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("num-with-suffix-4", func(t *testing.T) {
		t.Parallel()
		q := `1.23mib`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1.23 * (1 << 20), 1.23 * (1 << 20), 1.23 * (1 << 20), 1.23 * (1 << 20), 1.23 * (1 << 20), 1.23 * (1 << 20)},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("num-with-suffix-5", func(t *testing.T) {
		t.Parallel()
		q := `1_234M`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1234e6, 1234e6, 1234e6, 1234e6, 1234e6, 1234e6},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("simple-arithmetic", func(t *testing.T) {
		t.Parallel()
		q := `-1+2 *3 ^ 4+5%6`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{166, 166, 166, 166, 166, 166},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("simple-string", func(t *testing.T) {
		t.Parallel()
		q := `"foobar"`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("simple-string-op-number", func(t *testing.T) {
		t.Parallel()
		q := `1+"foobar"*2%9`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("scalar-vector-arithmetic", func(t *testing.T) {
		t.Parallel()
		q := `scalar(-1)+2 *vector(3) ^ scalar(4)+5`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{166, 166, 166, 166, 166, 166},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("scalar-string-nonnum", func(t *testing.T) {
		t.Parallel()
		q := `scalar("fooobar")`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("scalar-string-num", func(t *testing.T) {
		t.Parallel()
		q := `scalar("-12.34")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-12.34, -12.34, -12.34, -12.34, -12.34, -12.34},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("bitmap_and(0xB3, 0x11)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_and(0xB3, 0x11)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{17, 17, 17, 17, 17, 17},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("bitmap_and(time(), 0x11)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_and(time(), 0x11)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 16, 16, 0, 0, 16},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("bitmap_and(NaN, 1)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_and(NaN, 1)`
		f(q, nil)
	})
	t.Run("bitmap_and(1, NaN)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_and(1, NaN)`
		f(q, nil)
	})
	t.Run("bitmap_and(round(rand(1) > 0.5, 1), 1)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_and(round(rand(1) > 0.5, 1), 1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, nan, nan, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("bitmap_or(0xA2, 0x11)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_or(0xA2, 0x11)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{179, 179, 179, 179, 179, 179},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("bitmap_or(time(), 0x11)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_or(time(), 0x11)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1017, 1201, 1401, 1617, 1817, 2001},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("bitmap_or(NaN, 1)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_or(NaN, 1)`
		f(q, nil)
	})
	t.Run("bitmap_or(round(rand(1) > 0.5, 1), 1)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_or(round(rand(1) > 0.5, 1), 1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, nan, nan, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("bitmap_xor(0xB3, 0x11)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_xor(0xB3, 0x11)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{162, 162, 162, 162, 162, 162},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("bitmap_xor(time(), 0x11)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_xor(time(), 0x11)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1017, 1185, 1385, 1617, 1817, 1985},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("bitmap_xor(NaN, 1)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_xor(NaN, 1)`
		f(q, nil)
	})
	t.Run("bitmap_xor(round(rand(1) > 0.5, 1), 1)", func(t *testing.T) {
		t.Parallel()
		q := `bitmap_xor(round(rand(1) > 0.5, 1), 1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, nan, nan, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("timezone_offset(UTC)", func(t *testing.T) {
		t.Parallel()
		q := `timezone_offset("UTC")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("timezone_offset(America/New_York)", func(t *testing.T) {
		t.Parallel()
		q := `timezone_offset("America/New_York")`
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			t.Fatalf("cannot obtain timezone: %s", err)
		}
		at := time.Unix(timestampsExpected[0]/1000, 0)
		_, offset := at.In(loc).Zone()
		off := float64(offset)
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{off, off, off, off, off, off},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("timezone_offset(Local)", func(t *testing.T) {
		t.Parallel()
		q := `timezone_offset("Local")`
		loc, err := time.LoadLocation("Local")
		if err != nil {
			t.Fatalf("cannot obtain timezone: %s", err)
		}
		at := time.Unix(timestampsExpected[0]/1000, 0)
		_, offset := at.In(loc).Zone()
		off := float64(offset)
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{off, off, off, off, off, off},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time()", func(t *testing.T) {
		t.Parallel()
		q := `time()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() offset 0s", func(t *testing.T) {
		t.Parallel()
		q := `time() offset 0s`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("(a, b) offset 0s", func(t *testing.T) {
		t.Parallel()
		q := `sort((label_set(time(), "foo", "bar"), label_set(time()+10, "foo", "baz")) offset 0s)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1010, 1210, 1410, 1610, 1810, 2010},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("baz"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run("time()[:100s] offset 0s", func(t *testing.T) {
		t.Parallel()
		q := `time()[:100s] offset 0s`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time()[:100] offset 0", func(t *testing.T) {
		t.Parallel()
		q := `time()[:100] offset 0`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() offset 1h40s0ms", func(t *testing.T) {
		t.Parallel()
		q := `time() offset 1h40s0ms`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-2800, -2600, -2400, -2200, -2000, -1800},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() offset 3640", func(t *testing.T) {
		t.Parallel()
		q := `time() offset 3640`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-2800, -2600, -2400, -2200, -2000, -1800},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() offset -1h40s0ms", func(t *testing.T) {
		t.Parallel()
		q := `time() offset -1h40s0ms`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4600, 4800, 5000, 5200, 5400, 5600},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() offset -100s", func(t *testing.T) {
		t.Parallel()
		q := `time() offset -100s`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("(a, b) offset 100s", func(t *testing.T) {
		t.Parallel()
		q := `sort((label_set(time(), "foo", "bar"), label_set(time()+10, "foo", "baz")) offset 100s)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{800, 1000, 1200, 1400, 1600, 1800},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{810, 1010, 1210, 1410, 1610, 1810},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("baz"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run("(a offset 100s, b offset 50s)", func(t *testing.T) {
		t.Parallel()
		q := `sort((label_set(time() offset 100s, "foo", "bar"), label_set(time()+10, "foo", "baz") offset 50s))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{800, 1000, 1200, 1400, 1600, 1800},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{810, 1010, 1210, 1410, 1610, 1810},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("baz"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run("(a offset 100s, b offset 50s) offset 400s", func(t *testing.T) {
		t.Parallel()
		q := `sort((label_set(time() offset 100s, "foo", "bar"), label_set(time()+10, "foo", "baz") offset 50s) offset 400s)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{400, 600, 800, 1000, 1200, 1400},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{410, 610, 810, 1010, 1210, 1410},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("baz"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run("(a offset -100s, b offset -50s) offset -400s", func(t *testing.T) {
		t.Parallel()
		q := `sort((label_set(time() offset -100s, "foo", "bar"), label_set(time()+10, "foo", "baz") offset -50s) offset -400s)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1400, 1600, 1800, 2000, 2200, 2400},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1410, 1610, 1810, 2010, 2210, 2410},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("baz"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run("1h", func(t *testing.T) {
		t.Parallel()
		q := `1h`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3600, 3600, 3600, 3600, 3600, 3600},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("sum_over_time(time()[1h]) / 1h", func(t *testing.T) {
		t.Parallel()
		q := `sum_over_time(time()[1h]) / 1h`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-3.5, -2.5, -1.5, -0.5, 0.5, 1.5},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time()[:100s] offset 100s", func(t *testing.T) {
		t.Parallel()
		q := `time()[:100s] offset 100s`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{900, 1100, 1300, 1500, 1700, 1900},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time()[300s:100s] offset 100s", func(t *testing.T) {
		t.Parallel()
		q := `time()[300s:100s] offset 100s`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{900, 1100, 1300, 1500, 1700, 1900},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time()[300:100] offset 100", func(t *testing.T) {
		t.Parallel()
		q := `time()[300:100] offset 100`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{900, 1100, 1300, 1500, 1700, 1900},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time()[1.5i:0.5i] offset 0.5i", func(t *testing.T) {
		t.Parallel()
		q := `time()[1.5i:0.5i] offset 0.5i`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{900, 1100, 1300, 1500, 1700, 1900},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time()[300s] offset 100s", func(t *testing.T) {
		t.Parallel()
		q := `time()[300s] offset 100s`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{800, 1000, 1200, 1400, 1600, 1800},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time()[300s]", func(t *testing.T) {
		t.Parallel()
		q := `time()[300s]`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() + time()", func(t *testing.T) {
		t.Parallel()
		q := `time() + time()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("timestamp(123)", func(t *testing.T) {
		t.Parallel()
		q := `timestamp(123)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("timestamp(time())", func(t *testing.T) {
		t.Parallel()
		q := `timestamp(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("timestamp(456/time()+123)", func(t *testing.T) {
		t.Parallel()
		q := `timestamp(456/time()+123)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("timestamp(time()>=1600)", func(t *testing.T) {
		t.Parallel()
		q := `timestamp(time()>=1600)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("timestamp(alias(time()>=1600))", func(t *testing.T) {
		t.Parallel()
		q := `timestamp(alias(time()>=1600,"foo"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("tlast_change_over_time(hit_last)", func(t *testing.T) {
		t.Parallel()
		q := `tlast_change_over_time(
			time()[1h]
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("tlast_change_over_time(hit_middle)", func(t *testing.T) {
		t.Parallel()
		q := `tlast_change_over_time(
			(time() >=bool 1600)[1h]
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1600, 1600},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("tlast_change_over_time(miss)", func(t *testing.T) {
		t.Parallel()
		q := `tlast_change_over_time(
			1[1h]
		)`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("timestamp_with_name(alias(time()>=1600))", func(t *testing.T) {
		t.Parallel()
		q := `timestamp_with_name(alias(time()>=1600,"foo"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foo")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time()/100", func(t *testing.T) {
		t.Parallel()
		q := `time()/100`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 12, 14, 16, 18, 20},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("1e3/time()*2*9*7", func(t *testing.T) {
		t.Parallel()
		q := `1e3/time()*2*9*7`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{126, 105, 90, 78.75, 70, 63},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("minute()", func(t *testing.T) {
		t.Parallel()
		q := `minute()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{16, 20, 23, 26, 30, 33},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("day_of_month()", func(t *testing.T) {
		t.Parallel()
		q := `day_of_month(time()*1e4)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{26, 19, 12, 5, 28, 20},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("day_of_week()", func(t *testing.T) {
		t.Parallel()
		q := `day_of_week(time()*1e4)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 2, 5, 0, 2, 4},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("day_of_year()", func(t *testing.T) {
		t.Parallel()
		q := `day_of_year(time()*1e4)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{116, 139, 163, 186, 209, 232},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("days_in_month()", func(t *testing.T) {
		t.Parallel()
		q := `days_in_month(time()*2e4)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{31, 31, 30, 31, 28, 30},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("hour()", func(t *testing.T) {
		t.Parallel()
		q := `hour(time()*1e4)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{17, 21, 0, 4, 8, 11},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("month()", func(t *testing.T) {
		t.Parallel()
		q := `month(time()*1e4)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4, 5, 6, 7, 7, 8},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("year()", func(t *testing.T) {
		t.Parallel()
		q := `year(time()*1e5)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1973, 1973, 1974, 1975, 1975, 1976},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("minute(30*60+time())", func(t *testing.T) {
		t.Parallel()
		q := `minute(30*60+time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{46, 50, 53, 56, 0, 3},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`minute(series_with_NaNs)`, func(t *testing.T) {
		t.Parallel()
		q := `minute(time() <= 1200 or time() > 1600)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{16, 20, nan, nan, 30, 33},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("rate({})", func(t *testing.T) {
		t.Parallel()
		q := `rate({})`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("abs(1500-time())", func(t *testing.T) {
		t.Parallel()
		q := `abs(1500-time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{500, 300, 100, 100, 300, 500},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("abs(-time()+1300)", func(t *testing.T) {
		t.Parallel()
		q := `abs(-time()+1300)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{300, 100, 100, 300, 500, 700},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("ceil(time() / 900)", func(t *testing.T) {
		t.Parallel()
		q := `ceil(time()/500)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 3, 3, 4, 4, 4},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("absent(time())", func(t *testing.T) {
		t.Parallel()
		q := `absent(time())`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("absent_over_time(time())", func(t *testing.T) {
		t.Parallel()
		q := `absent_over_time(time())`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("present_over_time(time())", func(t *testing.T) {
		t.Parallel()
		q := `present_over_time(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("present_over_time(time()[100:300])", func(t *testing.T) {
		t.Parallel()
		q := `present_over_time(time()[100:300])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1, nan, nan, 1, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("present_over_time(time()<10m)", func(t *testing.T) {
		t.Parallel()
		q := `present_over_time(time()<1600)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("absent(123)", func(t *testing.T) {
		t.Parallel()
		q := `absent(123)`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("absent(vector(scalar(123)))", func(t *testing.T) {
		t.Parallel()
		q := `absent(vector(scalar(123)))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("absent(NaN)", func(t *testing.T) {
		t.Parallel()
		q := `absent(NaN)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("absent_over_time(nan[200s:10s])", func(t *testing.T) {
		t.Parallel()
		q := `absent_over_time(nan[200s:10s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`absent(scalar(multi-timeseries))`, func(t *testing.T) {
		t.Parallel()
		q := `
		absent(label_set(scalar(1 or label_set(2, "xx", "foo")), "yy", "foo"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`absent_over_time(non-nan)`, func(t *testing.T) {
		t.Parallel()
		q := `
		absent_over_time(time())`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`absent_over_time(nan)`, func(t *testing.T) {
		t.Parallel()
		q := `
		absent_over_time((time() < 1500)[300s:])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, nan, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`absent_over_time(multi-ts)`, func(t *testing.T) {
		t.Parallel()
		q := `
		absent_over_time((
			alias((time() < 1400)[200s:], "one"),
			alias((time() > 1600)[200s:], "two"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1, 1, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`absent(time() > 1500)`, func(t *testing.T) {
		t.Parallel()
		q := `
		absent(time() > 1500)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("clamp(time(), 1400, 1800)", func(t *testing.T) {
		t.Parallel()
		q := `clamp(time(), 1400, 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1400, 1400, 1400, 1600, 1800, 1800},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("clamp_max(time(), 1400)", func(t *testing.T) {
		t.Parallel()
		q := `clamp_max(time(), 1400)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1400, 1400, 1400},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`clamp_max(alias(time(),"foobar"), 1400)`, func(t *testing.T) {
		t.Parallel()
		q := `clamp_max(alias(time(), "foobar"), 1400)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1400, 1400, 1400},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`CLAmp_MAx(alias(time(),"foobar"), 1400)`, func(t *testing.T) {
		t.Parallel()
		q := `CLAmp_MAx(alias(time(), "foobar"), 1400)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1400, 1400, 1400},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("clamp_min(time(), -time()+3000)", func(t *testing.T) {
		t.Parallel()
		q := `clamp_min(time(), -time()+2500)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1500, 1300, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("clamp_min(1500, time())", func(t *testing.T) {
		t.Parallel()
		q := `clamp_min(1500, time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1500, 1500, 1500, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("exp(time()/1e3)", func(t *testing.T) {
		t.Parallel()
		q := `exp(alias(time()/1e3, "foobar"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2.718281828459045, 3.3201169227365472, 4.0551999668446745, 4.953032424395115, 6.0496474644129465, 7.38905609893065},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("exp(time()/1e3) keep_metric_names", func(t *testing.T) {
		t.Parallel()
		q := `exp(alias(time()/1e3, "foobar")) keep_metric_names`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2.718281828459045, 3.3201169227365472, 4.0551999668446745, 4.953032424395115, 6.0496474644129465, 7.38905609893065},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() @ 1h", func(t *testing.T) {
		t.Parallel()
		q := `time() @ 1h`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3600, 3600, 3600, 3600, 3600, 3600},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() @ start()", func(t *testing.T) {
		t.Parallel()
		q := `time() @ start()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1000, 1000, 1000, 1000, 1000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() @ end()", func(t *testing.T) {
		t.Parallel()
		q := `time() @ end()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2000, 2000, 2000, 2000, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() @ end() offset 10m", func(t *testing.T) {
		t.Parallel()
		q := `time() @ end() offset 10m`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1400, 1400, 1400, 1400, 1400, 1400},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time() @ (end()-10m)", func(t *testing.T) {
		t.Parallel()
		q := `time() @ (end()-10m)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1400, 1400, 1400, 1400, 1400, 1400},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("rand()", func(t *testing.T) {
		t.Parallel()
		q := `round(rand()/2)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("rand(0)", func(t *testing.T) {
		t.Parallel()
		q := `round(rand(0), 0.01)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.95, 0.24, 0.66, 0.05, 0.37, 0.28},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("rand_normal()", func(t *testing.T) {
		t.Parallel()
		q := `clamp_max(clamp_min(0, rand_normal()), 0)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("rand_normal(0)", func(t *testing.T) {
		t.Parallel()
		q := `round(rand_normal(0), 0.01)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-0.28, 0.57, -1.69, 0.2, 1.92, 0.9},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("rand_exponential()", func(t *testing.T) {
		t.Parallel()
		q := `clamp_max(clamp_min(0, rand_exponential()), 0)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("rand_exponential(0)", func(t *testing.T) {
		t.Parallel()
		q := `round(rand_exponential(0), 0.01)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4.67, 0.16, 3.05, 0.06, 1.86, 0.78},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("now()", func(t *testing.T) {
		t.Parallel()
		q := `round(now()/now())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("pi()", func(t *testing.T) {
		t.Parallel()
		q := `pi()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3.141592653589793, 3.141592653589793, 3.141592653589793, 3.141592653589793, 3.141592653589793, 3.141592653589793},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("sin()", func(t *testing.T) {
		t.Parallel()
		q := `sin(pi()*(2000-time())/1000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1.2246467991473515e-16, 0.5877852522924732, 0.9510565162951536, 0.9510565162951535, 0.5877852522924731, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("sinh()", func(t *testing.T) {
		t.Parallel()
		q := `sinh(pi()*(2000-time())/1000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{11.548739357257748, 6.132140673514712, 3.217113080357038, 1.6144880404748523, 0.6704839982471175, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("asin()", func(t *testing.T) {
		t.Parallel()
		q := `asin((2000-time())/1000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1.5707963267948966, 0.9272952180016123, 0.6435011087932843, 0.41151684606748806, 0.20135792079033082, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("asinh(sinh)", func(t *testing.T) {
		t.Parallel()
		q := `asinh(sinh((2000-time())/1000))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 0.8000000000000002, 0.6, 0.4000000000000001, 0.2, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("atan2()", func(t *testing.T) {
		t.Parallel()
		q := `time() atan2 time()/10`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.07853981633974483, 0.07853981633974483, 0.07853981633974483, 0.07853981633974483, 0.07853981633974483, 0.07853981633974483},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("atan()", func(t *testing.T) {
		t.Parallel()
		q := `atan((2000-time())/1000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.7853981633974483, 0.6747409422235526, 0.5404195002705842, 0.3805063771123649, 0.19739555984988078, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("atanh(tanh)", func(t *testing.T) {
		t.Parallel()
		q := `atanh(tanh((2000-time())/1000))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 0.8000000000000002, 0.6, 0.4000000000000001, 0.2, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("cos()", func(t *testing.T) {
		t.Parallel()
		q := `cos(pi()*(2000-time())/1000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-1, -0.8090169943749475, -0.30901699437494734, 0.30901699437494745, 0.8090169943749473, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("acos()", func(t *testing.T) {
		t.Parallel()
		q := `acos((2000-time())/1000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0.6435011087932843, 0.9272952180016123, 1.1592794807274085, 1.3694384060045657, 1.5707963267948966},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("acosh(cosh)", func(t *testing.T) {
		t.Parallel()
		q := `acosh(cosh((2000-time())/1000))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 0.8000000000000002, 0.5999999999999999, 0.40000000000000036, 0.20000000000000023, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("rad(deg)", func(t *testing.T) {
		t.Parallel()
		q := `rad(deg(time()/500))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2.3999999999999995, 2.8, 3.2, 3.6, 4},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("floor(time()/500)", func(t *testing.T) {
		t.Parallel()
		q := `floor(time()/500)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 3, 3, 4},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("sqrt(time())", func(t *testing.T) {
		t.Parallel()
		q := `sqrt(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{31.622776601683793, 34.64101615137755, 37.416573867739416, 40, 42.42640687119285, 44.721359549995796},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("ln(time())", func(t *testing.T) {
		t.Parallel()
		q := `ln(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.907755278982137, 7.090076835776092, 7.24422751560335, 7.3777589082278725, 7.495541943884256, 7.600902459542082},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("log2(time())", func(t *testing.T) {
		t.Parallel()
		q := `log2(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{9.965784284662087, 10.228818690495881, 10.451211111832329, 10.643856189774725, 10.813781191217037, 10.965784284662087},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("log10(time())", func(t *testing.T) {
		t.Parallel()
		q := `log10(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3, 3.0791812460476247, 3.1461280356782377, 3.2041199826559246, 3.255272505103306, 3.3010299956639813},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("time()*(-4)^0.5", func(t *testing.T) {
		t.Parallel()
		q := `time()*(-4)^0.5`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("time()*-4^0.5", func(t *testing.T) {
		t.Parallel()
		q := `time()*-4^0.5`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-2000, -2400, -2800, -3200, -3600, -4000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run("default_for_nan_series", func(t *testing.T) {
		t.Parallel()
		q := `label_set(0, "foo", "bar")/0 default 7`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{7, 7, 7, 7, 7, 7},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`alias()`, func(t *testing.T) {
		t.Parallel()
		q := `alias(time(), "foobar")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_set(tag)`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(time(), "tagname", "tagvalue")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("tagname"),
			Value: []byte("tagvalue"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_set(metricname)`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(time(), "__name__", "foobar")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_set(metricname, tag)`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(
			label_set(time(), "__name__", "foobar"),
			"tagname", "tagvalue"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("tagname"),
			Value: []byte("tagvalue"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_set(del_metricname)`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(
			label_set(time(), "__name__", "foobar"),
			"__name__", ""
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_set(del_tag)`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(
			label_set(time(), "tagname", "foobar"),
			"tagname", ""
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_set(multi)`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(time()+100, "t1", "v1", "t2", "v2", "__name__", "v3")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1300, 1500, 1700, 1900, 2100},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("v3")
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("t1"),
				Value: []byte("v1"),
			},
			{
				Key:   []byte("t2"),
				Value: []byte("v2"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_map(match)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(label_map((
			label_set(time(), "label", "v1"),
			label_set(time()+100, "label", "v2"),
			label_set(time()+200, "label", "v3"),
			label_set(time()+300, "x", "y"),
			label_set(time()+400, "label", "v4"),
		), "label", "v1", "foo", "v2", "bar", "", "qwe", "v4", ""))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("label"),
			Value: []byte("foo"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1300, 1500, 1700, 1900, 2100},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("label"),
			Value: []byte("bar"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1200, 1400, 1600, 1800, 2000, 2200},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("label"),
			Value: []byte("v3"),
		}}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1300, 1500, 1700, 1900, 2100, 2300},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("label"),
				Value: []byte("qwe"),
			},
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		r5 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1400, 1600, 1800, 2000, 2200, 2400},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r1, r2, r3, r4, r5}
		f(q, resultExpected)
	})
	t.Run(`label_uppercase`, func(t *testing.T) {
		t.Parallel()
		q := `label_uppercase(
			label_set(time(), "foo", "bAr", "XXx", "yyy", "zzz", "abc"),
			"foo", "XXx", "aaa"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("XXx"),
				Value: []byte("YYY"),
			},
			{
				Key:   []byte("foo"),
				Value: []byte("BAR"),
			},
			{
				Key:   []byte("zzz"),
				Value: []byte("abc"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_lowercase`, func(t *testing.T) {
		t.Parallel()
		q := `label_lowercase(
			label_set(time(), "foo", "bAr", "XXx", "yyy", "zzz", "aBc"),
			"foo", "XXx", "aaa"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("XXx"),
				Value: []byte("yyy"),
			},
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("zzz"),
				Value: []byte("aBc"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_copy(new_tag)`, func(t *testing.T) {
		t.Parallel()
		q := `label_copy(
			label_set(time(), "tagname", "foobar"),
			"tagname", "xxx"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("tagname"),
				Value: []byte("foobar"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("foobar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_move(new_tag)`, func(t *testing.T) {
		t.Parallel()
		q := `label_move(
			label_set(time(), "tagname", "foobar"),
			"tagname", "xxx"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("foobar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_copy(same_tag)`, func(t *testing.T) {
		t.Parallel()
		q := `label_copy(
			label_set(time(), "tagname", "foobar"),
			"tagname", "tagname"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("tagname"),
				Value: []byte("foobar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_move(same_tag)`, func(t *testing.T) {
		t.Parallel()
		q := `label_move(
			label_set(time(), "tagname", "foobar"),
			"tagname", "tagname"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("tagname"),
				Value: []byte("foobar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_copy(same_tag_nonexisting_src)`, func(t *testing.T) {
		t.Parallel()
		q := `label_copy(
			label_set(time(), "tagname", "foobar"),
			"non-existing-tag", "tagname"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("tagname"),
				Value: []byte("foobar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_move(same_tag_nonexisting_src)`, func(t *testing.T) {
		t.Parallel()
		q := `label_move(
			label_set(time(), "tagname", "foobar"),
			"non-existing-tag", "tagname"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("tagname"),
				Value: []byte("foobar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_copy(existing_tag)`, func(t *testing.T) {
		t.Parallel()
		q := `label_copy(
			label_set(time(), "tagname", "foobar", "xx", "yy"),
			"xx", "tagname"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("tagname"),
				Value: []byte("yy"),
			},
			{
				Key:   []byte("xx"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_move(existing_tag)`, func(t *testing.T) {
		t.Parallel()
		q := `label_move(
			label_set(time(), "tagname", "foobar", "xx", "yy"),
			"xx", "tagname"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("tagname"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_copy(from_metric_group)`, func(t *testing.T) {
		t.Parallel()
		q := `label_copy(
			label_set(time(), "tagname", "foobar", "__name__", "yy"),
			"__name__", "aa"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("yy")
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("aa"),
				Value: []byte("yy"),
			},
			{
				Key:   []byte("tagname"),
				Value: []byte("foobar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_move(from_metric_group)`, func(t *testing.T) {
		t.Parallel()
		q := `label_move(
			label_set(time(), "tagname", "foobar", "__name__", "yy"),
			"__name__", "aa"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("aa"),
				Value: []byte("yy"),
			},
			{
				Key:   []byte("tagname"),
				Value: []byte("foobar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_copy(to_metric_group)`, func(t *testing.T) {
		t.Parallel()
		q := `label_copy(
			label_set(time(), "tagname", "foobar"),
			"tagname", "__name__"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("tagname"),
				Value: []byte("foobar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_move(to_metric_group)`, func(t *testing.T) {
		t.Parallel()
		q := `label_move(
			label_set(time(), "tagname", "foobar"),
			"tagname", "__name__"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`labels_equal()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(labels_equal((
			label_set(10, "instance", "qwe", "host", "rty"),
			label_set(20, "instance", "qwe", "host", "qwe"),
			label_set(30, "aaa", "bbb", "instance", "foo", "host", "foo"),
		), "instance", "host"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{20, 20, 20, 20, 20, 20},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("host"),
				Value: []byte("qwe"),
			},
			{
				Key:   []byte("instance"),
				Value: []byte("qwe"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{30, 30, 30, 30, 30, 30},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("aaa"),
				Value: []byte("bbb"),
			},
			{
				Key:   []byte("host"),
				Value: []byte("foo"),
			},
			{
				Key:   []byte("instance"),
				Value: []byte("foo"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`drop_empty_series()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(drop_empty_series(
			(
				alias(time(), "foo"),
				alias(500 + time(), "bar"),
			) > 2000
		) default 123)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 2100, 2300, 2500},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("bar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`no drop_empty_series()`, func(t *testing.T) {
		t.Parallel()
		q := `sort((
			(
				alias(time(), "foo"),
				alias(500 + time(), "bar"),
			) > 2000
		) default 123)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 123, 123, 123},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("foo")
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 2100, 2300, 2500},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("bar")
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`drop_common_labels(single_series)`, func(t *testing.T) {
		t.Parallel()
		q := `drop_common_labels(label_set(time(), "foo", "bar", "__name__", "xxx", "q", "we"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`drop_common_labels(multi_series)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(drop_common_labels((
			label_set(time(), "foo", "bar", "__name__", "xxx", "q", "we"),
			label_set(time()/10, "foo", "bar", "__name__", "yyy"),
		)))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("xxx")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("q"),
			Value: []byte("we"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 120, 140, 160, 180, 200},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("yyy")
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`drop_common_labels(multi_args)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(drop_common_labels(
			label_set(time(), "foo", "bar", "__name__", "xxx", "q", "we"),
			label_set(time()/10, "foo", "bar", "__name__", "xxx"),
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 120, 140, 160, 180, 200},
			Timestamps: timestampsExpected,
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("q"),
			Value: []byte("we"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`label_keep(nolabels)`, func(t *testing.T) {
		t.Parallel()
		q := `label_keep(time(), "foo", "bar")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_keep(certain_labels)`, func(t *testing.T) {
		t.Parallel()
		q := `label_keep(label_set(time(), "foo", "bar", "__name__", "xxx", "q", "we"), "foo", "nonexisting-label")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_keep(metricname)`, func(t *testing.T) {
		t.Parallel()
		q := `label_keep(label_set(time(), "foo", "bar", "__name__", "xxx", "q", "we"), "nonexisting-label", "__name__")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("xxx")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_del(nolabels)`, func(t *testing.T) {
		t.Parallel()
		q := `label_del(time(), "foo", "bar")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_del(certain_labels)`, func(t *testing.T) {
		t.Parallel()
		q := `label_del(label_set(time(), "foo", "bar", "__name__", "xxx", "q", "we"), "foo", "nonexisting-label")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("xxx")
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("q"),
			Value: []byte("we"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_del(metricname)`, func(t *testing.T) {
		t.Parallel()
		q := `label_del(label_set(time(), "foo", "bar", "__name__", "xxx", "q", "we"), "nonexisting-label", "__name__")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("q"),
				Value: []byte("we"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_join(empty)`, func(t *testing.T) {
		t.Parallel()
		q := `label_join(vector(time()), "tt", "(sep)", "BAR")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_join(tt)`, func(t *testing.T) {
		t.Parallel()
		q := `label_join(vector(time()), "tt", "(sep)", "foo", "BAR")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("tt"),
			Value: []byte("(sep)"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_join(__name__)`, func(t *testing.T) {
		t.Parallel()
		q := `label_join(time(), "__name__", "(sep)", "foo", "BAR", "")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("(sep)(sep)")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_join(label_join)`, func(t *testing.T) {
		t.Parallel()
		q := `label_join(label_join(time(), "__name__", "(sep)", "foo", "BAR"), "xxx", ",", "foobar", "__name__")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("(sep)")
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("xxx"),
			Value: []byte(",(sep)"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_join dst_label is equal to src_label`, func(t *testing.T) {
		t.Parallel()
		q := `label_join(label_join(time(), "bar", "sep1", "a", "b"), "bar", "sep2", "a", "bar")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("bar"),
			Value: []byte("sep2sep1"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_value()`, func(t *testing.T) {
		t.Parallel()
		q := `with (
			x = (
				label_set(time() > 1500, "foo", "123.456", "__name__", "aaa"),
				label_set(-time(), "foo", "bar", "__name__", "bbb"),
				label_set(-time(), "__name__", "bxs"),
				label_set(-time(), "foo", "45", "bar", "xs"),
			)
		)
		sort(x + label_value(x, "foo"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-955, -1155, -1355, -1555, -1755, -1955},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("bar"),
				Value: []byte("xs"),
			},
			{
				Key:   []byte("foo"),
				Value: []byte("45"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1723.456, 1923.456, 2123.456},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("123.456"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`label_transform(mismatch)`, func(t *testing.T) {
		t.Parallel()
		q := `label_transform(time(), "__name__", "foobar", "xx")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_transform(match)`, func(t *testing.T) {
		t.Parallel()
		q := `label_transform(
			label_set(time(), "foo", "a.bar.baz"),
			"foo", "\\.", "-")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("a-bar-baz"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_replace(nonexisting_src)`, func(t *testing.T) {
		t.Parallel()
		q := `label_replace(time(), "__name__", "x${1}y", "foo", ".+")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_replace(nonexisting_src_match)`, func(t *testing.T) {
		t.Parallel()
		q := `label_replace(time(), "foo", "x", "bar", "")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("x"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_replace(nonexisting_src_mismatch)`, func(t *testing.T) {
		t.Parallel()
		q := `label_replace(time(), "foo", "x", "bar", "y")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_replace(mismatch)`, func(t *testing.T) {
		t.Parallel()
		q := `label_replace(label_set(time(), "foo", "foobar"), "__name__", "x${1}y", "foo", "bar(.+)")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("foobar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_replace(match)`, func(t *testing.T) {
		t.Parallel()
		q := `label_replace(time(), "__name__", "x${1}y", "foo", ".*")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("xy")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_replace(label_replace)`, func(t *testing.T) {
		t.Parallel()
		q := `
		label_replace(
			label_replace(
				label_replace(time(), "__name__", "x${1}y", "foo", ".*"),
				"xxx", "foo${1}bar(${1})", "__name__", "(.+)"),
			"xxx", "AA$1", "xxx", "foox(.+)"
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("xy")
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("xxx"),
			Value: []byte("AAybar(xy)"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_match()`, func(t *testing.T) {
		t.Parallel()
		q := `
		label_match((
			alias(time(), "foo"),
			alias(2*time(), "bar"),
		), "__name__", "f.+")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foo")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_mismatch()`, func(t *testing.T) {
		t.Parallel()
		q := `
		label_mismatch((
			alias(time(), "foo"),
			alias(2*time(), "bar"),
		), "__name__", "f.+")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("bar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`label_graphite_group()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(label_graphite_group((
			alias(1, "foo.bar.baz"),
			alias(2, "abc"),
			label_set(alias(3, "a.xx.zz.asd"), "qwe", "rty"),
	        ), 1, 3))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("bar.")
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte(".")
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3, 3, 3, 3, 3, 3},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.MetricGroup = []byte("xx.asd")
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("qwe"),
			Value: []byte("rty"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`limit_offset`, func(t *testing.T) {
		t.Parallel()
		q := `limit_offset(1, 1, sort_by_label((
			label_set(time()*1, "foo", "y"),
			label_set(time()*2, "foo", "a"),
			label_set(time()*3, "foo", "x"),
		), "foo"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3000, 3600, 4200, 4800, 5400, 6000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("x"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`limit_offset(too-big-offset)`, func(t *testing.T) {
		t.Parallel()
		q := `limit_offset(1, 10, sort_by_label((
			label_set(time()*1, "foo", "y"),
			label_set(time()*2, "foo", "a"),
			label_set(time()*3, "foo", "x"),
		), "foo"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`limit_offset NaN`, func(t *testing.T) {
		t.Parallel()
		// q returns 3 time series, where foo=3 contains only NaN values
		// limit_offset suppose to apply offset for non-NaN series only
		q := `limit_offset(1, 1, sort_by_label_desc((
			label_set(time()*1, "foo", "1"),
			label_set(time()*2, "foo", "2"),
			label_set(time()*3, "foo", "3"),
		) < 3000, "foo"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("1"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(label_graphite_group)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(sum by (__name__) (
			label_graphite_group((
				alias(1, "foo.bar.baz"),
				alias(2, "x.y.z"),
				alias(3, "qe.bar.qqq"),
			), 1)
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("y")
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4, 4, 4, 4, 4, 4},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("bar")
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`two_timeseries`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(time() or label_set(2, "xx", "foo"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("xx"),
			Value: []byte("foo"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`sgn(time()-1400)`, func(t *testing.T) {
		t.Parallel()
		q := `sgn(time()-1400)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-1, -1, 0, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`round(time()/1e3)`, func(t *testing.T) {
		t.Parallel()
		q := `round(time()/1e3)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`round(time()/1e3, 0.5)`, func(t *testing.T) {
		t.Parallel()
		q := `round(time()/1e3, 0.5)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1.5, 1.5, 2, 2},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`round(-time()/1e3, 1)`, func(t *testing.T) {
		t.Parallel()
		q := `round(-time()/1e3, 0.5)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-1, -1, -1.5, -1.5, -2, -2},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`scalar(multi-timeseries)`, func(t *testing.T) {
		t.Parallel()
		q := `scalar(1 or label_set(2, "xx", "foo"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`sort()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(2 or label_set(1, "xx", "foo"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("xx"),
			Value: []byte("foo"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`sort_desc()`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(1 or label_set(2, "xx", "foo"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("xx"),
			Value: []byte("foo"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`sort_by_label()`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label((
			alias(1, "foo"),
			alias(2, "bar"),
		), "__name__")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("bar")
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("foo")
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`sort_by_label_desc()`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label_desc((
			alias(1, "foo"),
			alias(2, "bar"),
		), "__name__")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("foo")
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("bar")
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`sort_by_label(multiple_labels)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label((
			label_set(1, "x", "b", "y", "aa"),
			label_set(2, "x", "a", "y", "aa"),
		), "y", "x")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("a"),
			},
			{
				Key:   []byte("y"),
				Value: []byte("aa"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("b"),
			},
			{
				Key:   []byte("y"),
				Value: []byte("aa"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`scalar < time()`, func(t *testing.T) {
		t.Parallel()
		q := `123 < time()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`time() > scalar`, func(t *testing.T) {
		t.Parallel()
		q := `time() > 1234`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`time() >bool scalar`, func(t *testing.T) {
		t.Parallel()
		q := `time() >bool 1234`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`nan >bool scalar1`, func(t *testing.T) {
		t.Parallel()
		q := `(time() > 1234) >bool 1450`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 0, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`nan!=bool scalar`, func(t *testing.T) {
		t.Parallel()
		q := `(time() > 1234) !=bool 1400`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 0, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`scalar!=bool nan`, func(t *testing.T) {
		t.Parallel()
		q := `1400 !=bool (time() > 1234)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 0, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`scalar > time()`, func(t *testing.T) {
		t.Parallel()
		q := `123 > time()`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`time() < scalar`, func(t *testing.T) {
		t.Parallel()
		q := `time() < 123`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`scalar1 < time() < scalar2`, func(t *testing.T) {
		t.Parallel()
		q := `1300 < time() < 1700`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1400, 1600, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`a cmp scalar (leave MetricGroup)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc((
			label_set(time(), "__name__", "foo", "a", "x"),
			label_set(time()+200, "__name__", "bar", "a", "x"),
		) > 1300)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1400, 1600, 1800, 2000, 2200},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("bar")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("a"),
			Value: []byte("x"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("foo")
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("a"),
			Value: []byte("x"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`a cmp bool scalar (drop MetricGroup)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc((
			label_set(time(), "__name__", "foo", "a", "x"),
			label_set(time()+200, "__name__", "bar", "a", "y"),
		) >= bool 1200)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("a"),
			Value: []byte("y"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("a"),
			Value: []byte("x"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`1 > 2`, func(t *testing.T) {
		t.Parallel()
		q := `1 > 2`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`vector(1) == bool time()`, func(t *testing.T) {
		t.Parallel()
		q := `vector(1) == bool time()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`vector(1) == time()`, func(t *testing.T) {
		t.Parallel()
		q := `vector(1) == time()`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`compare_to_nan_right`, func(t *testing.T) {
		t.Parallel()
		q := `1 != nan`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`compare_to_nan_left`, func(t *testing.T) {
		t.Parallel()
		q := `nan != 1`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`-1 < 2`, func(t *testing.T) {
		t.Parallel()
		q := `-1 < 2`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-1, -1, -1, -1, -1, -1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`time() > 2`, func(t *testing.T) {
		t.Parallel()
		q := `time() > 2`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`time() >= bool 2`, func(t *testing.T) {
		t.Parallel()
		q := `time() >= bool 2`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`1 and (0 > 1)`, func(t *testing.T) {
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6637
		t.Parallel()
		q := `1 and (0 > 1)`
		f(q, nil)
	})
	t.Run(`time() and 2`, func(t *testing.T) {
		t.Parallel()
		q := `time() and 2`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`time() and time() > 1300`, func(t *testing.T) {
		t.Parallel()
		q := `time() and time() > 1300`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`time() unless 2`, func(t *testing.T) {
		t.Parallel()
		q := `time() unless 2`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`time() unless time() > 1500`, func(t *testing.T) {
		t.Parallel()
		q := `time() unless time() > 1500`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`series or series`, func(t *testing.T) {
		t.Parallel()
		q := `(
			label_set(time(), "x", "foo"),
			label_set(time()+1, "x", "bar"),
		) or (
			label_set(time()+2, "x", "foo"),
			label_set(time()+3, "x", "baz"),
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1001, 1201, 1401, 1601, 1801, 2001},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("bar"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("foo"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1003, 1203, 1403, 1603, 1803, 2003},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("baz"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`scalar or scalar`, func(t *testing.T) {
		t.Parallel()
		q := `time() > 1400 or 123`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`timseries-with-tags unless 2`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(time(), "foo", "bar") unless 2`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`scalar default scalar`, func(t *testing.T) {
		t.Parallel()
		q := `time() > 1400 default 123`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`scalar default scalar_from_vector`, func(t *testing.T) {
		t.Parallel()
		q := `time() > 1400 default scalar(label_set(123, "foo", "bar"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`scalar default vector1`, func(t *testing.T) {
		t.Parallel()
		q := `time() > 1400 default label_set(123, "foo", "bar")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`scalar default vector2`, func(t *testing.T) {
		t.Parallel()
		q := `time() > 1400 default (
			label_set(123, "foo", "bar"),
			label_set(456, "__name__", "xxx"),
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{456, 456, 456, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`scalar default NaN`, func(t *testing.T) {
		t.Parallel()
		q := `time() > 1400 default (time() < -100)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`vector default scalar`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(union(
			label_set(time() > 1400, "__name__", "x", "foo", "bar"),
			label_set(time() < 1700, "__name__", "y", "foo", "baz")) default 123)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("x")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 123, 123},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("y")
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("baz"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector / scalar`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc((label_set(time(), "foo", "bar") or label_set(10, "foo", "qwert")) / 2)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{500, 600, 700, 800, 900, 1000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{5, 5, 5, 5, 5, 5},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("qwert"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector / scalar keep_metric_names`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(((label_set(time(), "foo", "bar", "__name__", "q1") or label_set(10, "foo", "qwert", "__name__", "q2")) / 2) keep_metric_names)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{500, 600, 700, 800, 900, 1000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("q1")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{5, 5, 5, 5, 5, 5},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("q2")
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("qwert"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector * scalar`, func(t *testing.T) {
		t.Parallel()
		q := `sum(time()) * 2`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`scalar * vector`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(2 * (label_set(time(), "foo", "bar") or label_set(10, "foo", "qwert")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{20, 20, 20, 20, 20, 20},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("qwert"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`scalar * vector keep_metric_names`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(2 * (label_set(time(), "foo", "bar", "__name__", "q1"), label_set(10, "foo", "qwert", "__name__", "q2")) keep_metric_names)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("q1")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{20, 20, 20, 20, 20, 20},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("q2")
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("qwert"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`scalar * on() group_right vector`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(2 * on() group_right() (label_set(time(), "foo", "bar") or label_set(10, "foo", "qwert")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{20, 20, 20, 20, 20, 20},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("qwert"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`scalar * on() group_right vector keep_metric_names`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(2 * on() group_right() (label_set(time(), "foo", "bar", "__name__", "q1"), label_set(10, "foo", "qwert", "__name__", "q2")) keep_metric_names)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("q1")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{20, 20, 20, 20, 20, 20},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("q2")
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("qwert"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`scalar * ignoring(foo) group_right vector`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(label_set(2, "a", "2") * ignoring(foo,a) group_right(a) (label_set(time(), "foo", "bar", "a", "1"), label_set(10, "foo", "qwert")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("a"),
				Value: []byte("2"),
			},
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{20, 20, 20, 20, 20, 20},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("a"),
				Value: []byte("2"),
			},
			{
				Key:   []byte("foo"),
				Value: []byte("qwert"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`scalar * ignoring(a) vector`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(label_set(2, "foo", "bar") * ignoring(a) (label_set(time(), "foo", "bar") or label_set(10, "foo", "qwert")))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`scalar * on(foo) vector`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(label_set(2, "foo", "bar", "aa", "bb") * on(foo) (label_set(time(), "foo", "bar", "xx", "yy") or label_set(10, "foo", "qwert")))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`vector * on(foo) scalar`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc((label_set(time(), "foo", "bar", "xx", "yy"), label_set(10, "foo", "qwert")) * on(foo) label_set(2, "foo","bar","aa","bb"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`vector * on(foo) scalar keep_metric_names`, func(t *testing.T) {
		t.Parallel()
		q := `((
		          label_set(time(), "foo", "bar", "xx", "yy", "__name__", "q1"),
			  label_set(10, "foo", "qwert", "__name__", "q2")
		      ) * on(foo) label_set(2, "foo","bar","aa","bb", "__name__", "q2")) keep_metric_names`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("q1")
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`vector * on(foo) group_left(additional_tag) duplicate_timeseries_differ_by_additional_tag`, func(t *testing.T) {
		t.Parallel()
		q := `sort(label_set(time()/10, "foo", "bar", "xx", "yy", "__name__", "qwert") + on(foo) group_left(op) (
			label_set(time() < 1400, "foo", "bar", "op", "le"),
			label_set(time() >= 1400, "foo", "bar", "op", "ge"),
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1320, nan, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("op"),
				Value: []byte("le"),
			},
			{
				Key:   []byte("xx"),
				Value: []byte("yy"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1540, 1760, 1980, 2200},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("op"),
				Value: []byte("ge"),
			},
			{
				Key:   []byte("xx"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector * on(foo) duplicate_nonoverlapping_timeseries`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(time()/10, "foo", "bar", "xx", "yy", "__name__", "qwert") + on(foo) (
			label_set(time() < 1400, "foo", "bar", "op", "le"),
			label_set(time() >= 1400, "foo", "bar", "op", "ge"),
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1320, 1540, 1760, 1980, 2200},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
		}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`vector * on(foo) group_left() duplicate_nonoverlapping_timeseries`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(time()/10, "foo", "bar", "xx", "yy", "__name__", "qwert") + on(foo) group_left() (
			label_set(time() < 1400, "foo", "bar", "op", "le"),
			label_set(time() >= 1400, "foo", "bar", "op", "ge"),
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1320, 1540, 1760, 1980, 2200},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("xx"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`vector * on(foo) group_left(__name__)`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(time()/10, "foo", "bar", "xx", "yy", "__name__", "qwert") + on(foo) group_left(__name__)
			label_set(time(), "foo", "bar", "__name__", "aaa")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1320, 1540, 1760, 1980, 2200},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("aaa")
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("xx"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`vector * on(foo) group_right()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(label_set(time()/10, "foo", "bar", "xx", "yy", "__name__", "qwert") + on(foo) group_right(xx) (
			label_set(time(), "foo", "bar", "__name__", "aaa"),
			label_set(time()+3, "foo", "bar", "__name__", "yyy","ppp", "123"),
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1320, 1540, 1760, 1980, 2200},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("xx"),
				Value: []byte("yy"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1103, 1323, 1543, 1763, 1983, 2203},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("ppp"),
				Value: []byte("123"),
			},
			{
				Key:   []byte("xx"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector * on() group_left scalar`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc((label_set(time(), "foo", "bar") or label_set(10, "foo", "qwert")) * on() group_left 2)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{20, 20, 20, 20, 20, 20},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("qwert"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector + vector matching`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(
			(label_set(time(), "t1", "v1") or label_set(10, "t2", "v2"))
			+
			(label_set(100, "t1", "v1") or label_set(time(), "t2", "v2"))
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1300, 1500, 1700, 1900, 2100},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("t1"),
			Value: []byte("v1"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1010, 1210, 1410, 1610, 1810, 2010},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("t2"),
			Value: []byte("v2"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector + vector partial matching`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(
			(label_set(time(), "t1", "v1") or label_set(10, "t2", "v2"))
			+
			(label_set(100, "t1", "v1") or label_set(time(), "t2", "v3"))
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1300, 1500, 1700, 1900, 2100},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("t1"),
			Value: []byte("v1"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`vector + vector partial matching keep_metric_names`, func(t *testing.T) {
		t.Parallel()
		q := `(
		  (label_set(time(), "t1", "v1", "__name__", "q1") or label_set(10, "t2", "v2", "__name__", "q2"))
		    +
		  (label_set(100, "t1", "v1", "__name__", "q1") or label_set(time(), "t2", "v3"))
		) keep_metric_names`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1300, 1500, 1700, 1900, 2100},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("q1")
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("t1"),
			Value: []byte("v1"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`vector + vector no matching`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(
			(label_set(time(), "t2", "v1") or label_set(10, "t2", "v2"))
			+
			(label_set(100, "t1", "v1") or label_set(time(), "t2", "v3"))
		)`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`vector + vector on matching`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(
			(label_set(time(), "t1", "v123", "t2", "v3") or label_set(10, "t2", "v2"))
			+ on (foo, t2)
			(label_set(100, "t1", "v1") or label_set(time(), "t2", "v3"))
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("t2"),
				Value: []byte("v3"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`vector + vector on group_left matching`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(
			(label_set(time(), "t1", "v123", "t2", "v3"), label_set(10, "t2", "v3", "xxx", "yy"))
			+ on (foo, t2) group_left (t1, noxxx)
			(label_set(100, "t1", "v1"), label_set(time(), "t2", "v3", "noxxx", "aa"))
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("noxxx"),
				Value: []byte("aa"),
			},
			{
				Key:   []byte("t2"),
				Value: []byte("v3"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1010, 1210, 1410, 1610, 1810, 2010},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("noxxx"),
				Value: []byte("aa"),
			},
			{
				Key:   []byte("t2"),
				Value: []byte("v3"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector + vector on group_left(*)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(
			(label_set(time(), "t1", "v123", "t2", "v3"), label_set(10, "t2", "v3", "xxx", "yy"))
			+ on (foo, t2) group_left (*)
			(label_set(100, "t1", "v1"), label_set(time(), "t2", "v3", "noxxx", "aa"))
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("noxxx"),
				Value: []byte("aa"),
			},
			{
				Key:   []byte("t1"),
				Value: []byte("v123"),
			},
			{
				Key:   []byte("t2"),
				Value: []byte("v3"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1010, 1210, 1410, 1610, 1810, 2010},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("noxxx"),
				Value: []byte("aa"),
			},
			{
				Key:   []byte("t2"),
				Value: []byte("v3"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector + vector on group_left(*) prefix`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(
			(label_set(time(), "t1", "v123", "t2", "v3"), label_set(10, "t2", "v3", "xxx", "yy"))
			+ on (foo, t2) group_left (*) prefix "abc_"
			(label_set(100, "t1", "v1"), label_set(time(), "t2", "v3", "noxxx", "aa"))
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("abc_noxxx"),
				Value: []byte("aa"),
			},
			{
				Key:   []byte("t1"),
				Value: []byte("v123"),
			},
			{
				Key:   []byte("t2"),
				Value: []byte("v3"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1010, 1210, 1410, 1610, 1810, 2010},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("abc_noxxx"),
				Value: []byte("aa"),
			},
			{
				Key:   []byte("t2"),
				Value: []byte("v3"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector + vector on group_left (__name__)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(
			(union(label_set(time(), "t2", "v3", "__name__", "vv3", "x", "y"), label_set(10, "t2", "v3", "__name__", "yy")))
			+ on (t2, dfdf) group_left (__name__, xxx)
			(label_set(100, "t1", "v1") or label_set(time(), "t2", "v3", "__name__", "abc"))
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("abc")
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("t2"),
				Value: []byte("v3"),
			},
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1010, 1210, 1410, 1610, 1810, 2010},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("abc")
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("t2"),
			Value: []byte("v3"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`vector + vector ignoring matching`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(
			(label_set(time(), "t1", "v123", "t2", "v3") or label_set(10, "t2", "v2"))
			+ ignoring (foo, t1, bar)
			(label_set(100, "t1", "v1") or label_set(time(), "t2", "v3"))
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("t2"),
				Value: []byte("v3"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`vector + vector ignoring group_right matching`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(
			(label_set(time(), "t1", "v123", "t2", "v3") or label_set(10, "t2", "v321", "t1", "v123", "t32", "v32"))
			+ ignoring (foo, t2) group_right ()
			(label_set(100, "t1", "v123") or label_set(time(), "t1", "v123", "t2", "v3"))
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2400, 2800, 3200, 3600, 4000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("t1"),
				Value: []byte("v123"),
			},
			{
				Key:   []byte("t2"),
				Value: []byte("v3"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1100, 1300, 1500, 1700, 1900, 2100},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("t1"),
			Value: []byte("v123"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(scalar)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6, time())`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(scalar)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(123, time())`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(single-value-no-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6, label_set(100, "foo", "bar"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(single-value-no-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(123, label_set(100, "foo", "bar"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(single-value-invalid-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6, label_set(100, "le", "foobar"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(single-value-invalid-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(50, label_set(100, "le", "foobar"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(single-value-inf-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6, label_set(100, "le", "+Inf"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(zero-value-inf-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6, (
			label_set(100, "le", "+Inf"),
			label_set(0, "le", "42"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{42, 42, 42, 42, 42, 42},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(single-value-valid-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6, label_set(100, "le", "200"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{120, 120, 120, 120, 120, 120},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`stdvar_over_time()`, func(t *testing.T) {
		t.Parallel()
		q := `round(stdvar_over_time(rand(0)[200s:5s]), 0.001)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.082, 0.088, 0.092, 0.075, 0.101, 0.08},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_stdvar()`, func(t *testing.T) {
		t.Parallel()
		q := `round(histogram_stdvar(histogram_over_time(rand(0)[200s:5s])), 0.001)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.079, 0.089, 0.089, 0.071, 0.1, 0.082},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`mad_over_time()`, func(t *testing.T) {
		t.Parallel()
		q := `round(mad_over_time(rand(0)[200s:5s]), 0.001)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.243, 0.274, 0.256, 0.185, 0.266, 0.256},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`stddev_over_time()`, func(t *testing.T) {
		t.Parallel()
		q := `round(stddev_over_time(rand(0)[200s:5s]), 0.001)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.286, 0.297, 0.303, 0.274, 0.318, 0.283},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_stddev()`, func(t *testing.T) {
		t.Parallel()
		q := `round(histogram_stddev(histogram_over_time(rand(0)[200s:5s])), 0.001)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.281, 0.299, 0.298, 0.267, 0.316, 0.286},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`avg_over_time()`, func(t *testing.T) {
		t.Parallel()
		q := `round(avg_over_time(rand(0)[200s:5s]), 0.001)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.521, 0.518, 0.509, 0.544, 0.511, 0.504},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_avg()`, func(t *testing.T) {
		t.Parallel()
		q := `round(histogram_avg(histogram_over_time(rand(0)[200s:5s])), 0.001)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.519, 0.521, 0.503, 0.543, 0.511, 0.506},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(single-value-valid-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(80, label_set(100, "le", "200"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.4, 0.4, 0.4, 0.4, 0.4, 0.4},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(single-value-valid-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(200, label_set(100, "le", "200"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(single-value-valid-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(300, label_set(100, "le", "200"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(single-value-valid-le, boundsLabel)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(histogram_quantile(0.6, label_set(100, "le", "200"), "foobar"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foobar"),
			Value: []byte("lower"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{120, 120, 120, 120, 120, 120},
			Timestamps: timestampsExpected,
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foobar"),
			Value: []byte("upper"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(single-value-valid-le, boundsLabel)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(histogram_share(120, label_set(100, "le", "200"), "foobar"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foobar"),
			Value: []byte("lower"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.6, 0.6, 0.6, 0.6, 0.6, 0.6},
			Timestamps: timestampsExpected,
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foobar"),
			Value: []byte("upper"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(single-value-valid-le-max-phi)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(1, (
			label_set(100, "le", "200"),
			label_set(0, "le", "55"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(single-value-valid-le-max-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(200, (
			label_set(100, "le", "200"),
			label_set(0, "le", "55"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(single-value-valid-le-min-phi)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0, (
			label_set(100, "le", "200"),
			label_set(0, "le", "55"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{55, 55, 55, 55, 55, 55},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(single-value-valid-le-min-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(0, (
			label_set(100, "le", "200"),
			label_set(0, "le", "55"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(single-value-valid-le-low-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(55, (
			label_set(100, "le", "200"),
			label_set(0, "le", "55"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(single-value-valid-le-mid-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(105, (
			label_set(100, "le", "200"),
			label_set(0, "le", "55"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.3448275862068966, 0.3448275862068966, 0.3448275862068966, 0.3448275862068966, 0.3448275862068966, 0.3448275862068966},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(single-value-valid-le-min-phi-no-zero-bucket)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0, label_set(100, "le", "200"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(scalar-phi)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(time() / 2 / 1e3, label_set(100, "le", "200"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 120, 140, 160, 180, 200},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(scalar-phi)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(time() / 8, label_set(100, "le", "200"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.625, 0.75, 0.875, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(duplicate-le)`, func(t *testing.T) {
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3225
		t.Parallel()
		q := `round(sort(histogram_quantile(0.6,
			label_set(90, "foo", "bar", "le", "5")
			or label_set(100, "foo", "bar", "le", "5.0")
			or label_set(200, "foo", "bar", "le", "6.0")
			or label_set(300, "foo", "bar", "le", "+Inf")
		)), 0.1)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4.7, 4.7, 4.7, 4.7, 4.7, 4.7},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(valid)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(histogram_quantile(0.6,
			label_set(90, "foo", "bar", "le", "10")
			or label_set(100, "foo", "bar", "le", "30")
			or label_set(300, "foo", "bar", "le", "+Inf")
			or label_set(200, "tag", "xx", "le", "10")
			or label_set(300, "tag", "xx", "le", "30")
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{9, 9, 9, 9, 9, 9},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("tag"),
			Value: []byte("xx"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{30, 30, 30, 30, 30, 30},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(valid)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(histogram_share(25,
			label_set(90, "foo", "bar", "le", "10")
			or label_set(100, "foo", "bar", "le", "30")
			or label_set(300, "foo", "bar", "le", "+Inf")
			or label_set(200, "tag", "xx", "le", "10")
			or label_set(300, "tag", "xx", "le", "30")
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.325, 0.325, 0.325, 0.325, 0.325, 0.325},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.9166666666666666, 0.9166666666666666, 0.9166666666666666, 0.9166666666666666, 0.9166666666666666, 0.9166666666666666},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("tag"),
			Value: []byte("xx"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(negative-bucket-count)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6,
			label_set(90, "foo", "bar", "le", "10")
			or label_set(-100, "foo", "bar", "le", "30")
			or label_set(300, "foo", "bar", "le", "+Inf")
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{30, 30, 30, 30, 30, 30},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(nan-bucket-count-some)`, func(t *testing.T) {
		t.Parallel()
		q := `round(histogram_quantile(0.6,
			label_set(90, "foo", "bar", "le", "10")
			or label_set(NaN, "foo", "bar", "le", "30")
			or label_set(300, "foo", "bar", "le", "+Inf")
		),0.01)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{30, 30, 30, 30, 30, 30},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(normal-bucket-count)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.2,
			label_set(0, "foo", "bar", "le", "10")
			or label_set(100, "foo", "bar", "le", "30")
			or label_set(300, "foo", "bar", "le", "+Inf")
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{22, 22, 22, 22, 22, 22},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantiles()`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label(histogram_quantiles("phi", 0.2, 0.3,
			label_set(0, "foo", "bar", "le", "10")
			or label_set(100, "foo", "bar", "le", "30")
			or label_set(300, "foo", "bar", "le", "+Inf")
		), "phi")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{22, 22, 22, 22, 22, 22},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("phi"),
				Value: []byte("0.2"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{28, 28, 28, 28, 28, 28},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("phi"),
				Value: []byte("0.3"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(normal-bucket-count)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_share(35,
			label_set(0, "foo", "bar", "le", "10")
			or label_set(100, "foo", "bar", "le", "30")
			or label_set(300, "foo", "bar", "le", "+Inf")
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.3333333333333333, 0.3333333333333333, 0.3333333333333333, 0.3333333333333333, 0.3333333333333333, 0.3333333333333333},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(normal-bucket-count, boundsLabel)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(histogram_quantile(0.2,
			label_set(0, "foo", "bar", "le", "10")
			or label_set(100, "foo", "bar", "le", "30")
			or label_set(300, "foo", "bar", "le", "+Inf"),
			"xxx"
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("lower"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{22, 22, 22, 22, 22, 22},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{30, 30, 30, 30, 30, 30},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("upper"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`histogram_share(normal-bucket-count, boundsLabel)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(histogram_share(22,
			label_set(0, "foo", "bar", "le", "10")
			or label_set(100, "foo", "bar", "le", "30")
			or label_set(300, "foo", "bar", "le", "+Inf"),
			"xxx"
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("lower"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.3333333333333333, 0.3333333333333333, 0.3333333333333333, 0.3333333333333333, 0.3333333333333333, 0.3333333333333333},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("upper"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(zero-bucket-count)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6,
			label_set(0, "foo", "bar", "le", "10")
			or label_set(0, "foo", "bar", "le", "30")
			or label_set(0, "foo", "bar", "le", "+Inf")
		)`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(nan-bucket-count-all)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6,
			label_set(nan, "foo", "bar", "le", "10")
			or label_set(nan, "foo", "bar", "le", "30")
			or label_set(nan, "foo", "bar", "le", "+Inf")
		)`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`buckets_limit(zero)`, func(t *testing.T) {
		t.Parallel()
		q := `buckets_limit(0, (
			alias(label_set(100, "le", "inf", "x", "y"), "metric"),
			alias(label_set(50, "le", "120", "x", "y"), "metric"),
		))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`buckets_limit(unused)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(buckets_limit(5, (
			alias(label_set(100, "le", "inf", "x", "y"), "metric"),
			alias(label_set(50, "le", "120", "x", "y"), "metric"),
		)))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{50, 50, 50, 50, 50, 50},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("metric")
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("120"),
			},
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 100, 100, 100, 100, 100},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("metric")
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("inf"),
			},
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`buckets_limit(used)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(buckets_limit(2, (
			alias(label_set(100, "le", "inf", "x", "y"), "metric"),
			alias(label_set(98, "le", "300", "x", "y"), "metric"),
			alias(label_set(52, "le", "200", "x", "y"), "metric"),
			alias(label_set(50, "le", "120", "x", "y"), "metric"),
			alias(label_set(20, "le", "70", "x", "y"), "metric"),
			alias(label_set(10, "le", "30", "x", "y"), "metric"),
			alias(label_set(9, "le", "10", "x", "y"), "metric"),
		)))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{9, 9, 9, 9, 9, 9},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("metric")
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("10"),
			},
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{98, 98, 98, 98, 98, 98},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("metric")
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("300"),
			},
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 100, 100, 100, 100, 100},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.MetricGroup = []byte("metric")
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("inf"),
			},
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`prometheus_buckets(missing-vmrange)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(prometheus_buckets((
			alias(label_set(time()/20, "foo", "bar", "le", "0.2"), "xyz"),
			alias(label_set(time()/100, "foo", "bar", "vmrange", "foobar"), "xxx"),
			alias(label_set(time()/100, "foo", "bar", "vmrange", "30...foobar"), "xxx"),
			alias(label_set(time()/100, "foo", "bar", "vmrange", "30...40"), "xxx"),
			alias(label_set(time()/80, "foo", "bar", "vmrange", "0...900", "le", "54"), "yyy"),
			alias(label_set(time()/40, "foo", "bar", "vmrange", "900...+Inf", "le", "2343"), "yyy"),
		)))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("xxx")
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("30"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 12, 14, 16, 18, 20},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("xxx")
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("40"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 12, 14, 16, 18, 20},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.MetricGroup = []byte("xxx")
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("+Inf"),
			},
		}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{12.5, 15, 17.5, 20, 22.5, 25},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.MetricGroup = []byte("yyy")
		r4.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("900"),
			},
		}
		r5 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{37.5, 45, 52.5, 60, 67.5, 75},
			Timestamps: timestampsExpected,
		}
		r5.MetricName.MetricGroup = []byte("yyy")
		r5.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("+Inf"),
			},
		}
		r6 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{50, 60, 70, 80, 90, 100},
			Timestamps: timestampsExpected,
		}
		r6.MetricName.MetricGroup = []byte("xyz")
		r6.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("0.2"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3, r4, r5, r6}
		f(q, resultExpected)
	})
	t.Run(`prometheus_buckets(zero-vmrange-value)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(prometheus_buckets(label_set(0, "vmrange", "0...0")))`
		resultsExpected := []netstorage.Result{}
		f(q, resultsExpected)
	})
	t.Run(`prometheus_buckets(valid)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(prometheus_buckets((
			alias(label_set(90, "foo", "bar", "vmrange", "0...0"), "xxx"),
			alias(label_set(time()/20, "foo", "bar", "vmrange", "0...0.2"), "xxx"),
			alias(label_set(time()/100, "foo", "bar", "vmrange", "0.2...40"), "xxx"),
			alias(label_set(time()/10, "foo", "bar", "vmrange", "40...Inf"), "xxx"),
		)))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{90, 90, 90, 90, 90, 90},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("xxx")
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("0"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{140, 150, 160, 170, 180, 190},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("xxx")
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("0.2"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{150, 162, 174, 186, 198, 210},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.MetricGroup = []byte("xxx")
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("40"),
			},
		}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{250, 282, 314, 346, 378, 410},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.MetricGroup = []byte("xxx")
		r4.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("Inf"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3, r4}
		f(q, resultExpected)
	})
	t.Run(`prometheus_buckets(overlapped ranges)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(prometheus_buckets((
			alias(label_set(90, "foo", "bar", "vmrange", "0...0"), "xxx"),
			alias(label_set(time()/20, "foo", "bar", "vmrange", "0...0.2"), "xxx"),
			alias(label_set(time()/20, "foo", "bar", "vmrange", "0.2...0.25"), "xxx"),
			alias(label_set(time()/20, "foo", "bar", "vmrange", "0...0.26"), "xxx"),
			alias(label_set(time()/100, "foo", "bar", "vmrange", "0.2...40"), "xxx"),
			alias(label_set(time()/10, "foo", "bar", "vmrange", "40...Inf"), "xxx"),
		)))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{90, 90, 90, 90, 90, 90},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("xxx")
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("0"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{140, 150, 160, 170, 180, 190},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("xxx")
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("0.2"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{190, 210, 230, 250, 270, 290},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.MetricGroup = []byte("xxx")
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("0.25"),
			},
		}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{240, 270, 300, 330, 360, 390},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.MetricGroup = []byte("xxx")
		r4.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("0.26"),
			},
		}
		r5 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{250, 282, 314, 346, 378, 410},
			Timestamps: timestampsExpected,
		}
		r5.MetricName.MetricGroup = []byte("xxx")
		r5.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("40"),
			},
		}
		r6 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{350, 402, 454, 506, 558, 610},
			Timestamps: timestampsExpected,
		}
		r6.MetricName.MetricGroup = []byte("xxx")
		r6.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("Inf"),
			},
		}

		resultExpected := []netstorage.Result{r1, r2, r3, r4, r5, r6}
		f(q, resultExpected)
	})
	t.Run(`prometheus_buckets(overlapped ranges at the end)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(prometheus_buckets((
			alias(label_set(90, "foo", "bar", "vmrange", "0...0"), "xxx"),
			alias(label_set(time()/20, "foo", "bar", "vmrange", "0...0.2"), "xxx"),
			alias(label_set(time()/20, "foo", "bar", "vmrange", "0.2...0.25"), "xxx"),
			alias(label_set(time()/20, "foo", "bar", "vmrange", "0...0.25"), "xxx"),
			alias(label_set(time()/100, "foo", "bar", "vmrange", "0.2...40"), "xxx"),
			alias(label_set(time()/10, "foo", "bar", "vmrange", "40...Inf"), "xxx"),
		)))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{90, 90, 90, 90, 90, 90},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("xxx")
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("0"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{140, 150, 160, 170, 180, 190},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("xxx")
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("0.2"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{190, 210, 230, 250, 270, 290},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.MetricGroup = []byte("xxx")
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("0.25"),
			},
		}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 222, 244, 266, 288, 310},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.MetricGroup = []byte("xxx")
		r4.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("40"),
			},
		}
		r5 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{300, 342, 384, 426, 468, 510},
			Timestamps: timestampsExpected,
		}
		r5.MetricName.MetricGroup = []byte("xxx")
		r5.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("le"),
				Value: []byte("Inf"),
			},
		}

		resultExpected := []netstorage.Result{r1, r2, r3, r4, r5}
		f(q, resultExpected)
	})
	t.Run(`median_over_time()`, func(t *testing.T) {
		t.Parallel()
		q := `median_over_time({})`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`sum(scalar)`, func(t *testing.T) {
		t.Parallel()
		q := `sum(123)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 123, 123, 123},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(multi-args)`, func(t *testing.T) {
		t.Parallel()
		q := `sum(1, 2, 3)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6, 6, 6, 6, 6, 6},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(union-scalars)`, func(t *testing.T) {
		t.Parallel()
		q := `sum((1, 2, 3))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6, 6, 6, 6, 6, 6},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(union-vectors)`, func(t *testing.T) {
		t.Parallel()
		q := `sum((
			alias(1, "foo"),
			alias(2, "foo"),
			alias(3, "foo"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(scalar) by ()`, func(t *testing.T) {
		t.Parallel()
		q := `sum(123) by ()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 123, 123, 123},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(scalar) without ()`, func(t *testing.T) {
		t.Parallel()
		q := `sum(123) without ()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 123, 123, 123},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`mode()`, func(t *testing.T) {
		t.Parallel()
		q := `mode((
			alias(3, "m1"),
			alias(2, "m2"),
			alias(3, "m3"),
			alias(4, "m4"),
			alias(3, "m5"),
			alias(2, "m6"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3, 3, 3, 3, 3, 3},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`share()`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label(round(share((
			label_set(time()/100+10, "k", "v1"),
			label_set(time()/200+5, "k", "v2"),
			label_set(time()/110-10, "k", "v3"),
			label_set(time()/90-5, "k", "v4"),
		)), 0.001), "k")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.554, 0.521, 0.487, 0.462, 0.442, 0.426},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("k"),
			Value: []byte("v1"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.277, 0.26, 0.243, 0.231, 0.221, 0.213},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("k"),
			Value: []byte("v2"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 0.022, 0.055, 0.081, 0.1, 0.116},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("k"),
			Value: []byte("v3"),
		}}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.169, 0.197, 0.214, 0.227, 0.237, 0.245},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.Tags = []storage.Tag{{
			Key:   []byte("k"),
			Value: []byte("v4"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3, r4}
		f(q, resultExpected)
	})
	t.Run(`sum(share())`, func(t *testing.T) {
		t.Parallel()
		q := `round(sum(share((
			label_set(time()/100+10, "k", "v1"),
			label_set(time()/200+5, "k", "v2"),
			label_set(time()/110-10, "k", "v3"),
			label_set(time()/90-5, "k", "v4"),
		))), 0.001)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(share() by (k))`, func(t *testing.T) {
		t.Parallel()
		q := `round(sum(share((
			label_set(time()/100+10, "k", "v1"),
			label_set(time()/200+5, "k", "v2", "a", "b"),
			label_set(time()/110-10, "k", "v1", "a", "b"),
			label_set(time()/90-5, "k", "v2"),
		)) by (k)), 0.001)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`zscore()`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label(round(zscore((
			label_set(time()/100+10, "k", "v1"),
			label_set(time()/200+5, "k", "v2"),
			label_set(time()/110-10, "k", "v3"),
			label_set(time()/90-5, "k", "v4"),
		)), 0.001), "k")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1.482, 1.511, 1.535, 1.552, 1.564, 1.57},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("k"),
			Value: []byte("v1"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.159, 0.058, -0.042, -0.141, -0.237, -0.329},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("k"),
			Value: []byte("v2"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-1.285, -1.275, -1.261, -1.242, -1.219, -1.193},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("k"),
			Value: []byte("v3"),
		}}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-0.356, -0.294, -0.232, -0.17, -0.108, -0.048},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.Tags = []storage.Tag{{
			Key:   []byte("k"),
			Value: []byte("v4"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3, r4}
		f(q, resultExpected)
	})
	t.Run(`avg(scalar) without (xx, yy)`, func(t *testing.T) {
		t.Parallel()
		q := `avg without (xx, yy) (123)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 123, 123, 123},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`histogram(scalar)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(histogram(123)+(
			label_set(0, "le", "1.000e+02"),
			label_set(0, "le", "1.136e+02"),
			label_set(0, "le", "1.292e+02"),
			label_set(1, "le", "+Inf"),
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("1.136e+02"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("1.292e+02"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("+Inf"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`histogram(vector)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(histogram((
			label_set(1, "foo", "bar"),
			label_set(1.1, "xx", "yy"),
			alias(1.15, "foobar"),
		))+(
			label_set(0, "le", "8.799e-01"),
			label_set(0, "le", "1.000e+00"),
			label_set(0, "le", "1.292e+00"),
			label_set(1, "le", "+Inf"),
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("8.799e-01"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("1.000e+00"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3, 3, 3, 3, 3, 3},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("1.292e+00"),
			},
		}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4, 4, 4, 4, 4, 4},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("+Inf"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3, r4}
		f(q, resultExpected)
	})
	t.Run(`avg(scalar) wiTHout (xx, yy)`, func(t *testing.T) {
		t.Parallel()
		q := `avg wiTHout (xx, yy) (123)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{123, 123, 123, 123, 123, 123},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(time)`, func(t *testing.T) {
		t.Parallel()
		q := `sum(time()/100)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 12, 14, 16, 18, 20},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`geomean(time)`, func(t *testing.T) {
		t.Parallel()
		q := `geomean(time()/100)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 12, 14, 16, 18, 20},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`geomean_over_time(time)`, func(t *testing.T) {
		t.Parallel()
		q := `round(geomean_over_time(alias(time()/100, "foobar")[3i]), 0.1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{7.8, 9.9, 11.9, 13.9, 15.9, 17.9},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum2(time)`, func(t *testing.T) {
		t.Parallel()
		q := `sum2(time()/100)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 144, 196, 256, 324, 400},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum2_over_time(time)`, func(t *testing.T) {
		t.Parallel()
		q := `sum2_over_time(alias(time()/100, "foobar")[3i])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 308, 440, 596, 776, 980},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_over_time(time)`, func(t *testing.T) {
		t.Parallel()
		q := `range_over_time(alias(time()/100, "foobar")[3i])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4, 4, 4, 4, 4, 4},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(multi-vector)`, func(t *testing.T) {
		t.Parallel()
		q := `sum(label_set(10, "foo", "bar") or label_set(time()/100, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{20, 22, 24, 26, 28, 30},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`geomean(multi-vector)`, func(t *testing.T) {
		t.Parallel()
		q := `round(geomean(label_set(10, "foo", "bar") or label_set(time()/100, "baz", "sss")), 0.1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 11, 11.8, 12.6, 13.4, 14.1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum2(multi-vector)`, func(t *testing.T) {
		t.Parallel()
		q := `sum2(label_set(10, "foo", "bar") or label_set(time()/100, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 244, 296, 356, 424, 500},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sqrt(sum2(multi-vector))`, func(t *testing.T) {
		t.Parallel()
		q := `round(sqrt(sum2(label_set(10, "foo", "bar") or label_set(time()/100, "baz", "sss"))))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{14, 16, 17, 19, 21, 22},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`avg(multi-vector)`, func(t *testing.T) {
		t.Parallel()
		q := `avg(label_set(10, "foo", "bar") or label_set(time()/100, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 11, 12, 13, 14, 15},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`stddev(multi-vector)`, func(t *testing.T) {
		t.Parallel()
		q := `stddev(label_set(10, "foo", "bar") or label_set(time()/100, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 1, 2, 3, 4, 5},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`count(multi-vector)`, func(t *testing.T) {
		t.Parallel()
		q := `count(label_set(time()<1500, "foo", "bar") or label_set(time()<1800, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 1, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(multi-vector) by (known-tag)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(sum(label_set(10, "foo", "bar") or label_set(time()/100, "baz", "sss")) by (foo))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 12, 14, 16, 18, 20},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`sum(multi-vector) by (known-tag) limit 1`, func(t *testing.T) {
		t.Parallel()
		q := `sum(label_set(10, "foo", "bar") or label_set(time()/100, "baz", "sss")) by (foo) limit 1`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(multi-vector) by (known-tags)`, func(t *testing.T) {
		t.Parallel()
		q := `sum(label_set(10, "foo", "bar", "baz", "sss", "x", "y") or label_set(time()/100, "baz", "sss", "foo", "bar")) by (foo, baz, foo)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{20, 22, 24, 26, 28, 30},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("baz"),
				Value: []byte("sss"),
			},
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(multi-vector) by (__name__)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(sum(label_set(10, "__name__", "bar", "baz", "sss", "x", "y") or label_set(time()/100, "baz", "sss", "__name__", "aaa")) by (__name__))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("bar")
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 12, 14, 16, 18, 20},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("aaa")
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`min(multi-vector) by (unknown-tag)`, func(t *testing.T) {
		t.Parallel()
		q := `min(label_set(10, "foo", "bar") or label_set(time()/100/1.5, "baz", "sss")) by (unknowntag)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`max(multi-vector) by (unknown-tag)`, func(t *testing.T) {
		t.Parallel()
		q := `max(label_set(10, "foo", "bar") or label_set(time()/100/1.5, "baz", "sss")) by (unknowntag)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`quantile_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `quantile_over_time(0.9, label_set(round(rand(0), 0.01), "__name__", "foo", "xx", "yy")[200s:5s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.893, 0.892, 0.9510000000000001, 0.8730000000000001, 0.9250000000000002, 0.891},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foo")
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xx"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`equal-list`, func(t *testing.T) {
		t.Parallel()
		q := `time() == (100, 1000, 1400, 600)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, nan, 1400, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`equal-list-reverse`, func(t *testing.T) {
		t.Parallel()
		q := `(100, 1000, 1400, 600) == time()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, nan, 1400, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`not-equal-list`, func(t *testing.T) {
		t.Parallel()
		q := `alias(time(), "foobar") != UNIon(100, 1000, 1400, 600)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1200, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`not-equal-list-reverse`, func(t *testing.T) {
		t.Parallel()
		q := `(100, 1000, 1400, 600) != time()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1200, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`quantiles_over_time(single_sample)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label(
			quantiles_over_time("phi", 0.5, 0.9,
				time()[100s:100s]
			),
			"phi",
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("phi"),
				Value: []byte("0.5"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("phi"),
				Value: []byte("0.9"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`quantiles_over_time(multiple_samples)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label(
			quantiles_over_time("phi", 0.5, 0.9,
				label_set(round(rand(0), 0.01), "__name__", "foo", "xx", "yy")[200s:5s]
			),
			"phi",
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.46499999999999997, 0.57, 0.485, 0.54, 0.555, 0.515},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("foo")
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("phi"),
				Value: []byte("0.5"),
			},
			{
				Key:   []byte("xx"),
				Value: []byte("yy"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.893, 0.892, 0.9510000000000001, 0.8730000000000001, 0.9250000000000002, 0.891},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("foo")
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("phi"),
				Value: []byte("0.9"),
			},
			{
				Key:   []byte("xx"),
				Value: []byte("yy"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`count_values_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label(
			count_values_over_time("foo", round(label_set(rand(0), "x", "y"), 0.4)[200s:5s]),
			"foo",
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4, 8, 7, 6, 10, 9},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("0"),
			},
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{20, 13, 19, 18, 14, 13},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("0.4"),
			},
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{16, 19, 14, 16, 16, 18},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("0.8"),
			},
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`histogram_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label(histogram_over_time(alias(label_set(rand(0)*1.3+1.1, "foo", "bar"), "xxx")[200s:5s]), "vmrange")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 2, 2, 2, nan, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("vmrange"),
				Value: []byte("1.000e+00...1.136e+00"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3, 3, 4, 2, 8, 3},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("vmrange"),
				Value: []byte("1.136e+00...1.292e+00"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{7, 7, 5, 3, 3, 9},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("vmrange"),
				Value: []byte("1.292e+00...1.468e+00"),
			},
		}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{7, 4, 6, 5, 6, 4},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("vmrange"),
				Value: []byte("1.468e+00...1.668e+00"),
			},
		}
		r5 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6, 6, 9, 13, 7, 7},
			Timestamps: timestampsExpected,
		}
		r5.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("vmrange"),
				Value: []byte("1.668e+00...1.896e+00"),
			},
		}
		r6 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{5, 9, 4, 6, 7, 9},
			Timestamps: timestampsExpected,
		}
		r6.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("vmrange"),
				Value: []byte("1.896e+00...2.154e+00"),
			},
		}
		r7 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{11, 9, 10, 9, 9, 7},
			Timestamps: timestampsExpected,
		}
		r7.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("vmrange"),
				Value: []byte("2.154e+00...2.448e+00"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3, r4, r5, r6, r7}
		f(q, resultExpected)
	})
	t.Run(`sum(histogram_over_time) by (vmrange)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label(
			buckets_limit(
				3,
				sum(histogram_over_time(alias(label_set(rand(0)*1.3+1.1, "foo", "bar"), "xxx")[200s:5s])) by (vmrange)
			), "le"
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{40, 40, 40, 40, 40, 40},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("+Inf"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("1.000e+00"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{40, 40, 40, 40, 40, 40},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("le"),
				Value: []byte("2.448e+00"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`sum(histogram_over_time)`, func(t *testing.T) {
		t.Parallel()
		q := `sum(histogram_over_time(alias(label_set(rand(0)*1.3+1.1, "foo", "bar"), "xxx")[200s:5s]))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{40, 40, 40, 40, 40, 40},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(Histogram_OVER_time)`, func(t *testing.T) {
		t.Parallel()
		q := `sum(Histogram_OVER_time(alias(label_set(rand(0)*1.3+1.1, "foo", "bar"), "xxx")[200s:5s]))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{40, 40, 40, 40, 40, 40},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`topk_max(histogram_over_time)`, func(t *testing.T) {
		t.Parallel()
		q := `topk_max(1, histogram_over_time(alias(label_set(rand(0)*1.3+1.1, "foo", "bar"), "xxx")[200s:5s]))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6, 6, 9, 13, 7, 7},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("vmrange"),
				Value: []byte("1.668e+00...1.896e+00"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`duration_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `duration_over_time((time()<1200)[600s:10s], 20s)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{590, 580, 380, 180, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`share_gt_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `share_gt_over_time(rand(0)[200s:10s], 0.7)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.35, 0.3, 0.5, 0.3, 0.3, 0.25},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`share_le_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `share_le_over_time(rand(0)[200s:10s], 0.7)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.65, 0.7, 0.5, 0.7, 0.7, 0.75},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`share_eq_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `share_eq_over_time(round(5*rand(0))[200s:10s], 1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.1, 0.2, 0.25, 0.1, 0.3, 0.3},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`count_gt_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `count_gt_over_time(rand(0)[200s:10s], 0.7)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{7, 6, 10, 6, 6, 5},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`count_le_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `count_le_over_time(rand(0)[200s:10s], 0.7)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{13, 14, 10, 14, 14, 15},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`count_eq_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `count_eq_over_time(round(5*rand(0))[200s:10s], 1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 4, 5, 2, 6, 6},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`count_ne_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `count_ne_over_time(round(5*rand(0))[200s:10s], 1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{18, 16, 15, 18, 14, 14},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum_gt_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `round(sum_gt_over_time(rand(0)[200s:10s], 0.7), 0.1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{5.9, 5.2, 8.5, 5.1, 4.9, 4.5},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum_le_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `round(sum_le_over_time(rand(0)[200s:10s], 0.7), 0.1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4.2, 4.9, 3.2, 5.8, 4.1, 5.3},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum_eq_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `round(sum_eq_over_time(rand(0)[200s:10s], 0.7), 0.1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`increases_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `increases_over_time(rand(0)[200s:10s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{11, 9, 9, 12, 9, 8},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`decreases_over_time`, func(t *testing.T) {
		t.Parallel()
		q := `decreases_over_time(rand(0)[200s:10s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{9, 11, 11, 8, 11, 12},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`limitk(-1)`, func(t *testing.T) {
		t.Parallel()
		q := `limitk(-1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`limitk(1)`, func(t *testing.T) {
		t.Parallel()
		q := `limitk(1, label_set(10, "foo", "bar") or label_set(time()/150, "xbaz", "sss"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`limitk(10)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(limitk(10, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`limitk(inf)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(limitk(inf, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`any()`, func(t *testing.T) {
		t.Parallel()
		q := `any(label_set(10, "__name__", "x", "foo", "bar") or label_set(time()/150, "__name__", "y", "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("x")
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`any(empty-series)`, func(t *testing.T) {
		t.Parallel()
		q := `any(label_set(time()<0, "foo", "bar"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`group() by (test)`, func(t *testing.T) {
		t.Parallel()
		q := `group((
			label_set(5, "__name__", "data", "test", "three samples", "point", "a"),
			label_set(6, "__name__", "data", "test", "three samples", "point", "b"),
			label_set(7, "__name__", "data", "test", "three samples", "point", "c"),
		)) by (test)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = nil
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("test"),
			Value: []byte("three samples"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`group() without (point)`, func(t *testing.T) {
		t.Parallel()
		q := `group((
			label_set(5, "__name__", "data", "test", "three samples", "point", "a"),
			label_set(6, "__name__", "data", "test", "three samples", "point", "b"),
			label_set(7, "__name__", "data", "test", "three samples", "point", "c"),
		)) without (point)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = nil
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("test"),
			Value: []byte("three samples"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`topk(-1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(topk(-1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`topk(1)`, func(t *testing.T) {
		t.Parallel()
		q := `topk(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`topk_min(1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(topk_min(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`bottomk_min(1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(bottomk_min(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`topk_max(1)`, func(t *testing.T) {
		t.Parallel()
		q := `topk_max(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`topk_max(1, remaining_sum)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(topk_max(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"), "remaining_sum=foo"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("remaining_sum"),
				Value: []byte("foo"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`topk_max(2, remaining_sum)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(topk_max(2, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"), "remaining_sum"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`topk_max(3, remaining_sum)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(topk_max(3, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"), "remaining_sum"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`bottomk_max(1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(bottomk_max(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`topk_avg(1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(topk_avg(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`bottomk_avg(1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(bottomk_avg(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`topk_median(1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(topk_median(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`topk_last(1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(topk_last(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`bottomk_median(1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(bottomk_median(1, label_set(10, "foo", "bar") or label_set(time()/15, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`bottomk_last(1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(bottomk_last(1, label_set(10, "foo", "bar") or label_set(time()/15, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`topk(1, nan_timeseries)`, func(t *testing.T) {
		t.Parallel()
		q := `topk(1, label_set(NaN, "foo", "bar") or label_set(time()/150, "baz", "sss")) default 0`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`topk(2)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(topk(2, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`topk(NaN)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(topk(NaN, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`topk(100500)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(topk(100500, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`bottomk(1)`, func(t *testing.T) {
		t.Parallel()
		q := `bottomk(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss") or label_set(time()<100, "a", "b"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`keep_last_value()`, func(t *testing.T) {
		t.Parallel()
		q := `keep_last_value(label_set(time() < 1300 default time() > 1700, "__name__", "foobar", "x", "y"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1200, 1200, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("foobar")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("x"),
			Value: []byte("y"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`keep_next_value()`, func(t *testing.T) {
		t.Parallel()
		q := `keep_next_value(label_set(time() < 1300 default time() > 1700, "__name__", "foobar", "x", "y"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1800, 1800, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("foobar")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("x"),
			Value: []byte("y"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`interpolate()`, func(t *testing.T) {
		t.Parallel()
		q := `interpolate(label_set(time() < 1300 default time() > 1700, "__name__", "foobar", "x", "y"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("foobar")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("x"),
			Value: []byte("y"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`interpolate(tail)`, func(t *testing.T) {
		t.Parallel()
		q := `interpolate(time() < 1300)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, nan, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`interpolate(head)`, func(t *testing.T) {
		t.Parallel()
		q := `interpolate(time() > 1500)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`interpolate(tail_head_and_middle)`, func(t *testing.T) {
		t.Parallel()
		q := `interpolate(time() > 1100 and time() < 1300 default time() > 1700 and time() < 1900)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1200, 1400, 1600, 1800, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`distinct_over_time([500s])`, func(t *testing.T) {
		t.Parallel()
		q := `distinct_over_time((time() < 1700)[500s])`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3, 3, 3, 3, 2, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`distinct_over_time([2.5i])`, func(t *testing.T) {
		t.Parallel()
		q := `distinct_over_time((time() < 1700)[2.5i])`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3, 3, 3, 3, 2, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`distinct()`, func(t *testing.T) {
		t.Parallel()
		q := `distinct(union(
			1+time() > 1100,
			label_set(time() > 1700, "foo", "bar"),
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1, 1, 1, 2, 2},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`vector2 if vector1`, func(t *testing.T) {
		t.Parallel()
		q := `(
			label_set(time()/10, "x", "y"),
			label_set(time(), "foo", "bar", "__name__", "x"),
		) if (
			label_set(time()>1400, "foo", "bar"),
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("x")
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`vector2 if vector2`, func(t *testing.T) {
		t.Parallel()
		q := `sort((
			label_set(time()/10, "x", "y"),
			label_set(time(), "foo", "bar", "__name__", "x"),
		) if (
			label_set(time()>1400, "foo", "bar"),
			label_set(time()<1400, "x", "y"),
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 120, nan, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("x"),
			Value: []byte("y"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("x")
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`scalar if vector1`, func(t *testing.T) {
		t.Parallel()
		q := `time() if (
			label_set(123, "foo", "bar"),
		)`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`scalar if vector2`, func(t *testing.T) {
		t.Parallel()
		q := `time() if (
			label_set(123, "foo", "bar"),
			alias(time() > 1400, "xxx"),
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`if-default`, func(t *testing.T) {
		t.Parallel()
		q := `time() if time() > 1400 default -time()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-1000, -1200, -1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ifnot-default`, func(t *testing.T) {
		t.Parallel()
		q := `time() ifnot time() > 1400 default -time()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, -1600, -1800, -2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ifnot`, func(t *testing.T) {
		t.Parallel()
		q := `time() ifnot time() > 1400`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ifnot-no-matching-timeseries`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(time(), "foo", "bar") ifnot label_set(time() > 1400, "x", "y")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`quantile(-2)`, func(t *testing.T) {
		t.Parallel()
		q := `quantile(-2, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		inf := math.Inf(-1)
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{inf, inf, inf, inf, inf, inf},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`quantile(0.2)`, func(t *testing.T) {
		t.Parallel()
		q := `quantile(0.2, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{7.333333333333334, 8.4, 9.466666666666669, 10.133333333333333, 10.4, 10.666666666666668},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`quantile(0.5)`, func(t *testing.T) {
		t.Parallel()
		q := `quantile(0.5, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{8.333333333333334, 9, 9.666666666666668, 10.333333333333332, 11, 11.666666666666668},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`quantiles("phi", 0.2, 0.5)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(quantiles("phi", 0.2, 0.5, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{7.333333333333334, 8.4, 9.466666666666669, 10.133333333333333, 10.4, 10.666666666666668},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("phi"),
			Value: []byte("0.2"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{8.333333333333334, 9, 9.666666666666668, 10.333333333333332, 11, 11.666666666666668},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("phi"),
			Value: []byte("0.5"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`median()`, func(t *testing.T) {
		t.Parallel()
		q := `median(label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{8.333333333333334, 9, 9.666666666666668, 10.333333333333332, 11, 11.666666666666668},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`median(3-timeseries)`, func(t *testing.T) {
		t.Parallel()
		q := `median(union(label_set(10, "foo", "bar"), label_set(time()/150, "baz", "sss"), time()/200))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`quantile(3)`, func(t *testing.T) {
		t.Parallel()
		q := `quantile(3, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		inf := math.Inf(+1)
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{inf, inf, inf, inf, inf, inf},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`quantile(NaN)`, func(t *testing.T) {
		t.Parallel()
		q := `quantile(NaN, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`mad()`, func(t *testing.T) {
		t.Parallel()
		q := `mad(
			alias(time(), "metric1"),
			alias(time()*1.5, "metric2"),
			label_set(time()*0.9, "baz", "sss"),
		)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 120, 140, 160, 180, 200},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`outliers_iqr()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(outliers_iqr((
			alias(time(), "m1"),
			alias(time()*1.5, "m2"),
			alias(time()*10, "m3"),
			alias(time()*1.2, "m4"),
			alias(time()*0.1, "m5"),
		)))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 120, 140, 160, 180, 200},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("m5")
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10000, 12000, 14000, 16000, 18000, 20000},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("m3")
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`outliers_mad(1)`, func(t *testing.T) {
		t.Parallel()
		q := `outliers_mad(1, (
			alias(time(), "metric1"),
			alias(time()*1.5, "metric2"),
			label_set(time()*0.9, "baz", "sss"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1500, 1800, 2100, 2400, 2700, 3000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("metric2")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`outliers_mad(5)`, func(t *testing.T) {
		t.Parallel()
		q := `outliers_mad(5, (
			alias(time(), "metric1"),
			alias(time()*1.5, "metric2"),
			label_set(time()*0.9, "baz", "sss"),
		))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`outliersk(0)`, func(t *testing.T) {
		t.Parallel()
		q := `outliersk(0, (
			label_set(1300, "foo", "bar"),
			label_set(time(), "baz", "sss"),
		))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`outliersk(1)`, func(t *testing.T) {
		t.Parallel()
		q := `outliersk(1, (
			label_set(2000, "foo", "bar"),
			label_set(time(), "baz", "sss"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`outliersk(3)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(outliersk(3, (
			label_set(1300, "foo", "bar"),
			label_set(time(), "baz", "sss"),
		)))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1300, 1300, 1300, 1300, 1300, 1300},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`range_trim_outliers()`, func(t *testing.T) {
		t.Parallel()
		q := `range_trim_outliers(0.5, time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1400, 1600, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_trim_outliers(time() > 1200)`, func(t *testing.T) {
		t.Parallel()
		q := `range_trim_outliers(0.5, time() > 1200)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, 1800, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_trim_spikes()`, func(t *testing.T) {
		t.Parallel()
		q := `range_trim_spikes(0.2, time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1200, 1400, 1600, 1800, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_trim_spikes(time() > 1200 <= 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `range_trim_spikes(0.2, time() > 1200 <= 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_trim_zscore()`, func(t *testing.T) {
		t.Parallel()
		q := `range_trim_zscore(0.9, time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1200, 1400, 1600, 1800, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_trim_zscore(time() > 1200 <= 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `range_trim_zscore(0.9, time() > 1200 <= 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1600, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_zscore()`, func(t *testing.T) {
		t.Parallel()
		q := `round(range_zscore(time()), 0.1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-1.5, -0.9, -0.3, 0.3, 0.9, 1.5},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_zscore(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `round(range_zscore(time() > 1200 < 1800), 0.1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, -1, 1, nan, nan},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_quantile(0.5)`, func(t *testing.T) {
		t.Parallel()
		q := `range_quantile(0.5, time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1500, 1500, 1500, 1500, 1500, 1500},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_quantile(0.5, time() > 1200 < 2000)`, func(t *testing.T) {
		t.Parallel()
		q := `range_quantile(0.5, time() > 1200 < 2000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1600, 1600, 1600, 1600, 1600, 1600},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_stddev()`, func(t *testing.T) {
		t.Parallel()
		q := `round(range_stddev(time()),0.01)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{341.57, 341.57, 341.57, 341.57, 341.57, 341.57},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_stddev(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `round(range_stddev(time() > 1200 < 1800),0.01)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 100, 100, 100, 100, 100},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_stdvar()`, func(t *testing.T) {
		t.Parallel()
		q := `round(range_stdvar(time()),0.01)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{116666.67, 116666.67, 116666.67, 116666.67, 116666.67, 116666.67},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_stdvar(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `round(range_stdvar(time() > 1200 < 1800),0.01)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10000, 10000, 10000, 10000, 10000, 10000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_median()`, func(t *testing.T) {
		t.Parallel()
		q := `range_median(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1500, 1500, 1500, 1500, 1500, 1500},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ttf(2000-time())`, func(t *testing.T) {
		t.Parallel()
		q := `ttf(2000-time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 866.6666666666666, 688.8888888888889, 496.2962962962963, 298.7654320987655, 99.58847736625516},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ttf(1000-time())`, func(t *testing.T) {
		t.Parallel()
		q := `ttf(1000-time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ttf(1500-time())`, func(t *testing.T) {
		t.Parallel()
		q := `ttf(1500-time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{500, 366.6666666666667, 188.8888888888889, 62.962962962962976, 20.987654320987662, 6.995884773662555},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ru(time(), 2000)`, func(t *testing.T) {
		t.Parallel()
		q := `ru(time(), 2000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{50, 40, 30, 20, 10, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ru(time() offset 100s, 2000)`, func(t *testing.T) {
		t.Parallel()
		q := `ru(time() offset 100s, 2000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{60, 50, 40, 30, 20, 10},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ru(time() offset 0.5i, 2000)`, func(t *testing.T) {
		t.Parallel()
		q := `ru(time() offset 0.5i, 2000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{60, 50, 40, 30, 20, 10},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ru(time() offset 1i, 2000)`, func(t *testing.T) {
		t.Parallel()
		q := `ru(time() offset 1.5i, 2000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{70, 60, 50, 40, 30, 20},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ru(time(), 1600)`, func(t *testing.T) {
		t.Parallel()
		q := `ru(time(), 1600)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{37.5, 25, 12.5, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`ru(1500-time(), 1000)`, func(t *testing.T) {
		t.Parallel()
		q := `ru(1500-time(), 1000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{50, 70, 90, 100, 100, 100},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`mode_over_time()`, func(t *testing.T) {
		t.Parallel()
		q := `mode_over_time(round(time()/500)[100s:1s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 3, 3, 4, 4},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rate_over_sum()`, func(t *testing.T) {
		t.Parallel()
		q := `rate_over_sum(round(time()/500)[100s:5s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.4, 0.4, 0.6, 0.6, 0.71, 0.8},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`zscore_over_time(rand)`, func(t *testing.T) {
		t.Parallel()
		q := `round(zscore_over_time(rand(0)[100s:10s]), 0.01)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-1.17, -0.08, 0.98, 0.67, 1.61, 1.55},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`zscore_over_time(const)`, func(t *testing.T) {
		t.Parallel()
		q := `zscore_over_time(1[100s:10s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`integrate(1)`, func(t *testing.T) {
		t.Parallel()
		q := `integrate(1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`integrate(time())`, func(t *testing.T) {
		t.Parallel()
		q := `integrate(time()/1e3)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{160, 200, 240, 280, 320, 360},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rate(time())`, func(t *testing.T) {
		t.Parallel()
		q := `rate(label_set(alias(time(), "foo"), "x", "y"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rate(time()) keep_metric_names`, func(t *testing.T) {
		t.Parallel()
		q := `rate(label_set(alias(time(), "foo"), "x", "y")) keep_metric_names`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foo")
		r.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("y"),
			},
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`sum(rate(time()) keep_metric_names) by (__name__)`, func(t *testing.T) {
		t.Parallel()
		q := `sum(rate(label_set(alias(time(), "foo"), "x", "y")) keep_metric_names) by (__name__)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foo")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rate(2000-time())`, func(t *testing.T) {
		t.Parallel()
		q := `rate(2000-time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{5.5, 4.5, 3.5, 2.5, 1.5, 0.5},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rate((2000-time())[100s])`, func(t *testing.T) {
		t.Parallel()
		q := `rate((2000-time())[100s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{5, 4, 3, 2, 1, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rate((2000-time())[100s:])`, func(t *testing.T) {
		t.Parallel()
		q := `rate((2000-time())[100s:])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{5, 4, 3, 2, 1, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rate((2000-time())[100s:100s])`, func(t *testing.T) {
		t.Parallel()
		q := `rate((2000-time())[100s:100s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 6, 4, 2, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rate((2000-time())[100s:100s] offset 100s)`, func(t *testing.T) {
		t.Parallel()
		q := `rate((2000-time())[100s:100s] offset 100s)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 7, 5, 3, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rate((2000-time())[100s:100s] offset 100s)[:] offset 100s`, func(t *testing.T) {
		t.Parallel()
		q := `rate((2000-time())[100s:100s] offset 100s)[:] offset 100s`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 7, 5, 3},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`increase_pure(time())`, func(t *testing.T) {
		t.Parallel()
		q := `increase_pure(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`increase(time())`, func(t *testing.T) {
		t.Parallel()
		q := `increase(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`increase(2000-time())`, func(t *testing.T) {
		t.Parallel()
		q := `increase(2000-time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 800, 600, 400, 200, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`increase_prometheus(time())`, func(t *testing.T) {
		t.Parallel()
		q := `increase_prometheus(time())`
		f(q, nil)
	})
	t.Run(`increase_prometheus(time()[201s])`, func(t *testing.T) {
		t.Parallel()
		q := `increase_prometheus(time()[201s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`running_max(1)`, func(t *testing.T) {
		t.Parallel()
		q := `running_max(1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`running_min(abs(1500-time()))`, func(t *testing.T) {
		t.Parallel()
		q := `running_min(abs(1500-time()))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{500, 300, 100, 100, 100, 100},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`running_min(abs(1500-time()) < 400 > 100)`, func(t *testing.T) {
		t.Parallel()
		q := `running_min(abs(1500-time()) < 400 > 100)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 300, 300, 300, 300, 300},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`running_max(abs(1300-time()))`, func(t *testing.T) {
		t.Parallel()
		q := `running_max(abs(1300-time()))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{300, 300, 300, 300, 500, 700},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`running_max(abs(1300-time()) > 300 < 700)`, func(t *testing.T) {
		t.Parallel()
		q := `running_max(abs(1300-time()) > 300 < 700)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, nan, 500, 500},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`running_sum(1)`, func(t *testing.T) {
		t.Parallel()
		q := `running_sum(1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 2, 3, 4, 5, 6},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`running_sum(time())`, func(t *testing.T) {
		t.Parallel()
		q := `running_sum(time()/1e3)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 2.2, 3.6, 5.2, 7, 9},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`running_sum(time() > 1.2 < 1.8)`, func(t *testing.T) {
		t.Parallel()
		q := `running_sum(time()/1e3 > 1.2 < 1.8)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1.4, 3, 3, 3},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`running_avg(time())`, func(t *testing.T) {
		t.Parallel()
		q := `running_avg(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1100, 1200, 1300, 1400, 1500},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`running_avg(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `running_avg(time() > 1200 < 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1400, 1500, 1500, 1500},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`smooth_exponential(time(), 1)`, func(t *testing.T) {
		t.Parallel()
		q := `smooth_exponential(time(), 1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`smooth_exponential(time(), 0)`, func(t *testing.T) {
		t.Parallel()
		q := `smooth_exponential(time(), 0)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1000, 1000, 1000, 1000, 1000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`smooth_exponential(time(), 0.5)`, func(t *testing.T) {
		t.Parallel()
		q := `smooth_exponential(time(), 0.5)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1100, 1250, 1425, 1612.5, 1806.25},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`remove_resets()`, func(t *testing.T) {
		t.Parallel()
		q := `remove_resets(abs(1500-time()))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{500, 800, 900, 900, 1100, 1300},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`remove_resets(sum)`, func(t *testing.T) {
		t.Parallel()
		q := `remove_resets(sum(
			alias(time(), "full"),
			alias(time()/5 < 300, "partial"),
		))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1200, 1440, 1680, 1680, 1880, 2080},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_avg(time())`, func(t *testing.T) {
		t.Parallel()
		q := `range_avg(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1500, 1500, 1500, 1500, 1500, 1500},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_min(time())`, func(t *testing.T) {
		t.Parallel()
		q := `range_min(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1000, 1000, 1000, 1000, 1000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_min(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `range_min(time() > 1200 < 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1400, 1400, 1400, 1400, 1400, 1400},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_normalize(time(),alias(-time(),"negative"))`, func(t *testing.T) {
		t.Parallel()
		q := `range_normalize(time(),alias(-time(), "negative"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0.2, 0.4, 0.6, 0.8, 1},
			Timestamps: timestampsExpected,
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 0.8, 0.6, 0.4, 0.2, 0},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("negative")
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`range_normalize(time() > 1200 < 1800,alias(-(time() > 1400 < 2000),"negative"))`, func(t *testing.T) {
		t.Parallel()
		q := `range_normalize(time() > 1200 < 1800,alias(-(time() > 1200 < 2000), "negative"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 0, 1, nan, nan},
			Timestamps: timestampsExpected,
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1, 0.5, 0, nan},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("negative")
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`range_first(time())`, func(t *testing.T) {
		t.Parallel()
		q := `range_first(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1000, 1000, 1000, 1000, 1000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_first(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `range_first(time() > 1200 < 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1400, 1400, 1400, 1400, 1400, 1400},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_mad(time())`, func(t *testing.T) {
		t.Parallel()
		q := `range_mad(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{300, 300, 300, 300, 300, 300},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_mad(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `range_mad(time() > 1200 < 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{100, 100, 100, 100, 100, 100},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_max(time())`, func(t *testing.T) {
		t.Parallel()
		q := `range_max(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2000, 2000, 2000, 2000, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_max(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `range_max(time() > 1200 < 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1600, 1600, 1600, 1600, 1600, 1600},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_sum(time())`, func(t *testing.T) {
		t.Parallel()
		q := `range_sum(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{9000, 9000, 9000, 9000, 9000, 9000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_sum(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `range_sum(time() > 1200 < 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3000, 3000, 3000, 3000, 3000, 3000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_last(time())`, func(t *testing.T) {
		t.Parallel()
		q := `range_last(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2000, 2000, 2000, 2000, 2000, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_last(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `range_last(time() > 1200 < 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1600, 1600, 1600, 1600, 1600, 1600},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_linear_regression(time())`, func(t *testing.T) {
		t.Parallel()
		q := `range_linear_regression(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_linear_regression(-time())`, func(t *testing.T) {
		t.Parallel()
		q := `range_linear_regression(-time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-1000, -1200, -1400, -1600, -1800, -2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_linear_regression(time() > 1200 < 1800)`, func(t *testing.T) {
		t.Parallel()
		q := `range_linear_regression(time() > 1200 < 1800)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`range_linear_regression(100/time())`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(round((
				alias(range_linear_regression(100/time()), "regress"),
				alias(100/time(), "orig"),
			),
			0.001
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.1, 0.083, 0.071, 0.062, 0.056, 0.05},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("orig")
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.095, 0.085, 0.075, 0.066, 0.056, 0.046},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("regress")
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`deriv(N)`, func(t *testing.T) {
		t.Parallel()
		q := `deriv(1000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`deriv(time())`, func(t *testing.T) {
		t.Parallel()
		q := `deriv(2*time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`deriv(-time())`, func(t *testing.T) {
		t.Parallel()
		q := `deriv(-time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-1, -1, -1, -1, -1, -1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`delta(time())`, func(t *testing.T) {
		t.Parallel()
		q := `delta(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`delta(delta(time()))`, func(t *testing.T) {
		t.Parallel()
		q := `delta(delta(2*time()))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`delta(-time())`, func(t *testing.T) {
		t.Parallel()
		q := `delta(-time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-200, -200, -200, -200, -200, -200},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`delta(1)`, func(t *testing.T) {
		t.Parallel()
		q := `delta(1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`delta_prometheus(time())`, func(t *testing.T) {
		t.Parallel()
		q := `delta_prometheus(time())`
		f(q, nil)
	})
	t.Run(`delta_prometheus(time()[201s])`, func(t *testing.T) {
		t.Parallel()
		q := `delta_prometheus(time()[201s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`median_over_time("foo")`, func(t *testing.T) {
		t.Parallel()
		q := `median_over_time("foo")`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`median_over_time(12)`, func(t *testing.T) {
		t.Parallel()
		q := `median_over_time(12)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{12, 12, 12, 12, 12, 12},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`hoeffding_bound_lower()`, func(t *testing.T) {
		t.Parallel()
		q := `hoeffding_bound_lower(0.9, rand(0)[:10s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.2516770508510652, 0.2830570387745462, 0.27716232108436645, 0.3679356319931767, 0.3168460474120903, 0.23156726248243734},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`hoeffding_bound_upper()`, func(t *testing.T) {
		t.Parallel()
		q := `hoeffding_bound_upper(0.9, alias(rand(0), "foobar")[:10s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.6510581320042821, 0.7261021731890429, 0.7245290097397009, 0.8113950442584258, 0.7736122275568004, 0.6658564048254882},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`aggr_over_time(single-func)`, func(t *testing.T) {
		t.Parallel()
		q := `round(aggr_over_time("increase", rand(0)[:10s]),0.01)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{5.47, 6.64, 6.84, 7.24, 5.17, 6.59},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("increase"),
		}}
		resultExpected := []netstorage.Result{r1}
		f(q, resultExpected)
	})
	t.Run(`aggr_over_time(multi-func)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(aggr_over_time(("min_over_time", "median_over_time", "max_over_time"), round(rand(0),0.1)[:10s]))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 0, 0, 0, 0, 0},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("min_over_time"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.4, 0.5, 0.5, 0.75, 0.6, 0.45},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("median_over_time"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.8, 0.9, 1, 0.9, 1, 0.9},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("max_over_time"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`avg(aggr_over_time(multi-func))`, func(t *testing.T) {
		t.Parallel()
		q := `avg(aggr_over_time(("min_over_time", "max_over_time"), time()[:10s]))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{905, 1105, 1305, 1505, 1705, 1905},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`avg(aggr_over_time(multi-func)) by (rollup)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(avg(aggr_over_time(("min_over_time", "max_over_time"), time()[:10s])) by (rollup))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{810, 1010, 1210, 1410, 1610, 1810},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("min_over_time"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("max_over_time"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`rollup_candlestick()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(rollup_candlestick(alias(round(rand(0),0.01),"foobar")[:10s]))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.02, 0.02, 0.03, 0, 0.03, 0.02},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("foobar")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("low"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.9, 0.32, 0.82, 0.13, 0.28, 0.86},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("foobar")
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("open"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.1, 0.04, 0.49, 0.46, 0.57, 0.92},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.MetricGroup = []byte("foobar")
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("close"),
		}}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.9, 0.94, 0.97, 0.93, 0.98, 0.92},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.MetricGroup = []byte("foobar")
		r4.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("high"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3, r4}
		f(q, resultExpected)
	})
	t.Run(`rollup_candlestick(high)`, func(t *testing.T) {
		t.Parallel()
		q := `rollup_candlestick(alias(round(rand(0),0.01),"foobar")[:10s], "high")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.9, 0.94, 0.97, 0.93, 0.98, 0.92},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("foobar")
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("high"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rollup_increase()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(rollup_increase(time()))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("min"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("max"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{200, 200, 200, 200, 200, 200},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("avg"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`rollup_rate()`, func(t *testing.T) {
		t.Parallel()
		q := `rollup_rate((2200-time())[600s])`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6, 5, 4, 3, 2, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("avg"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{7, 6, 5, 4, 3, 2},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("max"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{5, 4, 3, 2, 1, 0},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("min"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`rollup_rate(q, "max")`, func(t *testing.T) {
		t.Parallel()
		q := `rollup_rate((2200-time())[600s], "max")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{7, 6, 5, 4, 3, 2},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rollup_rate(q, "avg")`, func(t *testing.T) {
		t.Parallel()
		q := `rollup_rate((2200-time())[600s], "avg")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6, 5, 4, 3, 2, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rollup_scrape_interval()`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label(rollup_scrape_interval(1[5m:10S]), "rollup")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("avg"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("max"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("min"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`rollup()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(rollup(time()[:50s]))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{850, 1050, 1250, 1450, 1650, 1850},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("min"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{925, 1125, 1325, 1525, 1725, 1925},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("avg"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("max"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`rollup_deriv()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(rollup_deriv(time()[100s:50s]))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("min"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("max"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("avg"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`rollup_deriv(q, "max")`, func(t *testing.T) {
		t.Parallel()
		q := `sort(rollup_deriv(time()[100s:50s], "max"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`{}`, func(t *testing.T) {
		t.Parallel()
		q := `{}`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`rate({}[:5s])`, func(t *testing.T) {
		t.Parallel()
		q := `rate({}[:5s])`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`start()`, func(t *testing.T) {
		t.Parallel()
		q := `time() - start()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0, 200, 400, 600, 800, 1000},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`end()`, func(t *testing.T) {
		t.Parallel()
		q := `end() - time()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 800, 600, 400, 200, 0},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`step()`, func(t *testing.T) {
		t.Parallel()
		q := `time() / step()`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{5, 6, 7, 8, 9, 10},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`lag()`, func(t *testing.T) {
		t.Parallel()
		q := `lag(time()[60s:17s])`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{14, 10, 6, 2, 15, 11},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`()`, func(t *testing.T) {
		t.Parallel()
		q := `()`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`union()`, func(t *testing.T) {
		t.Parallel()
		q := `union()`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`union(1)`, func(t *testing.T) {
		t.Parallel()
		q := `union(1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`(1)`, func(t *testing.T) {
		t.Parallel()
		q := `(1)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`union(identical_labels)`, func(t *testing.T) {
		t.Parallel()
		q := `union(label_set(1, "foo", "bar"), label_set(2, "foo", "bar"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`(identical_labels)`, func(t *testing.T) {
		t.Parallel()
		q := `(label_set(1, "foo", "bar"), label_set(2, "foo", "bar"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`union(identical_labels_with_names)`, func(t *testing.T) {
		t.Parallel()
		q := `union(label_set(1, "foo", "bar", "__name__", "xx"), label_set(2, "__name__", "xx", "foo", "bar"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("xx")
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`(identical_labels_with_names)`, func(t *testing.T) {
		t.Parallel()
		q := `(label_set(1, "foo", "bar", "__name__", "xx"), label_set(2, "__name__", "xx", "foo", "bar"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r.MetricName.MetricGroup = []byte("xx")
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`union(identical_labels_different_names)`, func(t *testing.T) {
		t.Parallel()
		q := `union(label_set(1, "foo", "bar", "__name__", "xx"), label_set(2, "__name__", "yy", "foo", "bar"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("xx")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("yy")
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`(identical_labels_different_names)`, func(t *testing.T) {
		t.Parallel()
		q := `(label_set(1, "foo", "bar", "__name__", "xx"), label_set(2, "__name__", "yy", "foo", "bar"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("xx")
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("yy")
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`((1),(2,3))`, func(t *testing.T) {
		t.Parallel()
		q := `((
			alias(1, "x1"),
		),(
			alias(2, "x2"),
			alias(3, "x3"),
		))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.MetricGroup = []byte("x1")
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("x2")
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3, 3, 3, 3, 3, 3},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.MetricGroup = []byte("x3")
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`union(more-than-two)`, func(t *testing.T) {
		t.Parallel()
		q := `union(
			label_set(1, "foo", "bar", "__name__", "xx"),
			label_set(2, "__name__", "yy", "foo", "bar"),
			label_set(time(), "qwe", "123") or label_set(3, "__name__", "rt"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1000, 1200, 1400, 1600, 1800, 2000},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("qwe"),
			Value: []byte("123"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3, 3, 3, 3, 3, 3},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.MetricGroup = []byte("rt")
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.MetricGroup = []byte("xx")
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.MetricGroup = []byte("yy")
		r4.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3, r4}
		f(q, resultExpected)
	})
	t.Run(`count_values_big_numbers`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label(
			count_values("xxx", (alias(772424014, "first"), alias(772424230, "second"))),
			"xxx"
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("772424014"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("772424230"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`count_values`, func(t *testing.T) {
		t.Parallel()
		q := `count_values("xxx", label_set(10, "foo", "bar") or label_set(time()/100, "foo", "bar", "baz", "xx"))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("10"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1, nan, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("12"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, 1, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("14"),
			},
		}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 1, nan, nan},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("16"),
			},
		}
		r5 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, nan, 1, nan},
			Timestamps: timestampsExpected,
		}
		r5.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("18"),
			},
		}
		r6 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, nan, nan, 1},
			Timestamps: timestampsExpected,
		}
		r6.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("20"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3, r4, r5, r6}
		f(q, resultExpected)
	})
	t.Run(`count_values by (xxx)`, func(t *testing.T) {
		t.Parallel()
		q := `count_values("xxx", label_set(10, "foo", "bar", "xxx", "aaa") or label_set(floor(time()/600), "foo", "bar", "baz", "xx")) by (xxx)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, nan, nan, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("1"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1, 1, 1, nan, nan},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("2"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, nan, 1, 1},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("3"),
			},
		}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("xxx"),
				Value: []byte("10"),
			},
		}
		// expected sorted output for strings 1, 10, 2, 3
		resultExpected := []netstorage.Result{r1, r4, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`count_values without (baz)`, func(t *testing.T) {
		t.Parallel()
		q := `count_values("xxx", label_set(floor(time()/600), "foo", "bar")) without (baz)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, nan, nan, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("1"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, 1, 1, 1, nan, nan},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("2"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, nan, 1, 1},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
			{
				Key:   []byte("xxx"),
				Value: []byte("3"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3}
		f(q, resultExpected)
	})
	t.Run(`result sorting`, func(t *testing.T) {
		t.Parallel()
		q := `(label_set(1, "instance", "localhost:1001", "type", "free"),
			label_set(1, "instance", "localhost:1001", "type", "buffers"),
			label_set(1, "instance", "localhost:1000", "type", "buffers"),
			label_set(1, "instance", "localhost:1000", "type", "free"),
		)`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		testAddLabels(t, &r1.MetricName,
			"instance", "localhost:1000", "type", "buffers")
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		testAddLabels(t, &r2.MetricName,
			"instance", "localhost:1000", "type", "free")
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		testAddLabels(t, &r3.MetricName,
			"instance", "localhost:1001", "type", "buffers")
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		testAddLabels(t, &r4.MetricName,
			"instance", "localhost:1001", "type", "free")
		resultExpected := []netstorage.Result{r1, r2, r3, r4}
		f(q, resultExpected)
	})
	t.Run(`no_sorting_for_or`, func(t *testing.T) {
		t.Parallel()
		q := `label_set(2, "foo", "bar") or label_set(1, "foo", "baz")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("foo"),
				Value: []byte("baz"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`sort_by_label_numeric(multiple_labels_only_string)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label_numeric((
			label_set(1, "x", "b", "y", "aa"),
			label_set(2, "x", "a", "y", "aa"),
		), "y", "x")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("a"),
			},
			{
				Key:   []byte("y"),
				Value: []byte("aa"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("b"),
			},
			{
				Key:   []byte("y"),
				Value: []byte("aa"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`sort_by_label_numeric(multiple_labels_numbers_special_chars)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label_numeric((
			label_set(1, "x", "1:0:2", "y", "1:0:1"),
			label_set(2, "x", "1:0:15", "y", "1:0:1"),
		), "x", "y")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("1:0:2"),
			},
			{
				Key:   []byte("y"),
				Value: []byte("1:0:1"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("1:0:15"),
			},
			{
				Key:   []byte("y"),
				Value: []byte("1:0:1"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`sort_by_label_numeric_desc(multiple_labels_numbers_special_chars)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label_numeric_desc((
			label_set(1, "x", "1:0:2", "y", "1:0:1"),
			label_set(2, "x", "1:0:15", "y", "1:0:1"),
		), "x", "y")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("1:0:15"),
			},
			{
				Key:   []byte("y"),
				Value: []byte("1:0:1"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("x"),
				Value: []byte("1:0:2"),
			},
			{
				Key:   []byte("y"),
				Value: []byte("1:0:1"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2}
		f(q, resultExpected)
	})
	t.Run(`sort_by_label_numeric(alias_numbers_with_special_chars)`, func(t *testing.T) {
		t.Parallel()
		q := `sort_by_label_numeric((
			label_set(4, "a", "DS50:1/0/15"),
			label_set(1, "a", "DS50:1/0/0"),
			label_set(2, "a", "DS50:1/0/1"),
			label_set(3, "a", "DS50:1/0/2"),
		), "a")`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("a"),
				Value: []byte("DS50:1/0/0"),
			},
		}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2, 2, 2, 2, 2, 2},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("a"),
				Value: []byte("DS50:1/0/1"),
			},
		}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{3, 3, 3, 3, 3, 3},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("a"),
				Value: []byte("DS50:1/0/2"),
			},
		}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{4, 4, 4, 4, 4, 4},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.Tags = []storage.Tag{
			{
				Key:   []byte("a"),
				Value: []byte("DS50:1/0/15"),
			},
		}
		resultExpected := []netstorage.Result{r1, r2, r3, r4}
		f(q, resultExpected)
	})
}

func TestExecError(t *testing.T) {
	f := func(q string) {
		t.Helper()
		ec := &EvalConfig{
			Start:              1000,
			End:                2000,
			Step:               100,
			MaxPointsPerSeries: 1e4,
			MaxSeries:          1000,
			Deadline:           searchutils.NewDeadline(time.Now(), time.Minute, ""),
			RoundDigits:        100,
		}
		for i := 0; i < 4; i++ {
			rv, err := Exec(nil, ec, q, false)
			if err == nil {
				t.Fatalf(`expecting non-nil error on %q`, q)
			}
			if rv != nil {
				t.Fatalf(`expecting nil rv`)
			}
			rv, err = Exec(nil, ec, q, true)
			if err == nil {
				t.Fatalf(`expecting non-nil error on %q`, q)
			}
			if rv != nil {
				t.Fatalf(`expecting nil rv`)
			}
		}
	}

	// Empty expr
	f("")
	f("    ")

	// Invalid expr
	f("1-")

	// Non-existing func
	f(`nonexisting()`)

	// Invalid number of args
	f(`range_stddev()`)
	f(`range_stdvar()`)
	f(`range_quantile()`)
	f(`range_quantile(1, 2, 3)`)
	f(`range_median()`)
	f(`abs()`)
	f(`abs(1,2)`)
	f(`absent(1, 2)`)
	f(`clamp()`)
	f(`clamp_max()`)
	f(`clamp_min(1,2,3)`)
	f(`hour(1,2)`)
	f(`label_join()`)
	f(`label_replace(1)`)
	f(`label_transform(1)`)
	f(`label_set()`)
	f(`label_set(1, "foo")`)
	f(`label_map()`)
	f(`label_map(1)`)
	f(`label_del()`)
	f(`label_keep()`)
	f(`label_match()`)
	f(`label_mismatch()`)
	f(`label_graphite_group()`)
	f(`round()`)
	f(`round(1,2,3)`)
	f(`sgn()`)
	f(`scalar()`)
	f(`sort(1,2)`)
	f(`sort_desc()`)
	f(`sort_by_label()`)
	f(`sort_by_label_desc()`)
	f(`sort_by_label_numeric()`)
	f(`sort_by_label_numeric_desc()`)
	f(`timestamp()`)
	f(`timestamp_with_name()`)
	f(`vector()`)
	f(`histogram_quantile()`)
	f(`histogram_quantiles()`)
	f(`sum()`)
	f(`count_values()`)
	f(`quantile()`)
	f(`any()`)
	f(`group()`)
	f(`topk()`)
	f(`topk_min()`)
	f(`topk_max()`)
	f(`topk_avg()`)
	f(`topk_median()`)
	f(`topk_last()`)
	f(`limitk()`)
	f(`bottomk()`)
	f(`bottomk_min()`)
	f(`bottomk_max()`)
	f(`bottomk_avg()`)
	f(`bottomk_median()`)
	f(`bottomk_last()`)
	f(`time(123)`)
	f(`start(1)`)
	f(`end(1)`)
	f(`step(1)`)
	f(`running_sum(1, 2)`)
	f(`range_mad()`)
	f(`range_sum(1, 2)`)
	f(`range_trim_outliers()`)
	f(`range_trim_spikes()`)
	f(`range_trim_zscore()`)
	f(`range_zscore()`)
	f(`range_first(1,  2)`)
	f(`range_last(1, 2)`)
	f(`range_linear_regression(1, 2)`)
	f(`smooth_exponential()`)
	f(`smooth_exponential(1)`)
	f(`remove_resets()`)
	f(`sin()`)
	f(`sinh()`)
	f(`cos()`)
	f(`cosh()`)
	f(`asin()`)
	f(`asinh()`)
	f(`acos()`)
	f(`acosh()`)
	f(`rand(123, 456)`)
	f(`rand_normal(123, 456)`)
	f(`rand_exponential(122, 456)`)
	f(`pi(123)`)
	f(`now(123)`)
	f(`label_copy()`)
	f(`label_move()`)
	f(`median_over_time()`)
	f(`median()`)
	f(`keep_last_value()`)
	f(`keep_next_value()`)
	f(`interpolate()`)
	f(`distinct_over_time()`)
	f(`distinct()`)
	f(`alias()`)
	f(`alias(1)`)
	f(`alias(1, "foo", "bar")`)
	f(`lifetime()`)
	f(`lag()`)
	f(`aggr_over_time()`)
	f(`aggr_over_time(foo)`)
	f(`aggr_over_time("foo", bar, 1)`)
	f(`sum(aggr_over_time())`)
	f(`sum(aggr_over_time(foo))`)
	f(`count(aggr_over_time("foo", bar, 1))`)
	f(`hoeffding_bound_lower()`)
	f(`hoeffding_bound_lower(1)`)
	f(`hoeffding_bound_lower(0.99, foo, 1)`)
	f(`hoeffding_bound_upper()`)
	f(`hoeffding_bound_upper(1)`)
	f(`hoeffding_bound_upper(0.99, foo, 1)`)
	f(`mad()`)
	f(`outliers_mad()`)
	f(`outliers_mad(1)`)
	f(`outliersk()`)
	f(`outliersk(1)`)
	f(`mode_over_time()`)
	f(`rate_over_sum()`)
	f(`zscore_over_time()`)
	f(`mode()`)
	f(`share()`)
	f(`zscore()`)
	f(`prometheus_buckets()`)
	f(`buckets_limit()`)
	f(`buckets_limit(1)`)
	f(`duration_over_time()`)
	f(`share_le_over_time()`)
	f(`share_gt_over_time()`)
	f(`count_le_over_time()`)
	f(`count_gt_over_time()`)
	f(`count_eq_over_time()`)
	f(`count_ne_over_time()`)
	f(`timezone_offset()`)
	f(`bitmap_and()`)
	f(`bitmap_or()`)
	f(`bitmap_xor()`)
	f(`quantiles()`)
	f(`limit_offset()`)
	f(`increase()`)
	f(`increase_prometheus()`)
	f(`changes()`)
	f(`changes_prometheus()`)
	f(`delta()`)
	f(`delta_prometheus()`)
	f(`rollup_candlestick()`)
	f(`rollup()`)
	f(`drop_empty_series()`)
	f(`drop_common_labels()`)
	f(`labels_equal()`)

	// Invalid argument type
	f(`median_over_time({}, 2)`)
	f(`smooth_exponential(1, 1 or label_set(2, "x", "y"))`)
	f(`count_values(1, 2)`)
	f(`count_values(1 or label_set(2, "xx", "yy"), 2)`)
	f(`quantile(1 or label_set(2, "xx", "foo"), 1)`)
	f(`clamp_max(1, 1 or label_set(2, "xx", "foo"))`)
	f(`clamp_min(1, 1 or label_set(2, "xx", "foo"))`)
	f(`topk(label_set(2, "xx", "foo") or 1, 12)`)
	f(`topk_avg(label_set(2, "xx", "foo") or 1, 12)`)
	f(`limitk(label_set(2, "xx", "foo") or 1, 12)`)
	f(`limit_offet((alias(1,"foo"),alias(2,"bar")), 2, 10)`)
	f(`limit_offet(1, (alias(1,"foo"),alias(2,"bar")), 10)`)
	f(`round(1, 1 or label_set(2, "xx", "foo"))`)
	f(`histogram_quantile(1 or label_set(2, "xx", "foo"), 1)`)
	f(`histogram_quantiles("foo", 1 or label_set(2, "xxx", "foo"), 2)`)
	f(`sort_by_label_numeric(1, 2)`)
	f(`label_set(1, 2, 3)`)
	f(`label_set(1, "foo", (label_set(1, "foo", bar") or label_set(2, "xxx", "yy")))`)
	f(`label_set(1, "foo", 3)`)
	f(`label_del(1, 2)`)
	f(`label_copy(1, 2)`)
	f(`label_move(1, 2, 3)`)
	f(`label_move(1, "foo", 3)`)
	f(`label_keep(1, 2)`)
	f(`label_join(1, 2, 3)`)
	f(`label_join(1, "foo", 2)`)
	f(`label_join(1, "foo", "bar", 2)`)
	f(`label_replace(1, 2, 3, 4, 5)`)
	f(`label_replace(1, "foo", 3, 4, 5)`)
	f(`label_replace(1, "foo", "bar", 4, 5)`)
	f(`label_replace(1, "foo", "bar", "baz", 5)`)
	f(`label_replace(1, "foo", "bar", "baz", "invalid(regexp")`)
	f(`label_transform(1, 2, 3, 4)`)
	f(`label_transform(1, "foo", 3, 4)`)
	f(`label_transform(1, "foo", "bar", 4)`)
	f(`label_transform(1, "foo", "invalid(regexp", "baz`)
	f(`label_match(1, 2, 3)`)
	f(`label_mismatch(1, 2, 3)`)
	f(`label_uppercase()`)
	f(`label_lowercase()`)
	f(`alias(1, 2)`)
	f(`aggr_over_time(1, 2)`)
	f(`aggr_over_time(("foo", "bar"), 3)`)
	f(`outliersk((label_set(1, "foo", "bar"), label_set(2, "x", "y")), 123)`)

	// Duplicate timeseries
	f(`(label_set(1, "foo", "bar") or label_set(2, "foo", "baz"))
		+ on(xx)
		(label_set(1, "foo", "bar") or label_set(2, "foo", "baz"))`)

	// Invalid binary op groupings
	f(`1 + group_left() (label_set(1, "foo", bar"), label_set(2, "foo", "baz"))`)
	f(`1 + on() group_left() (label_set(1, "foo", bar"), label_set(2, "foo", "baz"))`)
	f(`1 + on(a) group_left(b) (label_set(1, "foo", bar"), label_set(2, "foo", "baz"))`)
	f(`label_set(1, "foo", "bar") + on(foo) group_left() (label_set(1, "foo", "bar", "a", "b"), label_set(1, "foo", "bar", "a", "c"))`)
	f(`(label_set(1, "foo", bar"), label_set(2, "foo", "baz")) + group_right 1`)
	f(`(label_set(1, "foo", bar"), label_set(2, "foo", "baz")) + on() group_right 1`)
	f(`(label_set(1, "foo", bar"), label_set(2, "foo", "baz")) + on(a) group_right(b,c) 1`)
	f(`(label_set(1, "foo", bar"), label_set(2, "foo", "baz")) + on() 1`)
	f(`(label_set(1, "foo", "bar", "a", "b"), label_set(1, "foo", "bar", "a", "c")) + on(foo) group_right() label_set(1, "foo", "bar")`)
	f(`1 + on() (label_set(1, "foo", bar"), label_set(2, "foo", "baz"))`)

	// duplicate metrics after binary op
	f(`(
		label_set(time(), "__name__", "foo", "a", "x"),
		label_set(time()+200, "__name__", "bar", "a", "x"),
	) > bool 1300`)
	f(`(
		label_set(time(), "__name__", "foo", "a", "x"),
		label_set(time()+200, "__name__", "bar", "a", "x"),
	) + 10`)

	// Invalid aggregates
	f(`sum(1) foo (bar)`)
	f(`sum foo () (bar)`)
	f(`sum(foo) by (1)`)
	f(`count(foo) without ("bar")`)

	// With expressions
	f(`ttf()`)
	f(`ttf(1, 2)`)
	f(`ru()`)
	f(`ru(1)`)
	f(`ru(1,3,3)`)

	// Invalid rollup tags
	f(`rollup_rate(time()[5m], "")`)
	f(`rollup_rate(time()[5m], "foo")`)
	f(`rollup_rate(time()[5m], "foo", "bar")`)
	f(`rollup_candlestick(time(), "foo")`)
}

func testResultsEqual(t *testing.T, result, resultExpected []netstorage.Result) {
	t.Helper()
	if len(result) != len(resultExpected) {
		t.Fatalf(`unexpected timeseries count; got %d; want %d`, len(result), len(resultExpected))
	}
	for i := range result {
		r := &result[i]
		rExpected := &resultExpected[i]
		testMetricNamesEqual(t, &r.MetricName, &rExpected.MetricName, i)
		testRowsEqual(t, r.Values, r.Timestamps, rExpected.Values, rExpected.Timestamps)
	}
}

func testMetricNamesEqual(t *testing.T, mn, mnExpected *storage.MetricName, pos int) {
	t.Helper()
	if string(mn.MetricGroup) != string(mnExpected.MetricGroup) {
		t.Fatalf(`unexpected MetricGroup at #%d; got %q; want %q; metricGot=%s, metricExpected=%s`,
			pos, mn.MetricGroup, mnExpected.MetricGroup, mn.String(), mnExpected.String())
	}
	if len(mn.Tags) != len(mnExpected.Tags) {
		t.Fatalf(`unexpected tags count at #%d; got %d; want %d; metricGot=%s, metricExpected=%s`, pos, len(mn.Tags), len(mnExpected.Tags), mn.String(), mnExpected.String())
	}
	for i := range mn.Tags {
		tag := &mn.Tags[i]
		tagExpected := &mnExpected.Tags[i]
		if string(tag.Key) != string(tagExpected.Key) {
			t.Fatalf(`unexpected tag key at #%d,%d; got %q; want %q; metricGot=%s, metricExpected=%s`, pos, i, tag.Key, tagExpected.Key, mn.String(), mnExpected.String())
		}
		if string(tag.Value) != string(tagExpected.Value) {
			t.Fatalf(`unexpected tag value for key %q at #%d,%d; got %q; want %q; metricGot=%s, metricExpected=%s`,
				tag.Key, pos, i, tag.Value, tagExpected.Value, mn.String(), mnExpected.String())
		}
	}
}

func testAddLabels(t *testing.T, mn *storage.MetricName, labels ...string) {
	t.Helper()
	if len(labels)%2 > 0 {
		t.Fatalf("uneven number of labels passed: %v", labels)
	}
	for i := 0; i < len(labels); i += 2 {
		mn.Tags = append(mn.Tags, storage.Tag{
			Key:   []byte(labels[i]),
			Value: []byte(labels[i+1]),
		})
	}
}

func TestMetricsqlIsLikelyInvalid_False(t *testing.T) {
	f := func(q string) {
		t.Helper()

		e, err := metricsql.Parse(q)
		if err != nil {
			t.Fatal(err)
		}
		if metricsql.IsLikelyInvalid(e) {
			t.Fatalf("unexpected result for metricsql.IsLikelyInvalid(%q); got true; want false", q)
		}
	}

	f("http_total[5m]")
	f("sum(http_total)")
	f("sum(foo, bar)")
	f("absent(http_total)")
	f("rate(http_total[1m])")
	f("avg_over_time(up[1m])")
	f("sum(rate(http_total[1m]))")
	f("sum(sum(http_total))")
	f(`sum(sum_over_time(http_total[1m] )) by (instance)`)
	f("sum(up{cluster='a'}[1m] or up{cluster='b'}[1m])")
	f("(avg_over_time(alarm_test1[1m]) - avg_over_time(alarm_test1[1m] offset 5m)) > 0.1")
	f("http_total[1m] offset 1m")
	f("sum(http_total offset 1m)")

	// subquery
	f("rate(http_total[5m])[5m:1m]")
	f("rate(sum(http_total)[5m:1m])")
	f("rate(rate(http_total[5m])[5m:1m])")
	f("sum(rate(http_total[1m]))")
	f("sum(rate(sum(http_total)[5m:1m]))")
	f("rate(sum(rate(http_total[5m]))[5m:1m])")
	f("rate(sum(sum(http_total))[5m:1m])")
	f("rate(sum(rate(http_total[5m]))[5m:1m])")
	f("rate(sum(sum(http_total))[5m:1m])")
	f("avg_over_time(rate(http_total[5m])[5m:1m])")
	f("delta(avg_over_time(up[1m])[5m:1m]) > 0.1")
	f("avg_over_time(avg by (site) (metric)[2m:1m])")

	f("sum(http_total)[5m:1m] offset 1m")
	f("round(sum(sum_over_time(http_total[1m])) by (instance))[5m:1m] offset 1m")

	f("rate(sum(http_total)[5m:1m]) - rate(sum(http_total)[5m:1m])")
	f("avg_over_time((rate(http_total[5m])-rate(http_total[5m]))[5m:1m])")

	f("sum_over_time((up{cluster='a'} or up{cluster='b'})[5m:1m])")
	f("sum_over_time((up{cluster='a'} or up{cluster='b'})[5m:1m])")
	f("sum(sum_over_time((up{cluster='a'} or up{cluster='b'})[5m:1m])) by (instance)")

	// step (or resolution) is optional in subqueries
	f("max_over_time(rate(my_counter_total[5m])[1h:])")
	f("max_over_time(rate(my_counter_total[5m])[1h:1m])[5m:1m]")
	f("max_over_time(rate(my_counter_total[5m])[1h:])[5m:]")

	f(`
WITH (
    cpuSeconds = node_cpu_seconds_total{instance=~"$node:$port",job=~"$job"},
    cpuIdle = rate(cpuSeconds{mode='idle'}[5m])
)
max_over_time(cpuIdle[1h:])`)

	// These queries are mostly harmless, e.g. they return mostly correct results.
	f("rate(http_total)[5m:1m]")
	f("up[:5m]")
	f("sum(up[:5m])")
	f("absent(foo[5m])")
	f("sum(up[5m])")
	f("avg(foo[5m])")
	f("sort(foo[5m])")

	// These are valid subqueries with MetricsQL extention, which allows omitting lookbehind window for rollup functions
	f("rate(rate(http_total)[5m:1m])")
	f("rate(sum(rate(http_total))[5m:1m])")
	f("rate(sum(rate(http_total))[5m:1m])")
	f("avg_over_time((rate(http_total)-rate(http_total))[5m:1m])")

	// These are valid MetricsQL queries, which return correct result most of the time
	f("count_over_time(http_total)")

	// The following queries are from https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3974
	//
	// They are mostly correct. It is better to teach metricsql parser converting them to proper ones
	// instead of denying them.
	f("sum(http_total) offset 1m")
	f(`round(sum(sum_over_time(http_total[1m])) by (instance)) offset 1m`)

}

func TestMetricsqlIsLikelyInvalid_True(t *testing.T) {
	f := func(q string) {
		t.Helper()

		e, err := metricsql.Parse(q)
		if err != nil {
			t.Fatal(err)
		}
		if !metricsql.IsLikelyInvalid(e) {
			t.Fatalf("unexpected result for metricsql.IsLikelyInvalid(%q); got false; want true", q)
		}
	}

	f("rate(sum(http_total))")
	f("rate(rate(http_total))")
	f("sum(rate(sum(http_total)))")
	f("rate(sum(rate(http_total)))")
	f("rate(sum(sum(http_total)))")
	f("avg_over_time(rate(http_total[5m]))")

	f("rate(sum(http_total)) - rate(sum(http_total))")
	f("avg_over_time(rate(http_total)-rate(http_total))")

	// These queries are from https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3996
	f("sum_over_time(up{cluster='a'} or up{cluster='b'})")
	f("sum_over_time(up{cluster='a'}[1m] or up{cluster='b'}[1m])")
	f("sum(sum_over_time(up{cluster='a'}[1m] or up{cluster='b'}[1m])) by (instance)")

	f(`
WITH (
    cpuSeconds = node_cpu_seconds_total{instance=~"$node:$port",job=~"$job"},
    cpuIdle = rate(cpuSeconds{mode='idle'}[5m])
)
max_over_time(cpuIdle)`)
}
