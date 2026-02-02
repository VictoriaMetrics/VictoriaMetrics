/*
# Instructions for smat testing for roaring

[smat](https://github.com/mschoch/smat) is a framework that provides
state machine assisted fuzz testing.

To run the smat tests for roaring...

## Prerequisites

Go 1.18 or later (for native fuzzing support).

## Steps

1. Generate initial smat corpus:
```
go test -tags=gofuzz -run=TestGenerateSmatCorpus
```
You should see a directory `workdir` created with initial corpus files.

2. Run the fuzz test:
```
go test -run='^$' -fuzz=FuzzSmat -fuzztime=300s -timeout=60s
```

Adjust `-fuzztime` as needed for longer or shorter runs. If crashes are found,
check the test output and the reproducer files in the `workdir` directory.
You may copy the reproducers to roaring_tests.go
*/

package roaring

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/bits-and-blooms/bitset"
	"github.com/mschoch/smat"
)

// The native fuzz entry point lives in a _test.go file so the go test
// fuzz engine discovers it. See smat_fuzz_test.go for the fuzz wrapper.

var smatDebug = true

const max_value = 1048576
const max_pairs = 10

func smatLog(prefix, format string, args ...interface{}) {
	if smatDebug {
		fmt.Print(prefix)
		fmt.Printf(format, args...)
	}
}

type smatContext struct {
	pairs []*smatPair

	// Two registers, x & y.
	x int
	y int

	actions int
	// per-context last action for this fuzz worker
	lastAction *actionRecord
}

// actionRecord stores a snapshot of the state just before an action runs.
type actionRecord struct {
	Name          string
	X, Y          int
	PairSnapshots []string // base64-encoded MarshalBinary of each pair's Bitmap
}

type smatPair struct {
	bm *Bitmap
	bs *bitset.BitSet
	// parent context (nil if unknown)
	ctx *smatContext
}

// ------------------------------------------------------------------

var smatActionMap = smat.ActionMap{
	smat.ActionID('X'): smatAction("x++", smatWrap(func(c *smatContext) { c.x = (c.x + 1) % max_value })),
	smat.ActionID('x'): smatAction("x--", smatWrap(func(c *smatContext) { c.x = (c.x - 1 + max_value) % max_value })),
	smat.ActionID('Y'): smatAction("y++", smatWrap(func(c *smatContext) { c.y = (c.y + 1) % max_value })),
	smat.ActionID('y'): smatAction("y--", smatWrap(func(c *smatContext) { c.y = (c.y - 1 + max_value) % max_value })),
	smat.ActionID('*'): smatAction("x*y", smatWrap(func(c *smatContext) { c.x = (c.x * c.y) % max_value })),
	smat.ActionID('<'): smatAction("x<<", smatWrap(func(c *smatContext) { c.x = (c.x << 1) % max_value })),

	smat.ActionID('^'): smatAction("swap", smatWrap(func(c *smatContext) { c.x, c.y = c.y, c.x })),

	smat.ActionID('['): smatAction(" pushPair", smatWrap(smatPushPair)),
	smat.ActionID(']'): smatAction(" popPair", smatWrap(smatPopPair)),

	smat.ActionID('B'): smatAction(" setBit", smatWrap(smatSetBit)),
	smat.ActionID('b'): smatAction(" removeBit", smatWrap(smatRemoveBit)),

	smat.ActionID('o'): smatAction(" or", smatWrap(smatOr)),
	smat.ActionID('a'): smatAction(" and", smatWrap(smatAnd)),
	smat.ActionID('z'): smatAction(" xor", smatWrap(smatXor)),

	smat.ActionID('#'): smatAction(" cardinality", smatWrap(smatCardinality)),

	smat.ActionID('O'): smatAction(" orCardinality", smatWrap(smatOrCardinality)),
	smat.ActionID('A'): smatAction(" andCardinality", smatWrap(smatAndCardinality)),
	smat.ActionID('Z'): smatAction(" xorCardinality", smatWrap(smatXorCardinality)),

	smat.ActionID('c'): smatAction(" clear", smatWrap(smatClear)),
	smat.ActionID('r'): smatAction(" runOptimize", smatWrap(smatRunOptimize)),

	smat.ActionID('e'): smatAction(" isEmpty", smatWrap(smatIsEmpty)),

	smat.ActionID('i'): smatAction(" intersects", smatWrap(smatIntersects)),

	smat.ActionID('f'): smatAction(" flip", smatWrap(smatFlip)),

	smat.ActionID('-'): smatAction(" difference", smatWrap(smatDifference)),
}

