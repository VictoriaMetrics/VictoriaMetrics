package gcs

import (
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/googleapis/gax-go/v2"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// FS represents filesystem for backups in GCS.
//
// Init must be called before calling other FS methods.
type FS struct {
	// Path to GCP credentials file.
	//
	// Default credentials are used if empty.
	CredsFilePath string

	// GCS bucket to use.
	Bucket string

	// Prefix is the prefix filter to query objects
	// whose names begin with this prefix.
	//
	// Optional.
	Prefix string

	bkt *storage.BucketHandle
}

// Init initializes fs.
//
// The returned fs must be stopped when no long needed with MustStop call.
func (fs *FS) Init() error {
	if fs.bkt != nil {
		logger.Panicf("BUG: fs.Init has been already called")
	}
	ctx := context.Background()
	var client *storage.Client
	if len(fs.CredsFilePath) > 0 {
		creds := option.WithCredentialsFile(fs.CredsFilePath)
		c, err := storage.NewClient(ctx, creds)
		if err != nil {
			return fmt.Errorf("cannot create gcs client with credsFile %q: %w", fs.CredsFilePath, err)
		}
		client = c
	} else {
		c, err := storage.NewClient(ctx)
		if err != nil {
			return fmt.Errorf("cannot create default gcs client: %w", err)
		}
		client = c
	}

	client.SetRetry(
		storage.WithPolicy(storage.RetryAlways),
		storage.WithBackoff(gax.Backoff{
			Initial:    time.Second,
			Max:        time.Minute * 3,
			Multiplier: 3,
		}))
	fs.bkt = client.Bucket(fs.Bucket)
	return nil
}

// MustStop stops fs.
func (fs *FS) MustStop() {
	fs.bkt = nil
}

// String returns human-readable description for fs.
func (fs *FS) String() string {
	return fmt.Sprintf("GCS{bucket: %q, Prefix: %q}", fs.Bucket, fs.Prefix)
}

// selectAttrs contains object attributes to select in ListParts.
var selectAttrs = []string{"Name"}

// Read returns a map of read files where
// key is the file name and value is file's content.
func (fs *FS) Read() (map[string][]byte, error) {
	q := &storage.Query{
		Prefix: fs.Prefix,
	}
	if err := q.SetAttrSelection(selectAttrs); err != nil {
		return nil, fmt.Errorf("error in SetAttrSelection: %w", err)
	}
	result := make(map[string][]byte)
	it := fs.bkt.Objects(context.Background(), q)
	for {
		attr, err := it.Next()
		if err == iterator.Done {
			return result, nil
		}
		if err != nil {
			return nil, fmt.Errorf("error while iterating objects at %q: %w", fs.Prefix, err)
		}
		file := attr.Name
		o := fs.bkt.Object(file)
		r, err := o.NewReader(context.Background())
		if err != nil {
			return nil, fmt.Errorf("cannot open reader for %q: %w", file, err)
		}

		b, err := io.ReadAll(r)
		if err1 := r.Close(); err1 != nil && err == nil {
			err = err1
		}
		if err != nil {
			return nil, fmt.Errorf("faile to read %q: %w", file, err)
		}
		result[fs.fullPath(file)] = b
	}
}

func (fs *FS) fullPath(file string) string {
	return fmt.Sprintf("gcs://%s/%s", fs.Bucket, file)
}
