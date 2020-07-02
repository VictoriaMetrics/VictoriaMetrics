package opentsdbhttp

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentsdbhttp"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = tenantmetrics.NewCounterMap(`vm_rows_inserted_total{type="opentsdb-http"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="opentsdbhttp"}`)
)

// InsertHandler processes HTTP OpenTSDB put requests.
// See http://opentsdb.net/docs/build/html/api_http/put.html
func InsertHandler(req *http.Request) error {
	path := req.URL.Path
	p, err := httpserver.ParsePath(path)
	if err != nil {
		return fmt.Errorf("cannot parse path %q: %w", path, err)
	}
	if p.Prefix != "insert" {
		// This is not our link.
		return fmt.Errorf("unexpected path requested on HTTP OpenTSDB server: %q", path)
	}
	at, err := auth.NewToken(p.AuthToken)
	if err != nil {
		return fmt.Errorf("auth error: %w", err)
	}
	switch p.Suffix {
	case "api/put", "opentsdb/api/put":
		return writeconcurrencylimiter.Do(func() error {
			return parser.ParseStream(req, func(rows []parser.Row) error {
				return insertRows(at, rows)
			})
		})
	default:
		return fmt.Errorf("unexpected path requested on HTTP OpenTSDB server: %q", path)
	}
}

func insertRows(at *auth.Token, rows []parser.Row) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	for i := range rows {
		r := &rows[i]
		ctx.Labels = ctx.Labels[:0]
		ctx.AddLabel("", r.Metric)
		for j := range r.Tags {
			tag := &r.Tags[j]
			ctx.AddLabel(tag.Key, tag.Value)
		}
		ctx.ApplyRelabeling()
		if len(ctx.Labels) == 0 {
			// Skip metric without labels.
			continue
		}
		if err := ctx.WriteDataPoint(at, ctx.Labels, r.Timestamp, r.Value); err != nil {
			return err
		}
	}
	rowsInserted.Get(at).Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return ctx.FlushBufs()
}