var smatRunningPercentActions []smat.PercentAction

func init() {
	var ids []int
	for actionId := range smatActionMap {
		ids = append(ids, int(actionId))
	}
	sort.Ints(ids)

	pct := 100 / len(smatActionMap)
	for _, actionId := range ids {
		smatRunningPercentActions = append(smatRunningPercentActions,
			smat.PercentAction{Percent: pct, Action: smat.ActionID(actionId)})
	}

	smatActionMap[smat.ActionID('S')] = smatAction("SETUP", smatSetupFunc)
	smatActionMap[smat.ActionID('T')] = smatAction("TEARDOWN", smatTeardownFunc)
}

// We only have one smat state: running.
func smatRunning(next byte) smat.ActionID {
	return smat.PercentExecute(next, smatRunningPercentActions...)
}

func smatAction(name string, f func(ctx smat.Context) (smat.State, error)) func(smat.Context) (smat.State, error) {
	return func(ctx smat.Context) (smat.State, error) {
		c := ctx.(*smatContext)

		// Snapshot all pairs' bitmaps (base64 of MarshalBinary) before action
		rec := actionRecord{Name: name, X: c.x, Y: c.y}
		if len(c.pairs) > 0 {
			rec.PairSnapshots = make([]string, 0, len(c.pairs))
			for _, pair := range c.pairs {
				if pair == nil || pair.bm == nil {
					rec.PairSnapshots = append(rec.PairSnapshots, "<nil>")
					continue
				}
				b, err := pair.bm.MarshalBinary()
				if err != nil {
					rec.PairSnapshots = append(rec.PairSnapshots, "<marshal-error:"+err.Error()+">")
				} else {
					rec.PairSnapshots = append(rec.PairSnapshots, base64.StdEncoding.EncodeToString(b))
				}
			}
		}

		// record per-context last action (no global mutex required)
		if c != nil {
			c.lastAction = &rec
		}

		// catch panics inside action to dump a repro and stack before re-panicking
		defer func() {
			if r := recover(); r != nil {
				// best-effort: write quick repro with lastAction from context
				var lastAction *actionRecord
				if c != nil {
					lastAction = c.lastAction
				}
				ts := time.Now().UnixNano()
				repro := "// Reproducer generated by smat (panic)\n"
				repro += "package roaring\n\n"
				repro += "import (\n\t\"encoding/base64\"\n\t\"testing\"\n)\n\n"
				repro += fmt.Sprintf("func TestFuzzerPanicRepro_%d(t *testing.T) {\n", ts)
				// similar to checkEquals repro
				if lastAction != nil && len(lastAction.PairSnapshots) > 0 {
					pairIndex := lastAction.X % len(lastAction.PairSnapshots)
					if pairIndex < len(lastAction.PairSnapshots) {
						snapshot := lastAction.PairSnapshots[pairIndex]
						if snapshot != "<nil>" && !strings.HasPrefix(snapshot, "<") {
							repro += fmt.Sprintf("\tb, _ := base64.StdEncoding.DecodeString(\"%s\")\n", snapshot)
							repro += "\tbm := NewBitmap()\n"
							repro += "\tbm.UnmarshalBinary(b)\n"
							// perform the action that caused panic
							if strings.Contains(lastAction.Name, "setBit") {
								repro += fmt.Sprintf("\tbm.AddInt(%d)\n", lastAction.Y)
							} else if strings.Contains(lastAction.Name, "removeBit") {
								repro += fmt.Sprintf("\tbm.Remove(%d)\n", lastAction.Y)
							} else if strings.Contains(lastAction.Name, "flip") {
								repro += fmt.Sprintf("\tbm.Flip(uint64(%d), uint64(%d)+1)\n", lastAction.Y, lastAction.Y)
							} else if strings.Contains(lastAction.Name, "runOptimize") {
								repro += "\tbm.RunOptimize()\n"
							} else if strings.Contains(lastAction.Name, "clear") {
								repro += "\tbm.Clear()\n"
							} else if lastAction.Name == " or" {
								pairIndexY := lastAction.Y % len(lastAction.PairSnapshots)
								if pairIndexY < len(lastAction.PairSnapshots) {
									snapshotY := lastAction.PairSnapshots[pairIndexY]
									if snapshotY != "<nil>" && !strings.HasPrefix(snapshotY, "<") {
										repro += fmt.Sprintf("\tb2, _ := base64.StdEncoding.DecodeString(\"%s\")\n", snapshotY)
										repro += "\tbm2 := NewBitmap()\n"
										repro += "\tbm2.UnmarshalBinary(b2)\n"
										repro += "\tbm.Or(bm2)\n"
									}
								}
							} else if lastAction.Name == " and" {
								pairIndexY := lastAction.Y % len(lastAction.PairSnapshots)
								if pairIndexY < len(lastAction.PairSnapshots) {
									snapshotY := lastAction.PairSnapshots[pairIndexY]
									if snapshotY != "<nil>" && !strings.HasPrefix(snapshotY, "<") {
										repro += fmt.Sprintf("\tb2, _ := base64.StdEncoding.DecodeString(\"%s\")\n", snapshotY)
										repro += "\tbm2 := NewBitmap()\n"
										repro += "\tbm2.UnmarshalBinary(b2)\n"
										repro += "\tbm.And(bm2)\n"
									}
								}
							} else if lastAction.Name == " difference" {
								pairIndexY := lastAction.Y % len(lastAction.PairSnapshots)
								if pairIndexY < len(lastAction.PairSnapshots) {
									snapshotY := lastAction.PairSnapshots[pairIndexY]
									if snapshotY != "<nil>" && !strings.HasPrefix(snapshotY, "<") {
										repro += fmt.Sprintf("\tb2, _ := base64.StdEncoding.DecodeString(\"%s\")\n", snapshotY)
										repro += "\tbm2 := NewBitmap()\n"
										repro += "\tbm2.UnmarshalBinary(b2)\n"
										repro += "\tbm.AndNot(bm2)\n"
									}
								}
							} else if lastAction.Name == " xor" {
								pairIndexY := lastAction.Y % len(lastAction.PairSnapshots)
								if pairIndexY < len(lastAction.PairSnapshots) {
									snapshotY := lastAction.PairSnapshots[pairIndexY]
									if snapshotY != "<nil>" && !strings.HasPrefix(snapshotY, "<") {
										repro += fmt.Sprintf("\tb2, _ := base64.StdEncoding.DecodeString(\"%s\")\n", snapshotY)
										repro += "\tbm2 := NewBitmap()\n"
										repro += "\tbm2.UnmarshalBinary(b2)\n"
										repro += "\tbm.Xor(bm2)\n"
									}
								}
							} else {
								repro += fmt.Sprintf("\t// Unhandled action: %s\n", lastAction.Name)
							}
						} else {
							repro += "\t// invalid snapshot\n"
						}
					}
				}
				repro += "}\n"
				if path, werr := saveReproFile("smat_panic_repro", ts, repro); werr == nil {
					fmt.Printf("wrote panic repro to %s\n", path)
				} else {
					fmt.Printf("failed writing panic repro: %v\n", werr)
				}
				fmt.Printf("PANIC in action %s: %v\n", rec.Name, r)
				fmt.Printf("stack:\n%s\n", debug.Stack())
				panic(r)
			}
		}()

		c.actions++
		return f(ctx)
	}
}

