package thanos

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
)

// BlockInfo contains information about a block including Thanos metadata.
type BlockInfo struct {
	Block      tsdb.BlockReader
	Resolution ResolutionLevel
	IsThanos   bool
	// Closer releases the block's resources (file descriptors, mmap).
	// Must be called only after all queriers on this block have been closed.
	Closer io.Closer
}

// OpenBlocksWithInfo opens all blocks and returns them with their metadata.
// snapshotDir must be a snapshot directory containing block directories.
func OpenBlocksWithInfo(snapshotDir string, aggrType AggrType) ([]BlockInfo, error) {
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot directory: %w", err)
	}

	var blocks []BlockInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		blockDir := filepath.Join(snapshotDir, entry.Name())
		metaPath := filepath.Join(blockDir, "meta.json")

		// Check if this is a valid block directory (has meta.json)
		if _, err := os.Stat(metaPath); os.IsNotExist(err) {
			continue
		}

		meta, err := ReadBlockMeta(blockDir)
		if err != nil {
			CloseBlocks(blocks)
			return nil, fmt.Errorf("failed to read Thanos metadata for block %s: %w", blockDir, err)
		}

		var pool chunkenc.Pool
		if meta.IsDownsampled() {
			// Use AggrChunkPool for downsampled blocks
			pool = NewAggrChunkPool(aggrType)
		}

		block, err := tsdb.OpenBlock(nil, blockDir, pool, nil)
		if err != nil {
			// Close previously opened blocks before returning error
			CloseBlocks(blocks)
			return nil, fmt.Errorf("failed to open block %s: %w", blockDir, err)
		}

		blocks = append(blocks, BlockInfo{
			Block:      block,
			Resolution: meta.Resolution(),
			IsThanos:   true,
			Closer:     block,
		})
	}

	return blocks, nil
}

// CloseBlocks closes all blocks in the slice.
// Must be called only after all queriers on these blocks have been closed.
func CloseBlocks(blocks []BlockInfo) {
	for _, bi := range blocks {
		if bi.Closer != nil {
			_ = bi.Closer.Close()
		}
	}
}
