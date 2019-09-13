package promql

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestExpandWithExprsSuccess(t *testing.T) {
	f := func(q, qExpected string) {
		t.Helper()
		for i := 0; i < 3; i++ {
			qExpanded, err := ExpandWithExprs(q)
			if err != nil {
				t.Fatalf("unexpected error when expanding %q: %s", q, err)
			}
			if qExpanded != qExpected {
				t.Fatalf("unexpected expanded expression for %q;\ngot\n%q\nwant\n%q", q, qExpanded, qExpected)
			}
		}
	}

	f(`1`, `1`)
	f(`foobar`, `foobar`)
	f(`with (x = 1) x+x`, `2`)
	f(`with (f(x) = x*x) 3+f(2)+2`, `9`)
}

func TestExpandWithExprsError(t *testing.T) {
	f := func(q string) {
		t.Helper()
		for i := 0; i < 3; i++ {
			qExpanded, err := ExpandWithExprs(q)
			if err == nil {
				t.Fatalf("expecting non-nil error when expanding %q", q)
			}
			if qExpanded != "" {
				t.Fatalf("unexpected non-empty qExpanded=%q", qExpanded)
			}
		}
	}

	f(``)
	f(`  with (`)
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
			Start:    start,
			End:      end,
			Step:     step,
			Deadline: netstorage.NewDeadline(time.Minute),
		}
		for i := 0; i < 5; i++ {
			result, err := Exec(ec, q, false)
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
		q := `scalar("fooobar")`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run("scalar-string-num", func(t *testing.T) {
		q := `scalar("-12.34")`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{-12.34, -12.34, -12.34, -12.34, -12.34, -12.34},
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
			Values:     []float64{900, 1100, 1300, 1500, 1700, 1900},
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
			Values:     []float64{900, 1100, 1300, 1500, 1700, 1900},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{910, 1110, 1310, 1510, 1710, 1910},
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
	t.Run("time() offset 100s", func(t *testing.T) {
		t.Parallel()
		q := `time() offset 100s`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{800, 1000, 1200, 1400, 1600, 1800},
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
			Values:     []float64{860, 1060, 1260, 1460, 1660, 1860},
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
			Values:     []float64{300, 500, 700, 900, 1100, 1300},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{360, 560, 760, 960, 1160, 1360},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("baz"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
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
			Values:     []float64{900, 1100, 1300, 1500, 1700, 1900},
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
	t.Run(`absent(scalar(multi-timeseries))`, func(t *testing.T) {
		t.Parallel()
		q := `
		absent(label_set(scalar(1 or label_set(2, "xx", "foo")), "yy", "foo"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
		r.MetricName.Tags = []storage.Tag{{
			Key:   []byte("yy"),
			Value: []byte("foo"),
		}}
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
		q := `exp(time()/1e3)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{2.718281828459045, 3.3201169227365472, 4.0551999668446745, 4.953032424395115, 6.0496474644129465, 7.38905609893065},
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
	t.Run("time()*-1^0.5", func(t *testing.T) {
		t.Parallel()
		q := `time()*-1^0.5`
		resultExpected := []netstorage.Result{}
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
	t.Run(`label_value()`, func(t *testing.T) {
		t.Parallel()
		q := `with (
			x = (
				label_set(time(), "foo", "123.456", "__name__", "aaa"),
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
			Values:     []float64{1123.456, 1323.456, 1523.456, 1723.456, 1923.456, 2123.456},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("123.456"),
		}}
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
	t.Run(`label_replace(mismatch)`, func(t *testing.T) {
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
	t.Run(`time() unless 2`, func(t *testing.T) {
		t.Parallel()
		q := `time() unless 2`
		resultExpected := []netstorage.Result{}
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
	t.Run(`scalar * ignoring(foo) group_right vector`, func(t *testing.T) {
		t.Parallel()
		q := `sort_desc(2 * ignoring(foo) group_right(a,foo) (label_set(time(), "foo", "bar") or label_set(10, "foo", "qwert")))`
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
			(label_set(time(), "t1", "v123", "t2", "v3") or label_set(10, "t2", "v3", "xxx", "yy"))
			+ on (foo, t2) group_left (t1, noxxx)
			(label_set(100, "t1", "v1") or label_set(time(), "t2", "v3", "noxxx", "aa"))
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
	t.Run(`histogram_quantile(single-value-no-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6, label_set(100, "foo", "bar"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`histogram_quantile(single-value-invalid-le)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6, label_set(100, "le", "foobar"))`
		resultExpected := []netstorage.Result{}
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
	t.Run(`histogram_quantile(nan-bucket-count)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6,
			label_set(90, "foo", "bar", "le", "10")
			or label_set(NaN, "foo", "bar", "le", "30")
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
	t.Run(`histogram_quantile(nan-bucket-count)`, func(t *testing.T) {
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
	t.Run(`histogram_quantile(nan-bucket-count)`, func(t *testing.T) {
		t.Parallel()
		q := `histogram_quantile(0.6,
			label_set(nan, "foo", "bar", "le", "10")
			or label_set(nan, "foo", "bar", "le", "30")
			or label_set(nan, "foo", "bar", "le", "+Inf")
		)`
		resultExpected := []netstorage.Result{}
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
			Values:     []float64{6.8, 8.8, 10.9, 12.9, 14.9, 16.9},
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
			Values:     []float64{155, 251, 371, 515, 683, 875},
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
	t.Run(`limitk(-1)`, func(t *testing.T) {
		t.Parallel()
		q := `limitk(-1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`limitk(1)`, func(t *testing.T) {
		t.Parallel()
		q := `limitk(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
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
	t.Run(`topk(-1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(topk(-1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		resultExpected := []netstorage.Result{}
		f(q, resultExpected)
	})
	t.Run(`topk(1)`, func(t *testing.T) {
		t.Parallel()
		q := `sort(topk(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		resultExpected := []netstorage.Result{r1, r2}
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
		q := `sort(bottomk(1, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss")))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, nan, nan, nan},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("baz"),
			Value: []byte("sss"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{nan, nan, nan, 10, 10, 10},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("foo"),
			Value: []byte("bar"),
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
	t.Run(`quantile(-2)`, func(t *testing.T) {
		t.Parallel()
		q := `quantile(-2, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10, 10, 10},
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
			Values:     []float64{6.666666666666667, 8, 9.333333333333334, 10, 10, 10},
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
			Values:     []float64{10, 10, 10, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`median()`, func(t *testing.T) {
		t.Parallel()
		q := `median(label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10.666666666666666, 12, 13.333333333333334},
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
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10.666666666666666, 12, 13.333333333333334},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`quantile(NaN)`, func(t *testing.T) {
		t.Parallel()
		q := `quantile(NaN, label_set(10, "foo", "bar") or label_set(time()/150, "baz", "sss"))`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{10, 10, 10, 10.666666666666666, 12, 13.333333333333334},
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
			Values:     []float64{1600, 1600, 1600, 1600, 1600, 1600},
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
			Values:     []float64{1600, 1600, 1600, 1600, 1600, 1600},
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
		q := `ru(time() offset 1i, 2000)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{65, 55.00000000000001, 45, 35, 25, 15},
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
		q := `integrate(time()*1e-3)`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{160, 200, 240.00000000000003, 280, 320, 360},
			Timestamps: timestampsExpected,
		}
		resultExpected := []netstorage.Result{r}
		f(q, resultExpected)
	})
	t.Run(`rate(time())`, func(t *testing.T) {
		t.Parallel()
		q := `rate(time())`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{1, 1, 1, 1, 1, 1},
			Timestamps: timestampsExpected,
		}
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
			Values:     []float64{5.5, 4.5, 3.5, 2.5, 1.5, 0.5},
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
			Values:     []float64{5.5, 4.5, 3.5, 2.5, 1.5, 0.5},
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
			Values:     []float64{5.5, 4.5, 6.5, 4.5, 2.5, 0.5},
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
			Values:     []float64{6, 5, 7.5, 5.5, 3.5, 1.5},
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
			Values:     []float64{7, 6, 5, 7.5, 5.5, 3.5},
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
			Values:     []float64{1100, 900, 700, 500, 300, 100},
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
		q := `remove_resets( abs(1500-time()) )`
		r := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{500, 800, 900, 900, 1100, 1300},
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
	t.Run(`deriv(1)`, func(t *testing.T) {
		t.Parallel()
		q := `deriv(1)`
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
	t.Run(`rollup_candlestick()`, func(t *testing.T) {
		t.Parallel()
		q := `sort(rollup_candlestick(round(rand(0),0.01)[:10s]))`
		r1 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.02, 0.02, 0.03, 0, 0.03, 0.02},
			Timestamps: timestampsExpected,
		}
		r1.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("low"),
		}}
		r2 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.32, 0.82, 0.13, 0.28, 0.86, 0.57},
			Timestamps: timestampsExpected,
		}
		r2.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("close"),
		}}
		r3 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.9, 0.32, 0.82, 0.13, 0.28, 0.86},
			Timestamps: timestampsExpected,
		}
		r3.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("open"),
		}}
		r4 := netstorage.Result{
			MetricName: metricNameExpected,
			Values:     []float64{0.85, 0.94, 0.97, 0.93, 0.98, 0.92},
			Timestamps: timestampsExpected,
		}
		r4.MetricName.Tags = []storage.Tag{{
			Key:   []byte("rollup"),
			Value: []byte("high"),
		}}
		resultExpected := []netstorage.Result{r1, r2, r3, r4}
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
		resultExpected := []netstorage.Result{r1, r2, r3, r4}
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
}

func TestExecError(t *testing.T) {
	f := func(q string) {
		t.Helper()
		ec := &EvalConfig{
			Start:    1000,
			End:      2000,
			Step:     100,
			Deadline: netstorage.NewDeadline(time.Minute),
		}
		for i := 0; i < 4; i++ {
			rv, err := Exec(ec, q, false)
			if err == nil {
				t.Fatalf(`expecting non-nil error on %q`, q)
			}
			if rv != nil {
				t.Fatalf(`expecting nil rv`)
			}
			rv, err = Exec(ec, q, true)
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
	f(`range_quantile()`)
	f(`range_quantile(1, 2, 3)`)
	f(`range_median()`)
	f(`abs()`)
	f(`abs(1,2)`)
	f(`absent(1, 2)`)
	f(`clamp_max()`)
	f(`clamp_min(1,2,3)`)
	f(`hour(1,2)`)
	f(`label_join()`)
	f(`label_replace(1)`)
	f(`label_transform(1)`)
	f(`label_set()`)
	f(`label_set(1, "foo")`)
	f(`label_del()`)
	f(`label_keep()`)
	f(`round()`)
	f(`round(1,2,3)`)
	f(`scalar()`)
	f(`sort(1,2)`)
	f(`sort_desc()`)
	f(`timestamp()`)
	f(`vector()`)
	f(`histogram_quantile()`)
	f(`sum()`)
	f(`count_values()`)
	f(`quantile()`)
	f(`topk()`)
	f(`limitk()`)
	f(`bottomk()`)
	f(`time(123)`)
	f(`start(1)`)
	f(`end(1)`)
	f(`step(1)`)
	f(`running_sum(1, 2)`)
	f(`range_sum(1, 2)`)
	f(`range_first(1,  2)`)
	f(`range_last(1, 2)`)
	f(`smooth_exponential()`)
	f(`smooth_exponential(1)`)
	f(`remove_resets()`)
	f(`sin()`)
	f(`cos()`)
	f(`asin()`)
	f(`acos()`)
	f(`rand(123, 456)`)
	f(`rand_normal(123, 456)`)
	f(`rand_exponential(122, 456)`)
	f(`pi(123)`)
	f(`label_copy()`)
	f(`label_move()`)
	f(`median_over_time()`)
	f(`median()`)
	f(`median("foo", "bar")`)
	f(`keep_last_value()`)
	f(`distinct_over_time()`)
	f(`distinct()`)
	f(`alias()`)
	f(`alias(1)`)
	f(`alias(1, "foo", "bar")`)

	// Invalid argument type
	f(`median_over_time({}, 2)`)
	f(`smooth_exponential(1, 1 or label_set(2, "x", "y"))`)
	f(`count_values(1, 2)`)
	f(`count_values(1 or label_set(2, "xx", "yy"), 2)`)
	f(`quantile(1 or label_set(2, "xx", "foo"), 1)`)
	f(`clamp_max(1, 1 or label_set(2, "xx", "foo"))`)
	f(`clamp_min(1, 1 or label_set(2, "xx", "foo"))`)
	f(`topk(label_set(2, "xx", "foo") or 1, 12)`)
	f(`limitk(label_set(2, "xx", "foo") or 1, 12)`)
	f(`round(1, 1 or label_set(2, "xx", "foo"))`)
	f(`histogram_quantile(1 or label_set(2, "xx", "foo"), 1)`)
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
	f(`alias(1, 2)`)

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
	f(`sum(1, 2)`)
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
}

func testResultsEqual(t *testing.T, result, resultExpected []netstorage.Result) {
	t.Helper()
	if len(result) != len(resultExpected) {
		t.Fatalf(`unexpected timeseries count; got %d; want %d`, len(result), len(resultExpected))
	}
	for i := range result {
		r := &result[i]
		rExpected := &resultExpected[i]
		testMetricNamesEqual(t, &r.MetricName, &rExpected.MetricName)
		testRowsEqual(t, r.Values, r.Timestamps, rExpected.Values, rExpected.Timestamps)
	}
}

func testMetricNamesEqual(t *testing.T, mn, mnExpected *storage.MetricName) {
	t.Helper()
	if string(mn.MetricGroup) != string(mnExpected.MetricGroup) {
		t.Fatalf(`unexpected MetricGroup; got %q; want %q`, mn.MetricGroup, mnExpected.MetricGroup)
	}
	if len(mn.Tags) != len(mnExpected.Tags) {
		t.Fatalf(`unexpected tags count; got %d; want %d`, len(mn.Tags), len(mnExpected.Tags))
	}
	for i := range mn.Tags {
		tag := &mn.Tags[i]
		tagExpected := &mnExpected.Tags[i]
		if string(tag.Key) != string(tagExpected.Key) {
			t.Fatalf(`unexpected tag key; got %q; want %q`, tag.Key, tagExpected.Key)
		}
		if string(tag.Value) != string(tagExpected.Value) {
			t.Fatalf(`unexpected tag value; got %q; want %q`, tag.Value, tagExpected.Value)
		}
	}
}
