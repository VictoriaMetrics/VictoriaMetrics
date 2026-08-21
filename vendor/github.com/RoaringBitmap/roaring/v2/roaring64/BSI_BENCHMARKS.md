# BSI64 Benchmarks

These notes capture local benchmark results for the BSI64 `BatchEqual` and
comparison paths. They are intended as reproducible PR evidence, not as
contractual performance guarantees.

Environment:

- CPU: 12th Gen Intel(R) Core(TM) i7-1255U
- OS/arch: linux/amd64
- Package: `github.com/RoaringBitmap/roaring/v2/roaring64`

Commands:

```sh
go test ./roaring64 -count=1
go test ./roaring64 -run '^$' -bench 'BenchmarkBSI64BatchEqual' -benchmem -count 3
go test ./roaring64 -run '^$' -bench 'BenchmarkBSI64Compare(Big)?Value|BenchmarkBSI64BatchEqual(Big)?LargeAgeFixture' -benchmem -count 1
go test ./roaring64 -run '^$' -bench 'BenchmarkBSI64CompareBSISameRow' -benchmem -count=5
go test ./roaring64 -run '^$' -bench 'BenchmarkBSI64GetBigValue' -benchmem -count=3
go test ./roaring64 -run '^$' -bench 'BenchmarkBSI64BatchEqual.*LargeFixture' -benchmem -benchtime=2s -count=5
```

Representative results:

| Benchmark | Before | After | Notes |
| --- | ---: | ---: | --- |
| `BenchmarkBSI64BatchEqualLargeAgeFixture` | ~13-14s/op, ~12.4GB/op | ~145-205ms/op, ~25.5MB/op | Avoids row-by-row `GetBigValue` for int64-width values. |
| `BenchmarkBSI64BatchEqualM128Scattered` | ~1.25s/op, ~458MB/op | ~11-17ms/op, ~12.5MB/op | Detects complete bit-cube value patterns. |
| `BenchmarkBSI64CompareValueEQLargeAgeFixture` | ~4.44s/op, ~461MB/op | ~100-118ms/op, ~19.7MB/op | `EQ` delegates to optimized `BatchEqual`. |
| `BenchmarkBSI64CompareValueRangeLargeAgeFixture` | ~7.49s/op, ~501MB/op | ~204-224ms/op, ~122.6MB/op | Uses bitmap-native signed int64 comparison. |
| `BenchmarkBSI64CompareValueGELargeAgeFixture` | ~3.45s/op, ~500MB/op | ~168-184ms/op, ~82.3MB/op | Uses bitmap-native signed int64 comparison. |
| `BenchmarkBSI64CompareBSISameRowBitwise` | ~127-168ms/op, ~69.7MB/op | ~568-795us/op, ~619KB/op | Compares two BSI values per column ID through bitplane algebra instead of row-by-row `GetBigValue`. |
| `BenchmarkBSI64GetBigValuesLargeFixture` | ~69-92ms/op, ~35.6MB/op, ~1.3M allocs/op for a row-by-row `GetBigValue` loop | ~23-34ms/op, ~8.2MB/op, ~200k allocs/op | Extracts aligned BSI values for a column batch by walking bit-slices once. |
| `BenchmarkBSI64BatchEqualValuesLargeFixture` | ~5.4-7.1ms/op for `BatchEqual` plus `GetBigValues`; ~10.9-13.0ms/op for `BatchEqual` plus row-by-row `GetValue` | ~1.6-2.3ms/op, ~2.0MB/op, ~432 allocs/op | Emits matched column IDs and int64 values directly from trie leaves, avoiding a second value lookup pass. |

Compatibility:

- Public method signatures are unchanged.
- `CompareBigValue` and `BatchEqualBig` internally delegate to the optimized
  int64 paths only when the BSI and query values fit in signed 64-bit space.
- True wider-than-64-bit values continue to use the existing generic paths.
- `BatchEqualBig` now keys values by sign and magnitude so positive and negative
  values with the same magnitude do not collide.
- `GetBigValues` returns values aligned to the requested column IDs, with nil
  entries for missing values, while preserving `GetBigValue` semantics.
- `BatchEqualValues` returns matched column IDs and int64 values for `BatchEqual`
  shapes, optionally restricted by a found set. Result order is intentionally
  unspecified.

Follow-up:

- This change is scoped to `roaring64`. The 32-bit `BitSliceIndexing` package
  already has separate `BatchEqual` coverage, and `CompareValue` parity can be
  addressed in a follow-up PR with its own benchmarks and signed-value tests.
