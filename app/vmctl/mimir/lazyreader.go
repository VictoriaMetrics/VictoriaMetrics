package mimir

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/oklog/ulid/v2"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/tombstones"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

var _ tsdb.BlockReader = (*lazyBlockReader)(nil)

// lazyBlockReader is stores block id and segment num information.
// It is used to lazily fetch and parse block data.
// It implements tsdb.BlockReader interface.
type lazyBlockReader struct {
	// Block ID.
	ID ulid.ULID
	// SegmentsNum stores the number of chunks segments in the block.
	SegmentsNum int

	mu          sync.Mutex
	reader      *tsdb.Block
	tempDirPath string
	fs          common.RemoteFS
	err         error
}

// newLazyBlockReader returns a new LazyBlockReader for the given block.
func newLazyBlockReader(block *Block, fs common.RemoteFS) (*lazyBlockReader, error) {
	if block.SegmentsFormat != "1b6d" {
		return nil, fmt.Errorf("unsupported segments format: %s", block.SegmentsFormat)
	}

	return &lazyBlockReader{
		ID:          block.ID,
		SegmentsNum: block.SegmentsNum,
		fs:          fs,
	}, nil
}

func (lbr *lazyBlockReader) initialize() error {
	lbr.mu.Lock()
	defer lbr.mu.Unlock()
	if lbr.reader != nil {
		return nil
	}
	// fetching block and parse it and store it in lbr.reader
	temp, err := lbr.mkTempDir()
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %s", err)
	}

	lbr.tempDirPath = temp

	// TODO: replace fetchFile and writeFile with buffered IO if needed
	meta, err := lbr.fetchFile(metaFilename)
	if err != nil {
		return err
	}
	if err := lbr.writeFile(temp, metaFilename, meta); err != nil {
		return fmt.Errorf("failed to write meta file: %w", err)
	}
	idx, err := lbr.fetchFile(indexFilename)
	if err != nil {
		return fmt.Errorf("failed to fetch index file %q: %w", indexFilename, err)
	}
	if err := lbr.writeFile(temp, indexFilename, idx); err != nil {
		return err
	}

	for i := 1; i <= lbr.SegmentsNum; i++ {
		// segments formats has format 1b06d
		// https://github.com/grafana/mimir/blob/main/pkg/storage/tsdb/bucketindex/index.go#L32
		chunkName := fmt.Sprintf("%06d", i)
		blockChunkPath := filepath.Join("chunks", chunkName)
		chunk, err := lbr.fetchFile(blockChunkPath)
		if err != nil {
			return fmt.Errorf("failed to fetch chunk file: %q: %w", chunkName, err)
		}
		if err := lbr.writeFile(temp, blockChunkPath, chunk); err != nil {
			return fmt.Errorf("failed to write chunk file: %q: %s", chunkName, err)
		}
	}

	// Set postingDecoder to nil because
	// If it is nil then a default decoder is used, compatible with Prometheus v2.
	pb, err := tsdb.OpenBlock(nil, temp, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to open block %q: %w", lbr.ID, err)
	}
	lbr.reader = pb
	return nil
}

// Index returns an IndexReader over the block's data.
func (lbr *lazyBlockReader) Index() (tsdb.IndexReader, error) {
	if err := lbr.initialize(); err != nil {
		return nil, err
	}
	return lbr.reader.Index()
}

// Chunks returns a ChunkReader over the block's data.
func (lbr *lazyBlockReader) Chunks() (tsdb.ChunkReader, error) {
	if err := lbr.initialize(); err != nil {
		return nil, err
	}
	return lbr.reader.Chunks()
}

// Tombstones returns a tombstones.Reader over the block's deleted data.
func (lbr *lazyBlockReader) Tombstones() (tombstones.Reader, error) {
	if err := lbr.initialize(); err != nil {
		return nil, err
	}
	return lbr.reader.Tombstones()
}

// Meta provides meta information about the block reader.
func (lbr *lazyBlockReader) Meta() tsdb.BlockMeta {
	if err := lbr.initialize(); err != nil {
		lbr.err = fmt.Errorf("cannot get BlockMeta: %w", err)
		return tsdb.BlockMeta{}
	}
	return lbr.reader.Meta()
}

// Size returns the number of bytes that the block takes up on disk.
func (lbr *lazyBlockReader) Size() int64 {
	if err := lbr.initialize(); err != nil {
		lbr.err = fmt.Errorf("error get Size of the block: %s, return zero size", err)
		return 0
	}
	return lbr.reader.Size()
}

// Err returns the last error that occurred on the block reader.
func (lbr *lazyBlockReader) Err() error {
	return lbr.err
}

// Close closes block and releases all resources
func (lbr *lazyBlockReader) Close() error {
	lbr.mu.Lock()
	defer lbr.mu.Unlock()

	err := lbr.reader.Close()
	lbr.reader = nil
	lbr.tempDirPath = ""

	if err := os.RemoveAll(lbr.tempDirPath); err != nil {
		log.Printf("failed to remove temp dir: %s", err)
	}
	return err
}

func (lbr *lazyBlockReader) mkTempDir() (string, error) {
	temp, err := os.MkdirTemp("", lbr.ID.String())
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %s", err)
	}
	err = os.Mkdir(filepath.Join(temp, "chunks"), os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %s", err)
	}
	return temp, nil
}

func (lbr *lazyBlockReader) fetchFile(filePath string) ([]byte, error) {
	blockID := lbr.ID.String()
	blockPath := filepath.Join(blockID, filePath)
	has, err := lbr.fs.HasFile(blockPath)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, fmt.Errorf("block meta %s not found", blockID)
	}
	return lbr.fs.ReadFile(blockPath)
}

func (lbr *lazyBlockReader) writeFile(folder string, filename string, file []byte) error {
	fileName := filepath.Join(folder, filename)
	return os.WriteFile(fileName, file, os.ModePerm)
}
