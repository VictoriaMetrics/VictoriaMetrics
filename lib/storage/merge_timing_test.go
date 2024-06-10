package storage

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
)

func BenchmarkMergeBlockStreamsTwoSourcesWorstCase(b *testing.B) {
	benchmarkMergeBlockStreams(b, benchTwoSourcesWorstCaseMPS, benchTwoSourcesWorstCaseMPSRowsPerLoop)
}

func BenchmarkMergeBlockStreamsTwoSourcesBestCase(b *testing.B) {
	benchmarkMergeBlockStreams(b, benchTwoSourcesBestCaseMPS, benchTwoSourcesBestCaseMPSRowsPerLoop)
}

func BenchmarkMergeBlockStreamsFourSourcesWorstCase(b *testing.B) {
	benchmarkMergeBlockStreams(b, benchFourSourcesWorstCaseMPS, benchFourSourcesWorstCaseMPSRowsPerLoop)
}

func BenchmarkMergeBlockStreamsFourSourcesBestCase(b *testing.B) {
	benchmarkMergeBlockStreams(b, benchFourSourcesBestCaseMPS, benchFourSourcesBestCaseMPSRowsPerLoop)
}

func benchmarkMergeBlockStreams(b *testing.B, mps []*inmemoryPart, rowsPerLoop int64) {
	var rowsMerged, rowsDeleted atomic.Uint64
	strg := newTestStorage()

	b.ReportAllocs()
	b.SetBytes(rowsPerLoop)
	b.RunParallel(func(pb *testing.PB) {
		var bsw blockStreamWriter
		var mpOut inmemoryPart
		bsrs := make([]*blockStreamReader, len(mps))
		for i := range mps {
			var bsr blockStreamReader
			bsrs[i] = &bsr
		}
		for pb.Next() {
			for i, mp := range mps {
				bsrs[i].MustInitFromInmemoryPart(mp)
			}
			mpOut.Reset()
			bsw.MustInitFromInmemoryPart(&mpOut, -5)
			if err := mergeBlockStreams(&mpOut.ph, &bsw, bsrs, nil, strg, 0, &rowsMerged, &rowsDeleted); err != nil {
				panic(fmt.Errorf("cannot merge block streams: %w", err))
			}
		}
	})

	stopTestStorage(strg)
}

var benchTwoSourcesWorstCaseMPS = func() []*inmemoryPart {
	rng := rand.New(rand.NewSource(1))
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = defaultPrecisionBits
	for i := 0; i < maxRowsPerBlock/2-1; i++ {
		r.Value = rng.NormFloat64()
		r.Timestamp = rng.Int63n(1e12)
		rows = append(rows, r)
	}
	mp := newTestInmemoryPart(rows)
	return []*inmemoryPart{mp, mp}
}()

const benchTwoSourcesWorstCaseMPSRowsPerLoop = (maxRowsPerBlock - 2)

var benchTwoSourcesBestCaseMPS = func() []*inmemoryPart {
	var r rawRow
	var mps []*inmemoryPart
	for i := 0; i < 2; i++ {
		var rows []rawRow
		r.PrecisionBits = defaultPrecisionBits
		r.TSID.MetricID = uint64(i)
		for j := 0; j < maxRowsPerBlock; j++ {
			rows = append(rows, r)
		}
		mp := newTestInmemoryPart(rows)
		mps = append(mps, mp)
	}
	return mps
}()

const benchTwoSourcesBestCaseMPSRowsPerLoop = 2 * maxRowsPerBlock

var benchFourSourcesWorstCaseMPS = func() []*inmemoryPart {
	rng := rand.New(rand.NewSource(1))
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = defaultPrecisionBits
	for i := 0; i < maxRowsPerBlock/2-1; i++ {
		r.Value = rng.NormFloat64()
		r.Timestamp = rng.Int63n(1e12)
		rows = append(rows, r)
	}
	mp := newTestInmemoryPart(rows)
	return []*inmemoryPart{mp, mp, mp, mp}
}()

const benchFourSourcesWorstCaseMPSRowsPerLoop = 2 * (maxRowsPerBlock - 2)

var benchFourSourcesBestCaseMPS = func() []*inmemoryPart {
	var r rawRow
	var mps []*inmemoryPart
	for i := 0; i < 4; i++ {
		var rows []rawRow
		r.PrecisionBits = defaultPrecisionBits
		r.TSID.MetricID = uint64(i)
		for j := 0; j < maxRowsPerBlock; j++ {
			rows = append(rows, r)
		}
		mp := newTestInmemoryPart(rows)
		mps = append(mps, mp)
	}
	return mps
}()

const benchFourSourcesBestCaseMPSRowsPerLoop = 4 * maxRowsPerBlock
