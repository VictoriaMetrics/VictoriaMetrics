package datasource

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const sqPath = "/sql"

type sqlColumMeta struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type sqlResponse struct {
	Meta []sqlColumMeta      `json:"meta"`
	Data [][]json.RawMessage `json:"data"`
	Row  int                 `json:"row"`
}

func parseSQLNumericValue(raw json.RawMessage) (float64, error) {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, fmt.Errorf("cannot parse %q as numeric value", string(raw))
	}
	return strconv.ParseFloat(s, 64)
}

func normalizeSQLType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	for {
		switch {
		case strings.HasPrefix(t, "nullable(") && strings.HasSuffix(t, ")"):
			t = strings.TrimSuffix(strings.TrimPrefix(t, "nullable("), ")")
		case strings.HasPrefix(t, "lowcardinality(") && strings.HasSuffix(t, ")"):
			t = strings.TrimSuffix(strings.TrimPrefix(t, "lowcardinality("), ")")
		default:
			return t
		}
	}
}

func isNumericSQLType(t string) bool {
	t = normalizeSQLType(t)
	switch {
	case strings.HasPrefix(t, "int"),
		strings.HasPrefix(t, "uint"),
		strings.HasPrefix(t, "float"),
		strings.HasPrefix(t, "decimal"),
		strings.HasPrefix(t, "numeric"),
		strings.HasPrefix(t, "double"),
		strings.HasPrefix(t, "real"),
		strings.HasPrefix(t, "number"):
		return true
	default:
		return false
	}
}

func sqlEvalTimestamp(resp *http.Response) (int64, error) {
	if resp.Request == nil {
		return 0, fmt.Errorf("missing request in SQL response")
	}
	raw := resp.Request.URL.Query().Get("time")
	if raw == "" {
		return 0, fmt.Errorf("missing time query param in SQL request")
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return 0, fmt.Errorf("cannot parse SQL evaluation time %q: %w", raw, err)
	}
	return t.Unix(), nil
}
func valueColumnIndex(meta []sqlColumMeta) (int, error) {
	idx := -1
	for i, col := range meta {
		if strings.EqualFold(col.Name, "value") {
			if !isNumericSQLType(col.Type) {
				return -1, fmt.Errorf(`column "value" must be numeric, got %q`, col.Type)
			}
			if idx != -1 {
				return -1, fmt.Errorf(`multiple columns named "value"`)
			}
			idx = i
		}
	}
	if idx == -1 {
		return -1, fmt.Errorf(`SQL response must contain exactly one numeric column named "value"`)
	}
	return idx, nil
}

func parseSQLResponse(resp *http.Response) (Result, error) {
	var r sqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Result{}, fmt.Errorf("error parsing SQL response: %w", err)
	}
	if len(r.Meta) == 0 {
		return Result{}, fmt.Errorf("SQL response has no columns")
	}
	valueIdx, err := valueColumnIndex(r.Meta)
	if err != nil {
		return Result{}, err
	}
	if valueIdx < 0 {
		return Result{}, fmt.Errorf("SQL response has no numeric column to use as metric value")
	}
	ts, err := sqlEvalTimestamp(resp)
	if err != nil {
		return Result{}, err
	}
	var metrics []Metric
	for _, row := range r.Data {
		if len(row) != len(r.Meta) {
			return Result{}, fmt.Errorf("SQL row has %d values, expected %d columns", len(row), len(r.Meta))
		}
		var m Metric
		val, err := parseSQLNumericValue(row[valueIdx])
		if err != nil {
			return Result{}, fmt.Errorf("error parsing value column %q: %w", r.Meta[valueIdx], err)
		}
		m.Values = []float64{val}
		m.Timestamps = []int64{ts}
		for i, col := range r.Meta {
			if i == valueIdx {
				continue
			}
			var labelVal string
			if err := json.Unmarshal(row[i], &labelVal); err != nil {
				var numVal float64
				if err := json.Unmarshal(row[i], &numVal); err != nil {
					return Result{}, fmt.Errorf("error parsing label column %q: %w", col.Name, err)
				}
				labelVal = strconv.FormatFloat(numVal, 'f', -1, 64)
			}
			m.AddLabel(col.Name, labelVal)
		}
		metrics = append(metrics, m)
	}
	return Result{Data: metrics}, nil
}
func (c *Client) setSQLInstantReqParams(r *http.Request, query string, timestamp time.Time) {
	if !*disablePathAppend {
		r.URL.Path += sqPath
	}
	q := r.URL.Query()
	q.Set("time", timestamp.Format(time.RFC3339))
	r.URL.RawQuery = q.Encode()
	c.setReqParams(r, query)
}
