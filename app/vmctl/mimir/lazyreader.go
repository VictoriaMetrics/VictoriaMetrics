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

var _ tsdb.BlockReader = (*LazyBlockReader)(nil)

// LazyBlockReader is stores block id and segment num information.
// It is used to lazily fetch and parse block data.
// It implements tsdb.BlockReader interface.
type LazyBlockReader struct {
	// Block ID.
	ID ulid.ULID
	// SegmentsNum stores the number of chunks segments in the block.
	SegmentsNum int

	mu     sync.Mutex
	reader tsdb.BlockReader
	fs     common.RemoteFS
	err    error
}

// NewLazyBlockReader returns a new LazyBlockReader for the given block.
func NewLazyBlockReader(block *Block, fs common.RemoteFS) (*LazyBlockReader, error) {
	if block.SegmentsFormat != "1b6d" {
		return nil, fmt.Errorf("unsupported segments format: %s", block.SegmentsFormat)
	}

	return &LazyBlockReader{
		ID:          block.ID,
		SegmentsNum: block.SegmentsNum,
		fs:          fs,
	}, nil
}

func (lbr *LazyBlockReader) initialize() error {
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

	defer func() {
		if err := os.RemoveAll(temp); err != nil {
			log.Printf("failed to remove temp dir: %s", err)
		}
		log.Printf("removed temp dir: %s", temp)
	}()

	meta, err := lbr.fetchFile(metaFilename)
	if err != nil {
		return err
	}
	if err := lbr.writeFile(temp, metaFilename, meta); err != nil {
		log.Printf("failed to write meta file: %s", err)
		return err
	}
	idx, err := lbr.fetchFile(indexFilename)
	if err != nil {
		log.Printf("failed to fetch index file %q: %s", indexFilename, err)
		return err
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
			log.Printf("failed to fetch chunk file: %q: %s", chunkName, err)
			return err
		}
		if err := lbr.writeFile(temp, blockChunkPath, chunk); err != nil {
			log.Printf("failed to write chunk file: %q: %s", chunkName, err)
			return err
		}
	}

	// Set postingDecoder to nil because
	// If it is nil then a default decoder is used, compatible with Prometheus v2.
	pb, err := tsdb.OpenBlock(nil, temp, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to open block %q: %s", lbr.ID, err)
	}
	lbr.reader = pb
	return nil
}

// Index returns an IndexReader over the block's data.
func (lbr *LazyBlockReader) Index() (tsdb.IndexReader, error) {
	if err := lbr.initialize(); err != nil {
		return nil, err
	}
	return lbr.reader.Index()
}

// Chunks returns a ChunkReader over the block's data.
func (lbr *LazyBlockReader) Chunks() (tsdb.ChunkReader, error) {
	if err := lbr.initialize(); err != nil {
		return nil, err
	}
	return lbr.reader.Chunks()
}

// Tombstones returns a tombstones.Reader over the block's deleted data.
func (lbr *LazyBlockReader) Tombstones() (tombstones.Reader, error) {
	if err := lbr.initialize(); err != nil {
		return nil, err
	}
	return lbr.reader.Tombstones()
}

// Meta provides meta information about the block reader.
func (lbr *LazyBlockReader) Meta() tsdb.BlockMeta {
	if err := lbr.initialize(); err != nil {
		lbr.err = fmt.Errorf("error get Block Meta: %s; return empty block", err)
		return tsdb.BlockMeta{}
	}
	return lbr.reader.Meta()
}

// Size returns the number of bytes that the block takes up on disk.
func (lbr *LazyBlockReader) Size() int64 {
	if err := lbr.initialize(); err != nil {
		lbr.err = fmt.Errorf("error get Size of the block: %s, return zero size", err)
		return 0
	}
	return lbr.reader.Size()
}

// Err returns the last error that occurred on the block reader.
func (lbr *LazyBlockReader) Err() error {
	return lbr.err
}

func (lbr *LazyBlockReader) mkTempDir() (string, error) {
	temp, err := os.MkdirTemp("", lbr.ID.String())
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %s", err)
	}
	err = os.Mkdir(filepath.Join(temp, "chunks"), 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %s", err)
	}
	return temp, nil
}

func (lbr *LazyBlockReader) fetchFile(filePath string) ([]byte, error) {
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

func (lbr *LazyBlockReader) writeFile(folder string, filename string, file []byte) error {
	fileName := filepath.Join(folder, filename)
	return os.WriteFile(fileName, file, 0644)
}
