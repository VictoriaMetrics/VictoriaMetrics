package logstorage

import (
	"fmt"
	"io"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func mustWriteColumnNames(w *writerWithStats, columnNames []string) {
	data := marshalColumnNames(nil, columnNames)
	w.MustWrite(data)
}

func mustReadColumnNames(r filestream.ReadCloser) ([]string, map[string]uint64) {
	src, err := io.ReadAll(r)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot read colum names: %s", r.Path(), err)
	}

	columnNames, err := unmarshalColumnNames(src)
	if err != nil {
		logger.Panicf("FATAL: %s: %s", r.Path(), err)
	}

	columnNameIDs, err := getColumnNameIDs(columnNames)
	if err != nil {
		logger.Panicf("BUG: %s: %s; columnNames=%v", r.Path(), err, columnNameIDs)
	}

	return columnNames, columnNameIDs
}

func getColumnNameIDs(columnNames []string) (map[string]uint64, error) {
	m := make(map[uint64]string, len(columnNames))
	columnNameIDs := make(map[string]uint64, len(columnNames))
	for i, name := range columnNames {
		id := uint64(i)
		if prevName, ok := m[id]; ok {
			return nil, fmt.Errorf("duplicate column name id=%d for columns %q and %q", id, prevName, name)
		}
		m[id] = name
		columnNameIDs[name] = id
	}

	return columnNameIDs, nil
}

func marshalColumnNames(dst []byte, columnNames []string) []byte {
	data := encoding.MarshalVarUint64(nil, uint64(len(columnNames)))

	for _, name := range columnNames {
		data = encoding.MarshalBytes(data, bytesutil.ToUnsafeBytes(name))
	}

	dst = encoding.CompressZSTDLevel(dst, data, 1)

	return dst
}

func unmarshalColumnNames(src []byte) ([]string, error) {
	data, err := encoding.DecompressZSTD(nil, src)
	if err != nil {
		return nil, fmt.Errorf("cannot decompress column names from len(src)=%d: %w", len(src), err)
	}
	src = data

	n, nBytes := encoding.UnmarshalVarUint64(src)
	if nBytes <= 0 {
		return nil, fmt.Errorf("cannot parse the number of column names for len(src)=%d", len(src))
	}
	src = src[nBytes:]

	m := make(map[string]uint64, n)
	columnNames := make([]string, n)
	for id := uint64(0); id < n; id++ {
		name, nBytes := encoding.UnmarshalBytes(src)
		if nBytes <= 0 {
			return nil, fmt.Errorf("cannot parse colum name number %d out of %d", id, n)
		}
		src = src[nBytes:]

		if idPrev, ok := m[string(name)]; ok {
			return nil, fmt.Errorf("duplicate ids for column name %q: %d and %d", name, idPrev, id)
		}

		m[string(name)] = id
		columnNames[id] = string(name)
	}

	if len(src) > 0 {
		return nil, fmt.Errorf("unexpected non-empty tail left after unmarshaling column name ids; len(tail)=%d", len(src))
	}

	return columnNames, nil
}

type columnNameIDGenerator struct {
	// columnNameIDs contains columnName->id mapping for already seen columns
	columnNameIDs map[string]uint64

	// columnNames contains id->columnName mapping for already seen columns
	columnNames []string
}

func (g *columnNameIDGenerator) reset() {
	g.columnNameIDs = nil
	g.columnNames = nil
}

func (g *columnNameIDGenerator) getColumnNameID(name string) uint64 {
	id, ok := g.columnNameIDs[name]
	if !ok {
		if g.columnNameIDs == nil {
			g.columnNameIDs = make(map[string]uint64)
		}
		id = uint64(len(g.columnNames))
		nameCopy := strings.Clone(name)
		g.columnNameIDs[nameCopy] = id
		g.columnNames = append(g.columnNames, nameCopy)
	}
	return id
}
