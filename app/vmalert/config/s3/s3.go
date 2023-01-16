package s3

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// FS represents filesystem for backups in S3.
//
// Init must be called before calling other FS methods.
type FS struct {
	// Path to S3 credentials file.
	CredsFilePath string

	// Path to S3 configs file.
	ConfigFilePath string

	// GCS bucket to use.
	Bucket string

	// Prefix is the prefix filter to query objects
	// whose names begin with this prefix.
	//
	// Optional.
	Prefix string

	// Set for using S3-compatible enpoint such as MinIO etc.
	CustomEndpoint string

	// Force to use path style for s3, true by default.
	S3ForcePathStyle bool

	// The name of S3 config profile to use.
	ProfileName string

	s3 *s3.Client
}

// Init initializes fs.
//
// The returned fs must be stopped when no long needed with MustStop call.
func (fs *FS) Init() error {
	if fs.s3 != nil {
		logger.Panicf("BUG: Init is already called")
	}
	configOpts := []func(*config.LoadOptions) error{
		config.WithSharedConfigProfile(fs.ProfileName),
		config.WithDefaultRegion("us-east-1"),
	}

	if len(fs.CredsFilePath) > 0 {
		configOpts = append(configOpts, config.WithSharedConfigFiles([]string{
			fs.ConfigFilePath,
			fs.CredsFilePath,
		}))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		configOpts...,
	)
	if err != nil {
		return fmt.Errorf("cannot load S3 config: %w", err)
	}
	var outerErr error
	fs.s3 = s3.NewFromConfig(cfg, func(o *s3.Options) {
		if len(fs.CustomEndpoint) > 0 {
			logger.Infof("Using provided custom S3 endpoint: %q", fs.CustomEndpoint)
			o.UsePathStyle = fs.S3ForcePathStyle
			o.EndpointResolver = s3.EndpointResolverFromURL(fs.CustomEndpoint)
		} else {
			region, err := manager.GetBucketRegion(context.Background(), s3.NewFromConfig(cfg), fs.Bucket)
			if err != nil {
				outerErr = fmt.Errorf("cannot determine region for bucket %q: %w", fs.Bucket, err)
				return
			}

			o.Region = region
			logger.Infof("bucket %q is stored at region %q; switching to this region", fs.Bucket, region)
		}
	})

	if outerErr != nil {
		return outerErr
	}
	return nil
}

// MustStop stops fs.
func (fs *FS) MustStop() {
	fs.s3 = nil
}

// String returns human-readable description for fs.
func (fs *FS) String() string {
	return fmt.Sprintf("S3{bucket: %q, Prefix: %q}", fs.Bucket, fs.Prefix)
}

// Read returns a map of read files where
// key is the file name and value is file's content.
func (fs *FS) Read() (map[string][]byte, error) {
	paginator := s3.NewListObjectsV2Paginator(fs.s3, &s3.ListObjectsV2Input{
		Bucket: aws.String(fs.Bucket),
		Prefix: aws.String(fs.Prefix),
	})
	prefix := fs.Prefix
	result := make(map[string][]byte)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("unexpected pagination error: %w", err)
		}
		for _, obj := range page.Contents {
			file := *obj.Key
			if !strings.HasPrefix(file, prefix) {
				return nil, fmt.Errorf("unexpected prefix for s3 key %q; want %q", file, prefix)
			}
			input := &s3.GetObjectInput{
				Bucket: aws.String(fs.Bucket),
				Key:    aws.String(file),
			}
			o, err := fs.s3.GetObject(context.Background(), input)
			if err != nil {
				return nil, fmt.Errorf("cannot open %q: %w", file, err)
			}
			r := o.Body
			b, err := io.ReadAll(r)
			if err1 := r.Close(); err1 != nil && err == nil {
				err = err1
			}
			if err != nil {
				return nil, fmt.Errorf("cannot read %q: %w", file, err)
			}
			result[fs.fullPath(file)] = b
		}
	}
	return result, nil
}

func (fs *FS) fullPath(file string) string {
	return fmt.Sprintf("s3://%s/%s", fs.Bucket, file)
}
