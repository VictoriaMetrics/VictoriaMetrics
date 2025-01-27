package logstorage

import (
	"fmt"
	"io"
	"math"
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
		logger.Panicf("FATAL: %s: cannot read column names: %s", r.Path(), err)
	}

	columnNames, columnNameIDs, err := unmarshalColumnNames(src)
	if err != nil {
		logger.Panicf("FATAL: %s: %s", r.Path(), err)
	}

	return columnNames, columnNameIDs
}

func marshalColumnNames(dst []byte, columnNames []string) []byte {
	data := encoding.MarshalVarUint64(nil, uint64(len(columnNames)))
	data = marshalStrings(data, columnNames)

	dst = encoding.CompressZSTDLevel(dst, data, 1)

	return dst
}

func unmarshalColumnNames(src []byte) ([]string, map[string]uint64, error) {
	data, err := encoding.DecompressZSTD(nil, src)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot decompress column names from len(src)=%d: %w", len(src), err)
	}
	src = data

	n, nBytes := encoding.UnmarshalVarUint64(src)
	if nBytes <= 0 {
		return nil, nil, fmt.Errorf("cannot parse the number of column names for len(src)=%d", len(src))
	}
	src = src[nBytes:]
	if n > math.MaxInt {
		return nil, nil, fmt.Errorf("too many distinct column names: %d; musn't exceed %d", n, math.MaxInt)
	}

	columnNameIDs := make(map[string]uint64, n)
	columnNames := make([]string, n)

	for id := uint64(0); id < n; id++ {
		name, nBytes := encoding.UnmarshalBytes(src)
		if nBytes <= 0 {
			return nil, nil, fmt.Errorf("cannot parse column name number %d out of %d", id, n)
		}
		src = src[nBytes:]

		// It should be good idea to intern column names, since usually the number of unique column names is quite small,
		// even for wide events (e.g. less than a few thousands). So, if the average length of the column name
		// exceeds 8 bytes (this is a typical case for Kubernetes with long column names), then interning saves some RAM.
		nameStr := bytesutil.InternBytes(name)

		if idPrev, ok := columnNameIDs[nameStr]; ok {
			return nil, nil, fmt.Errorf("duplicate ids for column name %q: %d and %d", name, idPrev, id)
		}

		columnNameIDs[nameStr] = id
		columnNames[id] = nameStr
	}

	if len(src) > 0 {
		return nil, nil, fmt.Errorf("unexpected non-empty tail left after unmarshaling column name ids; len(tail)=%d", len(src))
	}

	return columnNames, columnNameIDs, nil
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