// saveReproFile writes the given repro content to workdir/<prefix>_<ts>_test.go
// or falls back to the OS temp dir. Returns full path or error.
func saveReproFile(prefix string, ts int64, content string) (string, error) {
	// try workdir
	if err := os.MkdirAll("workdir", 0o755); err == nil {
		fname := fmt.Sprintf("workdir/%s_%d_test.go", prefix, ts)
		if err := os.WriteFile(fname, []byte(content), 0o644); err == nil {
			return fname, nil
		}
	}
	// fallback to temp
	tmp := os.TempDir()
	fname := fmt.Sprintf("%s_%d_test.go", prefix, ts)
	full := filepath.Join(tmp, fname)
	if err := os.WriteFile(full, []byte(content), 0o644); err == nil {
		return full, nil
	} else {
		return "", err
	}
}

// Creates an smat action func based on a simple callback.
func smatWrap(cb func(c *smatContext)) func(smat.Context) (next smat.State, err error) {
	return func(ctx smat.Context) (next smat.State, err error) {
		c := ctx.(*smatContext)
		cb(c)
		return smatRunning, nil
	}
}

// Invokes a callback function with the input v bounded to len(c.pairs).
func (c *smatContext) withPair(v int, cb func(*smatPair)) {
	if len(c.pairs) > 0 {
		if v < 0 {
			v = -v
		}
		v = v % len(c.pairs)
		cb(c.pairs[v])
	}
}

