package thanos

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// BlockMeta extends Prometheus BlockMeta with Thanos-specific fields.
type BlockMeta struct {
	// Thanos-specific metadata
	Thanos ThanosMeta `json:"thanos,omitempty"`
}

// ThanosMeta contains Thanos-specific block metadata.
type ThanosMeta struct {
	// Labels are external labels identifying the producer.
	Labels map[string]string `json:"labels,omitempty"`

	// Downsample contains downsampling information.
	Downsample ThanosDownsample `json:"downsample,omitempty"`

	// Source indicates where the block came from.
	Source string `json:"source,omitempty"`

	// SegmentFiles contains list of segment files in the block.
	SegmentFiles []string `json:"segment_files,omitempty"`

	// Files contains metadata about files in the block.
	Files []ThanosFile `json:"files,omitempty"`
}

// ThanosDownsample contains downsampling resolution info.
type ThanosDownsample struct {
	// Resolution is the downsampling resolution in milliseconds.
	// 0 means raw data (no downsampling).
	// 300000 (5 minutes) or 3600000 (1 hour) for downsampled data.
	Resolution int64 `json:"resolution"`
}

// ThanosFile contains metadata about a file in the block.
type ThanosFile struct {
	RelPath   string `json:"rel_path"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

// ResolutionLevel represents the downsampling resolution.
type ResolutionLevel int64

const (
	// ResolutionRaw is for raw, non-downsampled data.
	ResolutionRaw ResolutionLevel = 0
	// Resolution5m is for 5-minute downsampled data (300000 ms).
	Resolution5m ResolutionLevel = 300000
	// Resolution1h is for 1-hour downsampled data (3600000 ms).
	Resolution1h ResolutionLevel = 3600000
)

// String returns human-readable resolution string.
func (r ResolutionLevel) String() string {
	switch r {
	case ResolutionRaw:
		return "raw"
	case Resolution5m:
		return "5m"
	case Resolution1h:
		return "1h"
	default:
		return "unknown"
	}
}

// ReadBlockMeta reads Thanos-extended block metadata from meta.json.
func ReadBlockMeta(blockDir string) (*BlockMeta, error) {
	metaPath := filepath.Join(blockDir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta BlockMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// IsDownsampled returns true if the block contains downsampled data.
func (m *BlockMeta) IsDownsampled() bool {
	return m.Thanos.Downsample.Resolution > 0
}

// Resolution returns the block's downsampling resolution.
func (m *BlockMeta) Resolution() ResolutionLevel {
	return ResolutionLevel(m.Thanos.Downsample.Resolution)
}

// ResolutionSuffix returns a suffix string for metric names based on resolution.
// For example: ":5m" or ":1h" for downsampled data, empty for raw data.
func (m *BlockMeta) ResolutionSuffix() string {
	switch m.Resolution() {
	case Resolution5m:
		return ":5m"
	case Resolution1h:
		return ":1h"
	default:
		return ""
	}
}
