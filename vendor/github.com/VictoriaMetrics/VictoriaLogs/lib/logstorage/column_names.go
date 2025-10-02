package logstorage

import (
	"fmt"
	"io"
	"math"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func mustWriteColumnIdxs(w *writerWithStats, columnIdxs map[uint64]uint64) {
	data := marshalColumnIdxs(nil, columnIdxs)
	w.MustWrite(data)
}

func mustReadColumnIdxs(r filestream.ReadCloser, columnNames []string, shardsCount uint64) map[string]uint64 {
	src, err := io.ReadAll(r)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot read column indexes: %s", r.Path(), err)
	}

	columnIdxs, err := unmarshalColumnIdxs(src, columnNames, shardsCount)
	if err != nil {
		logger.Panicf("FATAL: %s: cannot parse column indexes: %s", r.Path(), err)
	}

	return columnIdxs
}

func marshalColumnIdxs(dst []byte, columnIdxs map[uint64]uint64) []byte {
	dst = encoding.MarshalVarUint64(dst, uint64(len(columnIdxs)))
	for columnID, shardIdx := range columnIdxs {
		dst = encoding.MarshalVarUint64(dst, columnID)
		dst = encoding.MarshalVarUint64(dst, shardIdx)
	}
	return dst
}

func unmarshalColumnIdxs(src []byte, columnNames []string, shardsCount uint64) (map[string]uint64, error) {
	n, nBytes := encoding.UnmarshalVarUint64(src)
	if nBytes <= 0 {
		return nil, fmt.Errorf("cannot parse the number of entries from len(src)=%d", len(src))
	}
	src = src[nBytes:]
	if n > math.MaxInt {
		return nil, fmt.Errorf("too many entries: %d; mustn't exceed %d", n, math.MaxInt)
	}

	shardIdxs := make(map[string]uint64, n)
	for i := uint64(0); i < n; i++ {
		columnID, nBytes := encoding.UnmarshalVarUint64(src)
		if nBytes <= 0 {
			return nil, fmt.Errorf("cannot parse columnID #%d", i)
		}
		src = src[nBytes:]

		shardIdx, nBytes := encoding.UnmarshalVarUint64(src)
		if nBytes <= 0 {
			return nil, fmt.Errorf("cannot parse shardIdx #%d", i)
		}
		if shardIdx >= shardsCount {
			return nil, fmt.Errorf("too big shardIdx=%d; must be smaller than %d", shardIdx, shardsCount)
		}
		src = src[nBytes:]

		if columnID >= uint64(len(columnNames)) {
			return nil, fmt.Errorf("too big columnID; got %d; must be smaller than %d", columnID, len(columnNames))
		}
		columnName := columnNames[columnID]
		shardIdxs[columnName] = shardIdx
	}
	if len(src) > 0 {
		return nil, fmt.Errorf("unexpected tail left after reading column indexes; len(tail)=%d", len(src))
	}

	return shardIdxs, nil
}

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
		return nil, nil, fmt.Errorf("too many distinct column names: %d; mustn't exceed %d", n, math.MaxInt)
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
	if ok {
		return id
	}
	if g.columnNameIDs == nil {
		g.columnNameIDs = make(map[string]uint64)
	}
	id = uint64(len(g.columnNames))

	// it is better to intern the column name instead of cloning it with string.Clone,
	// since the number of column names is usually small (e.g. less than 10K).
	// This reduces memory allocations.
	nameCopy := bytesutil.InternString(name)

	g.columnNameIDs[nameCopy] = id
	g.columnNames = append(g.columnNames, nameCopy)
	return id
}
