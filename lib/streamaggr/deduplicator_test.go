package streamaggr

import (
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestDeduplicator(t *testing.T) {
	var tssResult []prompbmarshal.TimeSeries
	var tssResultLock sync.Mutex
	pushFunc := func(tss []prompbmarshal.TimeSeries) {
		tssResultLock.Lock()
		tssResult = appendClonedTimeseries(tssResult, tss)
		tssResultLock.Unlock()
	}

	tss := mustParsePromMetrics(`
foo{instance="x",job="aaa",pod="sdfd-dfdfdfs",node="aosijjewrerfd",namespace="asdff",container="ohohffd"} 123
bar{instance="x",job="aaa",pod="sdfd-dfdfdfs",node="aosijjewrerfd",namespace="asdff",container="ohohffd"} 34.54
x 8943 1
baz_aaa_aaa_fdd{instance="x",job="aaa",pod="sdfd-dfdfdfs",node="aosijjewrerfd",namespace="asdff",container="ohohffd"} -34.34
x 90984
x 433 1
asfjkldsf{instance="x",job="aaa",pod="sdfd-dfdfdfs",node="aosijjewrerfd",namespace="asdff",container="ohohffd"} 12322
foo{instance="x",job="aaa",pod="sdfd-dfdfdfs",node="aosijjewrerfd",namespace="asdff",container="ohohffd"} 894
baz_aaa_aaa_fdd{instance="x",job="aaa",pod="sdfd-dfdfdfs",node="aosijjewrerfd",namespace="asdff",container="ohohffd"} -2.3
`)

	dedupInterval := time.Hour
	d := NewDeduplicator(pushFunc, dedupInterval, []string{"node", "instance"}, "global")
	for i := 0; i < 10; i++ {
		d.Push(tss)
	}

	flushTime := time.Now()
	flushIntervals := flushTime.UnixMilli()/dedupInterval.Milliseconds() + 1
	idx := int(flushIntervals % int64(aggrStateSize))

	d.flush(pushFunc, time.Hour, time.Now(), idx)
	d.MustStop()

	result := timeSeriessToString(tssResult)
	resultExpected := `asfjkldsf{container="ohohffd",job="aaa",namespace="asdff",pod="sdfd-dfdfdfs"} 12322
bar{container="ohohffd",job="aaa",namespace="asdff",pod="sdfd-dfdfdfs"} 34.54
baz_aaa_aaa_fdd{container="ohohffd",job="aaa",namespace="asdff",pod="sdfd-dfdfdfs"} -2.3
foo{container="ohohffd",job="aaa",namespace="asdff",pod="sdfd-dfdfdfs"} 894
x 8943
`
	if result != resultExpected {
		t.Fatalf("unexpected result; got\n%s\nwant\n%s", result, resultExpected)
	}
}
