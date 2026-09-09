package promremotewrite

import (
	"flag"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/promremotewrite/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeserieslimits"
)

var (
	rowsInserted     = metrics.NewCounter(`vm_rows_inserted_total{type="promremotewrite"}`)
	rowsPerInsert    = metrics.NewHistogram(`vm_rows_per_insert{type="promremotewrite"}`)
	metadataInserted = metrics.NewCounter(`vm_metadata_rows_inserted_total{type="promremotewrite"}`)

	sortLabels = flag.Bool("sortLabels", false, `Whether to sort labels for incoming samples before writing them to storage. `+
		`This may be needed for reducing memory usage at storage when the order of labels in incoming samples is random. `+
		`For example, if m{k1="v1",k2="v2"} may be sent as m{k2="v2",k1="v1"}. `+
		`Enabled sorting for labels can slow down ingestion performance a bit`)
)

// Storage is used for writing Prometheus remote write data into vmstorage.
type Storage interface {
	WriteRows(rows []storage.MetricRow) error
	WriteMetadata(rows []metricsmetadata.Row) error
	IsReadOnly() bool
}

// InsertHandler processes Prometheus remote write requests.
func InsertHandler(at *auth.Token, req *http.Request, s Storage) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	isVMRemoteWrite := req.Header.Get("Content-Encoding") == "zstd"
	return stream.Parse(req.Body, isVMRemoteWrite, func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error {
		return insertRows(s, at, tss, mms, extraLabels)
	})
}

func insertRows(s Storage, at *auth.Token, tss []prompb.TimeSeries, mms []prompb.MetricMetadata, extraLabels []prompb.Label) error {
	if s.IsReadOnly() {
		return storage.ErrReadOnly
	}
	ctx := getCtx()
	defer putCtx(ctx)

	rowsLen := 0
	for i := range tss {
		rowsLen += len(tss[i].Samples)
	}
	if rowsLen > 0 {
		ctx.rows = ctx.rows[:0]
		if cap(ctx.rows) < rowsLen {
			ctx.rows = make([]storage.MetricRow, 0, rowsLen)
		}
		ctx.metricNameBuf = ctx.metricNameBuf[:0]
		for i := range tss {
			ts := &tss[i]
			ctx.labels = ctx.labels[:0]
			for j := range ts.Labels {
				label := &ts.Labels[j]
				ctx.addLabel(label.Name, label.Value)
			}
			for j := range extraLabels {
				label := &extraLabels[j]
				ctx.addLabel(label.Name, label.Value)
			}
			atLocal := ctx.getLocalAuthToken(at)
			if !ctx.tryPrepareLabels() {
				continue
			}
			var metricNameRaw []byte
			for j := range ts.Samples {
				sample := &ts.Samples[j]
				if len(metricNameRaw) == 0 {
					metricNameRaw = ctx.marshalMetricNameRaw(atLocal, ctx.labels)
				}
				if err := ctx.addRow(s, metricNameRaw, sample.Timestamp, sample.Value); err != nil {
					return err
				}
				if len(ctx.metricNameBuf) == 0 {
					metricNameRaw = nil
				}
			}
		}

		rowsInserted.Add(rowsLen)
		rowsPerInsert.Update(float64(rowsLen))
		if err := ctx.flushRows(s); err != nil {
			return err
		}
	}

	if prommetadata.IsEnabled() && len(mms) > 0 {
		ctx.metadataRows = ctx.metadataRows[:0]
		if cap(ctx.metadataRows) < len(mms) {
			ctx.metadataRows = make([]metricsmetadata.Row, 0, len(mms))
		}
		for i := range mms {
			mm := &mms[i]
			if timeserieslimits.IsMetricMetadataExceeding(mm) {
				continue
			}
			atLocal := ctx.getLocalAuthTokenForMetadata(at, mm)
			ctx.metadataRows = append(ctx.metadataRows, metricsmetadata.Row{
				MetricFamilyName: []byte(mm.MetricFamilyName),
				Help:             []byte(mm.Help),
				Unit:             []byte(mm.Unit),
				AccountID:        atLocal.AccountID,
				ProjectID:        atLocal.ProjectID,
				Type:             mm.Type,
			})
		}
		metadataInserted.Add(len(ctx.metadataRows))
		if err := s.WriteMetadata(ctx.metadataRows); err != nil {
			return fmt.Errorf("cannot write metadata: %w", err)
		}
	}

	return nil
}

type ctx struct {
	rows          []storage.MetricRow
	metadataRows  []metricsmetadata.Row
	labels        []prompb.Label
	metricNameBuf []byte
	at            auth.Token
}

func (ctx *ctx) reset() {
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

func getCtx() *ctx {
	if v := ctxPool.Get(); v != nil {
		return v.(*ctx)
	}
	return &ctx{}
}

func putCtx(ctx *ctx) {
	ctx.reset()
	ctxPool.Put(ctx)
}

var ctxPool sync.Pool

func (ctx *ctx) marshalMetricNameRaw(at *auth.Token, labels []prompb.Label) []byte {
	start := len(ctx.metricNameBuf)
	ctx.metricNameBuf = storage.MarshalMetricNameRaw(ctx.metricNameBuf, at.AccountID, at.ProjectID, labels)
	metricNameRaw := ctx.metricNameBuf[start:]
	return metricNameRaw[:len(metricNameRaw):len(metricNameRaw)]
}

func (ctx *ctx) addRow(s Storage, metricNameRaw []byte, timestamp int64, value float64) error {
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

func (ctx *ctx) flushRows(s Storage) error {
	if len(ctx.rows) == 0 {
		return nil
	}
	if err := s.WriteRows(ctx.rows); err != nil {
		return fmt.Errorf("cannot write rows: %w", err)
	}
	clear(ctx.rows)
	ctx.rows = ctx.rows[:0]
	ctx.metricNameBuf = ctx.metricNameBuf[:0]
	return nil
}

func (ctx *ctx) addLabel(name, value string) {
	if len(value) == 0 {
		return
	}
	ctx.labels = append(ctx.labels, prompb.Label{
		Name:  name,
		Value: value,
	})
}

func (ctx *ctx) tryPrepareLabels() bool {
	if len(ctx.labels) == 0 {
		return false
	}
	if timeserieslimits.Enabled() && timeserieslimits.IsExceeding(ctx.labels) {
		return false
	}
	ctx.sortLabelsIfNeeded()
	return true
}

func (ctx *ctx) sortLabelsIfNeeded() {
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

func (ctx *ctx) getLocalAuthToken(at *auth.Token) *auth.Token {
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

func (ctx *ctx) getLocalAuthTokenForMetadata(at *auth.Token, mm *prompb.MetricMetadata) *auth.Token {
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