// ------------------------------------------------------------------

func smatSetupFunc(ctx smat.Context) (next smat.State, err error) {
	return smatRunning, nil
}

func smatTeardownFunc(ctx smat.Context) (next smat.State, err error) {
	return nil, err
}

// ------------------------------------------------------------------

func smatPushPair(c *smatContext) {
	if len(c.pairs) >= max_pairs {
		return
	}
	p := &smatPair{
		bm:  NewBitmap(),
		bs:  bitset.New(100),
		ctx: c,
	}
	c.pairs = append(c.pairs, p)
}

func smatPopPair(c *smatContext) {
	if len(c.pairs) > 0 {
		c.pairs = c.pairs[0 : len(c.pairs)-1]
	}
}

func smatSetBit(c *smatContext) {
	c.withPair(c.x, func(p *smatPair) {
		p.Validate()
		y := uint32(c.y)
		p.bm.AddInt(int(y))
		p.bs.Set(uint(y))
		p.checkEquals()
	})
}

func smatRemoveBit(c *smatContext) {
	c.withPair(c.x, func(p *smatPair) {
		p.Validate()
		y := uint32(c.y)
		p.bm.Remove(y)
		p.bs.Clear(uint(y))
		p.checkEquals()
	})
}

func smatAnd(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		c.withPair(c.y, func(py *smatPair) {
			px.Validate()
			py.Validate()
			px.bm.And(py.bm)
			px.bs = px.bs.Intersection(py.bs)
			px.checkEquals()
			py.checkEquals()
		})
	})
}

func smatOr(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		c.withPair(c.y, func(py *smatPair) {
			px.Validate()
			py.Validate()
			px.bm.Or(py.bm)
			px.bs = px.bs.Union(py.bs)
			px.checkEquals()
			py.checkEquals()
		})
	})
}

func smatXor(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		c.withPair(c.y, func(py *smatPair) {
			px.Validate()
			py.Validate()
			px.bm.Xor(py.bm)
			px.bs = px.bs.SymmetricDifference(py.bs)
			px.checkEquals()
			py.checkEquals()
		})
	})
}

