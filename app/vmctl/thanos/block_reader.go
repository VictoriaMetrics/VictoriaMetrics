package thanos

import (
	"fmt"
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

		// Read Thanos metadata to determine resolution
		meta, err := ReadBlockMeta(blockDir)
		if err != nil {
			// If we can't read Thanos meta, treat as raw Prometheus block
			block, err := tsdb.OpenBlock(nil, blockDir, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to open block %s: %w", blockDir, err)
			}
			blocks = append(blocks, BlockInfo{
				Block:      block,
				Resolution: ResolutionRaw,
				IsThanos:   false,
			})
			continue
		}

		var pool chunkenc.Pool
		if meta.IsDownsampled() {
			// Use AggrChunkPool for downsampled blocks
			pool = NewAggrChunkPool(aggrType)
		}

		block, err := tsdb.OpenBlock(nil, blockDir, pool, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to open block %s: %w", blockDir, err)
		}

		blocks = append(blocks, BlockInfo{
			Block:      block,
			Resolution: meta.Resolution(),
			IsThanos:   true,
		})
	}

	return blocks, nil
}
