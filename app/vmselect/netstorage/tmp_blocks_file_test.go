package netstorage

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	tmpDir := "TestTmpBlocks"
	InitTmpBlocksDir(tmpDir)
	statusCode := m.Run()
	if err := os.RemoveAll(tmpDir); err != nil {
		logger.Panicf("cannot remove %q: %s", tmpDir, err)
	}
	os.Exit(statusCode)
}

func TestTmpBlocksFileSerial(t *testing.T) {
	if err := testTmpBlocksFile(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestTmpBlocksFileConcurrent(t *testing.T) {
	concurrency := 3
	ch := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			ch <- testTmpBlocksFile()
		}()
	}
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("timeout")
		}
	}
}

func testTmpBlocksFile() error {
	createBlock := func() *storage.Block {
		rowsCount := rand.Intn(8000) + 1
		var timestamps, values []int64
		ts := int64(rand.Intn(1023434))
		for i := 0; i < rowsCount; i++ {
			ts += int64(rand.Intn(1000) + 1)
			timestamps = append(timestamps, ts)
			values = append(values, int64(i*i+rand.Intn(20)))
		}
		tsid := &storage.TSID{
			MetricID: 234211,
		}
		scale := int16(rand.Intn(123))
		precisionBits := uint8(rand.Intn(63) + 1)
		var b storage.Block
		b.Init(tsid, timestamps, values, scale, precisionBits)
		_, _, _ = b.MarshalData(0, 0)
		return &b
	}
	tr := storage.TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: 1<<63 - 1,
	}
	for _, size := range []int{1024, 16 * 1024, maxInmemoryTmpBlocksFile() / 2, 2 * maxInmemoryTmpBlocksFile()} {
		err := func() error {
			tbf := getTmpBlocksFile()
			defer putTmpBlocksFile(tbf)

			// Write blocks until their summary size exceeds `size`.
			var addrs []tmpBlockAddr
			var blocks []*storage.Block
			bb := tmpBufPool.Get()
			defer tmpBufPool.Put(bb)
			for tbf.offset < uint64(size) {
				b := createBlock()
				bb.B = storage.MarshalBlock(bb.B[:0], b)
				addr, err := tbf.WriteBlockData(bb.B)
				if err != nil {
					return fmt.Errorf("cannot write block at offset %d: %w", tbf.offset, err)
				}
				if addr.offset+uint64(addr.size) != tbf.offset {
					return fmt.Errorf("unexpected addr=%+v for offset %v", &addr, tbf.offset)
				}
				addrs = append(addrs, addr)
				blocks = append(blocks, b)
			}
			if err := tbf.Finalize(); err != nil {
				return fmt.Errorf("cannot finalize tbf: %w", err)
			}

			// Read blocks in parallel and verify them
			concurrency := 2
			workCh := make(chan int)
			doneCh := make(chan error)
			for i := 0; i < concurrency; i++ {
				go func() {
					doneCh <- func() error {
						var b1 storage.Block
						for idx := range workCh {
							addr := addrs[idx]
							b := blocks[idx]
							if err := b.UnmarshalData(); err != nil {
								return fmt.Errorf("cannot unmarshal data from the original block: %w", err)
							}
							b1.Reset()
							tbf.MustReadBlockAt(&b1, addr)
							if err := b1.UnmarshalData(); err != nil {
								return fmt.Errorf("cannot unmarshal data from tbf: %w", err)
							}
							if b1.RowsCount() != b.RowsCount() {
								return fmt.Errorf("unexpected number of rows in tbf block; got %d; want %d", b1.RowsCount(), b.RowsCount())
							}
							timestamps1, values1 := b1.AppendRowsWithTimeRangeFilter(nil, nil, tr)
							timestamps, values := b.AppendRowsWithTimeRangeFilter(nil, nil, tr)
							if !reflect.DeepEqual(timestamps1, timestamps) {
								return fmt.Errorf("unexpected timestamps; got\n%v\nwant\n%v", timestamps1, timestamps)
							}
							if !reflect.DeepEqual(values1, values) {
								return fmt.Errorf("unexpected values; got\n%v\nwant\n%v", values1, values)
							}
						}
						return nil
					}()
				}()
			}
			for i := range addrs {
				workCh <- i
			}
			close(workCh)
			for i := 0; i < concurrency; i++ {
				select {
				case err := <-doneCh:
					if err != nil {
						return err
					}
				case <-time.After(time.Second):
					return fmt.Errorf("timeout")
				}
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