func smatAndCardinality(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		c.withPair(c.y, func(py *smatPair) {
			px.Validate()
			py.Validate()
			c0 := px.bm.AndCardinality(py.bm)
			c1 := px.bs.IntersectionCardinality(py.bs)
			if c0 != uint64(c1) {
				panic("expected same add cardinality")
			}
			px.checkEquals()
			py.checkEquals()
		})
	})
}

func smatOrCardinality(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		c.withPair(c.y, func(py *smatPair) {
			px.Validate()
			py.Validate()
			c0 := px.bm.OrCardinality(py.bm)
			c1 := px.bs.UnionCardinality(py.bs)
			if c0 != uint64(c1) {
				panic("expected same or cardinality")
			}
			px.checkEquals()
			py.checkEquals()
		})
	})
}

func smatXorCardinality(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		c.withPair(c.y, func(py *smatPair) {
			px.Validate()
			py.Validate()
			c0 := px.bm.OrCardinality(py.bm) - px.bm.AndCardinality(py.bm)
			c1 := px.bs.SymmetricDifferenceCardinality(py.bs)
			if c0 != uint64(c1) {
				panic("expected same xor cardinality")
			}
			px.checkEquals()
			py.checkEquals()
		})
	})
}

func smatRunOptimize(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		px.Validate()
		px.bm.RunOptimize()
		px.checkEquals()
	})
}

func smatClear(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		px.Validate()
		px.bm.Clear()
		px.bs = px.bs.ClearAll()
		px.checkEquals()
	})
}

func smatCardinality(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		c0 := px.bm.GetCardinality()
		c1 := px.bs.Count()
		if c0 != uint64(c1) {
			panic("expected same cardinality")
		}
	})
}

func smatIsEmpty(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		c0 := px.bm.IsEmpty()
		c1 := px.bs.None()
		if c0 != c1 {
			panic("expected same is empty")
		}
	})
}

func smatIntersects(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		c.withPair(c.y, func(py *smatPair) {
			px.Validate()
			py.Validate()
			v0 := px.bm.Intersects(py.bm)
			v1 := px.bs.IntersectionCardinality(py.bs) > 0
			if v0 != v1 {
				panic("intersects not equal")
			}

			px.checkEquals()
			py.checkEquals()
		})
	})
}

func smatFlip(c *smatContext) {
	c.withPair(c.x, func(p *smatPair) {
		p.Validate()
		y := uint32(c.y)
		p.bm.Flip(uint64(y), uint64(y)+1)
		p.bs = p.bs.Flip(uint(y))
		p.checkEquals()
	})
}

func smatDifference(c *smatContext) {
	c.withPair(c.x, func(px *smatPair) {
		c.withPair(c.y, func(py *smatPair) {
			px.Validate()
			py.Validate()
			px.bm.AndNot(py.bm)
			px.bs = px.bs.Difference(py.bs)
			px.checkEquals()
			py.checkEquals()
		})
	})
}

