package mimir

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/oklog/ulid"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

const (
	bucketIndex                   = "bucket-index.json"
	bucketIndexCompressedFilename = bucketIndex + ".gz"
	metaFilename                  = "meta.json"
	indexFilename                 = "index"
)

// BlockDeletionMark holds the information about a block's deletion mark in the index.
type BlockDeletionMark struct {
	// Block ID.
	ID ulid.ULID `json:"block_id"`

	// DeletionTime is a unix timestamp (seconds precision) of when the block was marked to be deleted.
	DeletionTime int64 `json:"deletion_time"`
}

// Block holds the information about a block in the index.
type Block struct {
	// Block ID.
	ID ulid.ULID `json:"block_id"`

	// MinTime and MaxTime specify the time range all samples in the block are in (millis precision).
	MinTime int64 `json:"min_time"`
	MaxTime int64 `json:"max_time"`

	// SegmentsFormat and SegmentsNum stores the format and number of chunks segments
	// in the block, if they match a known pattern. We don't store the full segments
	// files list in order to keep the index small. SegmentsFormat is empty if segments
	// are unknown or don't match a known format.
	SegmentsFormat string `json:"segments_format,omitempty"`
	SegmentsNum    int    `json:"segments_num,omitempty"`
}

// Index contains all known blocks and markers of a tenant.
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
	S3StorageClass          string
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

func (f filter) inRange(min, max int64) bool {
	fmin, fmax := f.min, f.max
	if min == 0 {
		fmin = min
	}
	if fmax == 0 {
		fmax = max
	}
	return min <= fmax && fmin <= max
}

// NewClient creates and validates new Client
// with given Config
func NewClient(cfg Config) (*Client, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	if cfg.TenantID != "" {
		cfg.Path = fmt.Sprintf("%s/%s", cfg.Path, cfg.TenantID)
	}

	var c Client
	rfs, err := NewRemoteFS(cfg)
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

// Explore fetches all available blocks from a remote storage or local filesystem
// and collects the Meta() data from each block.
// Explore does initial filtering by time-range
// for snapshot blocks but does not take into account
// label filters.
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

		lazyBlockReader, err := New(block, c.RemoteFS)
		if err != nil {
			log.Printf("failed to create lazy block reader: %s", err)
			continue
		}
		blocksToImport = append(blocksToImport, lazyBlockReader)
	}

	s.Blocks = len(blocksToImport)
	return blocksToImport, nil
}

// Read reads the given BlockReader according to configured
// time and label filters.
func (c *Client) Read(block tsdb.BlockReader) (storage.SeriesSet, error) {
	meta := block.Meta()
	if meta.ULID.String() == "" {
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
	ss := q.Select(context.Background(), false, nil, labels.MustNewMatcher(labels.MatchRegexp, c.filter.label, c.filter.labelValue))
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
