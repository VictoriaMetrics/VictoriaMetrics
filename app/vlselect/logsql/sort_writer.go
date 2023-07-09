package logsql

import (
	"bytes"
	"io"
	"sort"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logjson"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func getSortWriter() *sortWriter {
	v := sortWriterPool.Get()
	if v == nil {
		return &sortWriter{}
	}
	return v.(*sortWriter)
}

func putSortWriter(sw *sortWriter) {
	sw.reset()
	sortWriterPool.Put(sw)
}

var sortWriterPool sync.Pool

// sortWriter expects JSON line stream to be written to it.
//
// It buffers the incoming data until its size reaches maxBufLen.
// Then it streams the buffered data and all the incoming data to w.
//
// The FinalFlush() must be called when all the data is written.
// If the buf isn't empty at FinalFlush() call, then the buffered data
// is sorted by _time field.
type sortWriter struct {
	mu         sync.Mutex
	w          io.Writer
	maxBufLen  int
	buf        []byte
	bufFlushed bool

	hasErr bool
}

func (sw *sortWriter) reset() {
	sw.w = nil
	sw.maxBufLen = 0
	sw.buf = sw.buf[:0]
	sw.bufFlushed = false
	sw.hasErr = false
}

func (sw *sortWriter) Init(w io.Writer, maxBufLen int) {
	sw.reset()

	sw.w = w
	sw.maxBufLen = maxBufLen
}

func (sw *sortWriter) MustWrite(p []byte) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.hasErr {
		return
	}

	if sw.bufFlushed {
		if _, err := sw.w.Write(p); err != nil {
			sw.hasErr = true
		}
		return
	}
	if len(sw.buf)+len(p) < sw.maxBufLen {
		sw.buf = append(sw.buf, p...)
		return
	}
	sw.bufFlushed = true
	if len(sw.buf) > 0 {
		if _, err := sw.w.Write(sw.buf); err != nil {
			sw.hasErr = true
			return
		}
		sw.buf = sw.buf[:0]
	}
	if _, err := sw.w.Write(p); err != nil {
		sw.hasErr = true
	}
}

func (sw *sortWriter) FinalFlush() {
	if sw.hasErr || sw.bufFlushed {
		return
	}
	rs := getRowsSorter()
	rs.parseRows(sw.buf)
	rs.sort()
	WriteJSONRows(sw.w, rs.rows)
	putRowsSorter(rs)
}

func getRowsSorter() *rowsSorter {
	v := rowsSorterPool.Get()
	if v == nil {
		return &rowsSorter{}
	}
	return v.(*rowsSorter)
}

func putRowsSorter(rs *rowsSorter) {
	rs.reset()
	rowsSorterPool.Put(rs)
}

var rowsSorterPool sync.Pool

type rowsSorter struct {
	buf       []byte
	fieldsBuf []logstorage.Field
	rows      [][]logstorage.Field
	times     []string
}

func (rs *rowsSorter) reset() {
	rs.buf = rs.buf[:0]

	fieldsBuf := rs.fieldsBuf
	for i := range fieldsBuf {
		fieldsBuf[i].Reset()
	}
	rs.fieldsBuf = fieldsBuf[:0]

	rows := rs.rows
	for i := range rows {
		rows[i] = nil
	}
	rs.rows = rows[:0]

	times := rs.times
	for i := range times {
		times[i] = ""
	}
	rs.times = times[:0]
}

func (rs *rowsSorter) parseRows(src []byte) {
	rs.reset()

	buf := rs.buf
	fieldsBuf := rs.fieldsBuf
	rows := rs.rows
	times := rs.times

	p := logjson.GetParser()
	for len(src) > 0 {
		var line []byte
		n := bytes.IndexByte(src, '\n')
		if n < 0 {
			line = src
			src = nil
		} else {
			line = src[:n]
			src = src[n+1:]
		}
		if len(line) == 0 {
			continue
		}

		if err := p.ParseLogMessage(line); err != nil {
			logger.Panicf("BUG: unexpected invalid JSON line: %s", err)
		}

		timeValue := ""
		fieldsBufLen := len(fieldsBuf)
		for _, f := range p.Fields {
			bufLen := len(buf)
			buf = append(buf, f.Name...)
			name := bytesutil.ToUnsafeString(buf[bufLen:])

			bufLen = len(buf)
			buf = append(buf, f.Value...)
			value := bytesutil.ToUnsafeString(buf[bufLen:])

			fieldsBuf = append(fieldsBuf, logstorage.Field{
				Name:  name,
				Value: value,
			})

			if name == "_time" {
				timeValue = value
			}
		}
		rows = append(rows, fieldsBuf[fieldsBufLen:])
		times = append(times, timeValue)
	}
	logjson.PutParser(p)

	rs.buf = buf
	rs.fieldsBuf = fieldsBuf
	rs.rows = rows
	rs.times = times
}

func (rs *rowsSorter) Len() int {
	return len(rs.rows)
}

func (rs *rowsSorter) Less(i, j int) bool {
	times := rs.times
	return times[i] < times[j]
}

func (rs *rowsSorter) Swap(i, j int) {
	times := rs.times
	rows := rs.rows
	times[i], times[j] = times[j], times[i]
	rows[i], rows[j] = rows[j], rows[i]
}

func (rs *rowsSorter) sort() {
	sort.Sort(rs)
}