func (p *smatPair) checkEquals() {
	valid := p.bm.Validate()
	if valid != nil {
		// marshal current bitmap
		var curSnap string
		if p != nil && p.bm != nil {
			if b, err := p.bm.MarshalBinary(); err == nil {
				curSnap = base64.StdEncoding.EncodeToString(b)
			} else {
				curSnap = "<marshal-error:" + err.Error() + ">"
			}
		} else {
			curSnap = "<nil>"
		}

		// collect last action summary from context (per-worker)
		last := "<none>"
		if p != nil && p.ctx != nil {
			c := p.ctx
			if c.lastAction != nil {
				last = fmt.Sprintf("action=%s x=%d y=%d pairs=%d", c.lastAction.Name, c.lastAction.X, c.lastAction.Y, len(c.lastAction.PairSnapshots))
			}
		}

		// If debugging enabled, log extra info
		smatLog("ERROR: ", "bitmap invalid: %v\n", valid)

		// build a reproducible test snippet that reconstructs the bitmap and replays the failing action
		ts := time.Now().UnixNano()
		testName := fmt.Sprintf("TestFuzzerRepro_%d", ts)
		repro := "// Reproducer generated by smat\n"
		repro += "package roaring\n\n"
		repro += "import (\n\t\"encoding/base64\"\n\t\"testing\"\n)\n\n"
		repro += fmt.Sprintf("func %s(t *testing.T) {\n", testName)
		var lastAction *actionRecord
		if p != nil && p.ctx != nil {
			lastAction = p.ctx.lastAction
		}
		// use the snapshot of the modified pair
		if lastAction != nil && len(lastAction.PairSnapshots) > 0 {
			// assume the modified pair is x % len(pairs), but since pairs are in order, and x is lastAction.X
			pairIndex := lastAction.X % len(lastAction.PairSnapshots)
			if pairIndex < len(lastAction.PairSnapshots) {
				snapshot := lastAction.PairSnapshots[pairIndex]
				if snapshot != "<nil>" && !strings.HasPrefix(snapshot, "<") {
					repro += fmt.Sprintf("\tb, _ := base64.StdEncoding.DecodeString(\"%s\")\n", snapshot)
					repro += "\tbm := NewBitmap()\n"
					repro += "\tbm.UnmarshalBinary(b)\n"
					repro += "\tif err := bm.Validate(); err != nil {\n"
					repro += "\t\tt.Errorf(\"Initial Validate failed: %v\", err)\n"
					repro += "\t}\n"
					// perform the action
					if strings.Contains(lastAction.Name, "setBit") {
						repro += fmt.Sprintf("\tbm.AddInt(%d)\n", lastAction.Y)
					} else if strings.Contains(lastAction.Name, "removeBit") {
						repro += fmt.Sprintf("\tbm.Remove(%d)\n", lastAction.Y)
					} else if strings.Contains(lastAction.Name, "flip") {
						repro += fmt.Sprintf("\tbm.Flip(uint64(%d), uint64(%d)+1)\n", lastAction.Y, lastAction.Y)
					} else if strings.Contains(lastAction.Name, "runOptimize") {
						repro += "\tbm.RunOptimize()\n"
					} else if strings.Contains(lastAction.Name, "clear") {
						repro += "\tbm.Clear()\n"
					} else if lastAction.Name == " or" {
						pairIndexY := lastAction.Y % len(lastAction.PairSnapshots)
						if pairIndexY < len(lastAction.PairSnapshots) {
							snapshotY := lastAction.PairSnapshots[pairIndexY]
							if snapshotY != "<nil>" && !strings.HasPrefix(snapshotY, "<") {
								repro += fmt.Sprintf("\tb2, _ := base64.StdEncoding.DecodeString(\"%s\")\n", snapshotY)
								repro += "\tbm2 := NewBitmap()\n"
								repro += "\tbm2.UnmarshalBinary(b2)\n"
								repro += "\tbm.Or(bm2)\n"
							}
						}
					} else if lastAction.Name == " and" {
						pairIndexY := lastAction.Y % len(lastAction.PairSnapshots)
						if pairIndexY < len(lastAction.PairSnapshots) {
							snapshotY := lastAction.PairSnapshots[pairIndexY]
							if snapshotY != "<nil>" && !strings.HasPrefix(snapshotY, "<") {
								repro += fmt.Sprintf("\tb2, _ := base64.StdEncoding.DecodeString(\"%s\")\n", snapshotY)
								repro += "\tbm2 := NewBitmap()\n"
								repro += "\tbm2.UnmarshalBinary(b2)\n"
								repro += "\tbm.And(bm2)\n"
							}
						}
					} else if lastAction.Name == " difference" {
						pairIndexY := lastAction.Y % len(lastAction.PairSnapshots)
						if pairIndexY < len(lastAction.PairSnapshots) {
							snapshotY := lastAction.PairSnapshots[pairIndexY]
							if snapshotY != "<nil>" && !strings.HasPrefix(snapshotY, "<") {
								repro += fmt.Sprintf("\tb2, _ := base64.StdEncoding.DecodeString(\"%s\")\n", snapshotY)
								repro += "\tbm2 := NewBitmap()\n"
								repro += "\tbm2.UnmarshalBinary(b2)\n"
								repro += "\tbm.AndNot(bm2)\n"
							}
						}
					} else if lastAction.Name == " xor" {
						pairIndexY := lastAction.Y % len(lastAction.PairSnapshots)
						if pairIndexY < len(lastAction.PairSnapshots) {
							snapshotY := lastAction.PairSnapshots[pairIndexY]
							if snapshotY != "<nil>" && !strings.HasPrefix(snapshotY, "<") {
								repro += fmt.Sprintf("\tb2, _ := base64.StdEncoding.DecodeString(\"%s\")\n", snapshotY)
								repro += "\tbm2 := NewBitmap()\n"
								repro += "\tbm2.UnmarshalBinary(b2)\n"
								repro += "\tbm.Xor(bm2)\n"
							}
						}
					} else {
						repro += fmt.Sprintf("\t// Unhandled action: %s\n", lastAction.Name)
					}
					repro += "\tif err := bm.Validate(); err != nil {\n"
					repro += "\t\tt.Errorf(\"Validate failed: %v\", err)\n"
					repro += "\t} else {\n"
					repro += "\t\tt.Logf(\"Validate succeeded\")\n"
					repro += "\t}\n"
				} else {
					repro += "\t// invalid snapshot\n"
				}
			}
		}
		repro += "}\n"

		// print the repro snippet for the developer
		fmt.Println()
		fmt.Println("=== SMAT REPRODUCER SNIPPET ===")
		if len(repro) > 10000 {
			fmt.Println("// Reproducer too large, skipping full print")
		} else {
			fmt.Println(repro)
		}

		// also write the repro snippet to a timestamped file in workdir/
		if len(repro) > 10000 {
			repro = "// Reproducer too large, skipping\n"
		}
		if err := os.MkdirAll("workdir", 0o755); err == nil {
			fname := fmt.Sprintf("workdir/smat_repro_%d_test.go", ts)
			if werr := os.WriteFile(fname, []byte(repro), 0o644); werr == nil {
				fmt.Printf("Wrote repro to %s\n", fname)
			} else {
				fmt.Printf("Failed writing repro file: %v\n", werr)
			}
		} else {
			fmt.Printf("Failed creating workdir: %v\n", err)
		}

		panic(fmt.Sprintf("[checkEquals] bitmap invalid: %v\ncurrentBase64:%s\nlastAction:%s\n", valid, curSnap, last))
	}
	if !p.equalsBitSet(p.bs, p.bm) {
		panic("bitset mismatch")
	}
}

func (p *smatPair) Validate() {
	valid := p.bm.Validate()
	if valid != nil {
		panic(fmt.Sprintf("[Validate] bitmap invalid: %v", valid))
	}
}

func (p *smatPair) equalsBitSet(a *bitset.BitSet, b *Bitmap) bool {
	for i, e := a.NextSet(0); e; i, e = a.NextSet(i + 1) {
		if !b.ContainsInt(int(i)) {
			fmt.Printf("in a bitset, not b bitmap, i: %d\n", i)
			fmt.Printf("  a bitset: %s\n  b bitmap: %s\n",
				a.String(), b.String())
			return false
		}
	}

	i := b.Iterator()
	for i.HasNext() {
		v := i.Next()
		if !a.Test(uint(v)) {
			fmt.Printf("in b bitmap, not a bitset, v: %d\n", v)
			fmt.Printf("  a bitset: %s\n  b bitmap: %s\n",
				a.String(), b.String())
			return false
		}
	}

	return true
}
