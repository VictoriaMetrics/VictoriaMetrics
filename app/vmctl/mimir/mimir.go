package mimir

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/oklog/ulid/v2"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"

	utils "github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vmctlutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

const (
	bucketIndex                   = "bucket-index.json"
	bucketIndexCompressedFilename = bucketIndex + ".gz"
	metaFilename                  = "meta.json"
	indexFilename                 = "index"
)

// BlockDeletionMark holds the information about a block's deletion mark in the index.
// This type was copied from the mimir repository https://github.com/grafana/mimir/blob/main/pkg/storage/tsdb/bucketindex/index.go#L234.
type BlockDeletionMark struct {
	// Block ID.
	ID ulid.ULID `json:"block_id"`

	// DeletionTime is a unix timestamp (seconds precision) of when the block was marked to be deleted.
	DeletionTime int64 `json:"deletion_time"`
}

// Block holds the information about a block in the index.
// This is a partial implementation of the https://github.com/grafana/mimir/blob/main/pkg/storage/tsdb/bucketindex/index.go#L73
type Block struct {
	// Block ID.
	ID ulid.ULID `json:"block_id"`

	// MinTime and MaxTime specify the time range all samples in the block are in (millis precision).
	MinTime int64 `json:"min_time"`
	MaxTime int64 `json:"max_time"`

	// SegmentsFormat and SegmentsNum stores the format and number of chunks segments
	// in the block.
	SegmentsFormat string `json:"segments_format,omitempty"`
	SegmentsNum    int    `json:"segments_num,omitempty"`
}

// Index contains all known blocks and markers of a tenant.
// This is a partial implementation pof the https://github.com/grafana/mimir/blob/main/pkg/storage/tsdb/bucketindex/index.go#L36
type Index struct {
	// Version of the index format.
	Version int `json:"version"`

	// List of complete blocks (partial blocks are excluded from the index).
	Blocks []*Block `json:"blocks"`
}

// Config contains a list of params needed
// for reading Prometheus snapshots
type Config struct {
	// Path to remote storage bucket
	Path string
	// TenantID is the tenant id for the storage
	TenantID string

	Filter Filter

	CredsFilePath           string
	ConfigFilePath          string
	ConfigProfile           string
	CustomS3Endpoint        string
	S3ForcePathStyle        bool
	S3TLSInsecureSkipVerify bool
}

// Filter contains configuration for filtering
// the timeseries
type Filter struct {
	TimeMin    string
	TimeMax    string
	Label      string
	LabelValue string
}

// Client is a wrapper over Prometheus tsdb.DBReader
type Client struct {
	common.RemoteFS
	filter filter
}

type filter struct {
	min, max   int64
	label      string
	labelValue string
}

func (f filter) inRange(minTime, maxTime int64) bool {
	fmin, fmax := f.min, f.max
	if minTime == 0 {
		fmin = minTime
	}
	if fmax == 0 {
		fmax = maxTime
	}
	return minTime <= fmax && fmin <= maxTime
}

// NewClient creates and validates new Client
// with given Config
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	if cfg.TenantID != "" {
		cfg.Path = fmt.Sprintf("%s/%s", cfg.Path, cfg.TenantID)
	}

	var c Client
	rfs, err := NewRemoteFS(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `-src`=%q: %w", cfg.Path, err)
	}

	c.RemoteFS = rfs
	timeMin, err := utils.ParseTime(cfg.Filter.TimeMin)
	if err != nil {
		return nil, fmt.Errorf("failed to parse min time in filter: %s", err)
	}
	timeMax, err := utils.ParseTime(cfg.Filter.TimeMax)
	if err != nil {
		return nil, fmt.Errorf("failed to parse max time in filter: %s", err)
	}
	c.filter = filter{
		min:        timeMin.UnixMilli(),
		max:        timeMax.UnixMilli(),
		label:      cfg.Filter.Label,
		labelValue: cfg.Filter.LabelValue,
	}
	return &c, nil
}

// Explore a fetches bucket-index.json file from a remote storage or local filesystem
// and filter blocks via the defined time range, but does not take into account label filters.
func (c *Client) Explore() ([]tsdb.BlockReader, error) {

	s := &utils.Stats{
		Filtered: c.filter.min != 0 || c.filter.max != 0 || c.filter.label != "",
	}

	log.Printf("Fetching blocks from remote storage")

	indexFile, err := c.fetchIndexFile()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index file: %s", err)
	}

	var blocksToImport []tsdb.BlockReader
	for _, block := range indexFile.Blocks {
		if !c.filter.inRange(block.MinTime, block.MaxTime) {
			// Skipping block outside of time range
			continue
		}

		if block.ID.String() == "" {
			continue
		}

		lazyBlockReader, err := NewLazyBlockReader(block, c.RemoteFS)
		if err != nil {
			return nil, fmt.Errorf("failed to create lazy block reader: %s", err)
		}
		blocksToImport = append(blocksToImport, lazyBlockReader)
	}

	s.Blocks = len(blocksToImport)
	return blocksToImport, nil
}

// Read reads the given BlockReader according to configured
// time and label filters.
func (c *Client) Read(ctx context.Context, block tsdb.BlockReader) (storage.SeriesSet, error) {
	meta := block.Meta()
	if b, ok := block.(*LazyBlockReader); ok && b.Err() != nil {
		return nil, fmt.Errorf("failed to read block: %s", b.Err())
	}

	if meta.ULID.String() == "" {
		log.Printf("got block without the id. it is empty")
		return nil, fmt.Errorf("block without id")
	}

	minTime, maxTime := meta.MinTime, meta.MaxTime
	if c.filter.min != 0 {
		minTime = c.filter.min
	}
	if c.filter.max != 0 {
		maxTime = c.filter.max
	}
	q, err := tsdb.NewBlockQuerier(block, minTime, maxTime)
	if err != nil {
		return nil, err
	}
	ss := q.Select(ctx, false, nil, labels.MustNewMatcher(labels.MatchRegexp, c.filter.label, c.filter.labelValue))
	return ss, nil
}

func (c *Client) fetchIndexFile() (*Index, error) {
	has, err := c.RemoteFS.HasFile(bucketIndexCompressedFilename)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, fmt.Errorf("bucket-index.json.gz not found")
	}

	file, err := c.RemoteFS.ReadFile(bucketIndexCompressedFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to read bucket index: %s", err)
	}

	r := bytes.NewReader(file)
	// Read all the content.
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %s", err)
	}

	var indexFile Index
	err = json.NewDecoder(gzipReader).Decode(&indexFile)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bucket index: %s", err)
	}

	return &indexFile, nil
}
