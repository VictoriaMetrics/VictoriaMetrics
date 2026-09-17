package promremotewrite

import (
	"errors"
	"flag"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeserieslimits"
)

var sortLabels = flag.Bool("sortLabels", false, `Whether to sort labels for samples ingested via vmstorage ingestion APIs when -enableIngestionAPI is enabled. `+
	`This may be needed for reducing memory usage at storage when the order of labels in incoming samples is random. `+
	`For example, if m{k1="v1",k2="v2"} may be sent as m{k2="v2",k1="v1"}. `+
	`Enabled sorting for labels can slow down ingestion performance a bit`)

// InsertCtx is used for inserting Prometheus remote write data into vmstorage.
type InsertCtx struct {
	rows          []storage.MetricRow
	metadataRows  []metricsmetadata.Row
	labels        []prompb.Label
	metricNameBuf []byte
	at            auth.Token
}

func (ctx *InsertCtx) reset() {
	clear(ctx.rows)
	ctx.rows = ctx.rows[:0]

	for i := range ctx.metadataRows {
		ctx.metadataRows[i] = metricsmetadata.Row{}
	}
	ctx.metadataRows = ctx.metadataRows[:0]

	promrelabel.CleanLabels(ctx.labels)
	ctx.labels = ctx.labels[:0]

	ctx.metricNameBuf = ctx.metricNameBuf[:0]
	ctx.at.Set(0, 0)
}

func getInsertCtx() *InsertCtx {
	if v := insertCtxPool.Get(); v != nil {
		return v.(*InsertCtx)
	}
	return &InsertCtx{}
}

func putInsertCtx(ctx *InsertCtx) {
	ctx.reset()
	insertCtxPool.Put(ctx)
}

var insertCtxPool sync.Pool

func (ctx *InsertCtx) marshalMetricNameRaw(at *auth.Token, labels []prompb.Label) []byte {
	start := len(ctx.metricNameBuf)
	ctx.metricNameBuf = storage.MarshalMetricNameRaw(ctx.metricNameBuf, at.AccountID, at.ProjectID, labels)
	metricNameRaw := ctx.metricNameBuf[start:]
	return metricNameRaw[:len(metricNameRaw):len(metricNameRaw)]
}

func (ctx *InsertCtx) addRow(s Storage, metricNameRaw []byte, timestamp int64, value float64) error {
	ctx.rows = append(ctx.rows, storage.MetricRow{
		MetricNameRaw: metricNameRaw,
		Timestamp:     timestamp,
		Value:         value,
	})
	if len(ctx.metricNameBuf) > 16*1024*1024 {
		if err := ctx.flushRows(s); err != nil {
			return err
		}
	}
	return nil
}

func (ctx *InsertCtx) flushRows(s Storage) error {
	if len(ctx.rows) == 0 {
		return nil
	}
	if err := s.WriteRows(ctx.rows); err != nil {
		if errors.Is(err, storage.ErrReadOnly) {
			return errReadOnly()
		}
		return fmt.Errorf("cannot write rows: %w", err)
	}
	clear(ctx.rows)
	ctx.rows = ctx.rows[:0]
	ctx.metricNameBuf = ctx.metricNameBuf[:0]
	return nil
}

func (ctx *InsertCtx) addLabel(name, value string) {
	if len(value) == 0 {
		return
	}
	ctx.labels = append(ctx.labels, prompb.Label{
		Name:  name,
		Value: value,
	})
}

func (ctx *InsertCtx) tryPrepareLabels() bool {
	if len(ctx.labels) == 0 {
		return false
	}
	if timeserieslimits.Enabled() && timeserieslimits.IsExceeding(ctx.labels) {
		return false
	}
	ctx.sortLabelsIfNeeded()
	return true
}

func (ctx *InsertCtx) sortLabelsIfNeeded() {
	if *sortLabels {
		sort.Sort((*sortedLabels)(&ctx.labels))
	}
}

type sortedLabels []prompb.Label

func (sl *sortedLabels) Len() int { return len(*sl) }
func (sl *sortedLabels) Less(i, j int) bool {
	a := *sl
	return a[i].Name < a[j].Name
}
func (sl *sortedLabels) Swap(i, j int) {
	a := *sl
	a[i], a[j] = a[j], a[i]
}

func (ctx *InsertCtx) getLocalAuthToken(at *auth.Token) *auth.Token {
	if at != nil {
		return at
	}
	accountID := uint32(0)
	projectID := uint32(0)
	srcLabels := ctx.labels
	dstLabels := srcLabels[:0]
	for _, label := range srcLabels {
		switch label.Name {
		case "vm_account_id":
			accountID = parseUint32(label.Value)
			continue
		case "vm_project_id":
			projectID = parseUint32(label.Value)
			continue
		case "VictoriaMetrics_AccountID":
			accountID = parseUint32(label.Value)
		case "VictoriaMetrics_ProjectID":
			projectID = parseUint32(label.Value)
		}
		dstLabels = append(dstLabels, label)
	}
	clear(srcLabels[len(dstLabels):])
	ctx.labels = dstLabels
	ctx.at.Set(accountID, projectID)
	return &ctx.at
}

func (ctx *InsertCtx) getLocalAuthTokenForMetadata(at *auth.Token, mm *prompb.MetricMetadata) *auth.Token {
	if at != nil {
		return at
	}
	ctx.at.Set(mm.AccountID, mm.ProjectID)
	return &ctx.at
}

func parseUint32(s string) uint32 {
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n)
}
