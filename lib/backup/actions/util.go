package actions

import (
	"context"
	"flag"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/azremote"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fsremote"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/gcsremote"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/s3remote"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

var (
	objectMetadata = flag.String("objectMetadata", "", `Metadata to be set for uploaded objects to object storage. Must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}. Is ignored for local filesystem.`)

	credsFilePath = flag.String("credsFilePath", "", "Path to file with GCS or S3 credentials. Credentials are loaded from default locations if not set.\n"+
		"See https://cloud.google.com/iam/docs/creating-managing-service-account-keys and https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html")
	configFilePath = flag.String("configFilePath", "", "Path to file with S3 configs. Configs are loaded from default location if not set.\n"+
		"See https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html")
	configProfile = flag.String("configProfile", "", "Profile name for S3 configs. If no set, the value of the environment variable will be loaded (AWS_PROFILE or AWS_DEFAULT_PROFILE), "+
		"or if both not set, DefaultSharedConfigProfile is used")
	customS3Endpoint = flag.String("customS3Endpoint", "", "Custom S3 endpoint for use with S3-compatible storages (e.g. MinIO). S3 is used if not set")
	s3ACL            = flag.String("s3ACL", "bucket-owner-full-control", "ACL to be set for uploaded objects to S3.")
	s3ForcePathStyle = flag.Bool("s3ForcePathStyle", true, "Prefixing endpoint with bucket name when set false, true by default.")
	s3StorageClass   = flag.String("s3StorageClass", "", "The Storage Class applied to objects uploaded to AWS S3. Supported values are: GLACIER, "+
		"DEEP_ARCHIVE, GLACIER_IR, INTELLIGENT_TIERING, ONEZONE_IA, OUTPOSTS, REDUCED_REDUNDANCY, STANDARD, STANDARD_IA.\n"+
		"See https://docs.aws.amazon.com/AmazonS3/latest/userguide/storage-class-intro.html")
	s3ChecksumAlgorithm = flag.String("s3ChecksumAlgorithm", "", "Objects integrity checksum algorithm which is applied while uploading objects to AWS S3. "+
		"Supported values are: SHA256, SHA1, CRC32C, CRC32")
	s3SSEKMSKeyId           = flag.String("s3SSEKMSKeyId", "", "SSE KMS Key ID for use with S3-compatible storages.")
	s3SSEAlgorithm          = flag.String("s3SSEAlgorithm", "aws:kms", "SSE KMS Key Algorithm for use with S3-compatible storages.")
	s3TLSInsecureSkipVerify = flag.Bool("s3TLSInsecureSkipVerify", false, "Whether to skip TLS verification when connecting to the S3 endpoint.")
	s3Tags                  = flag.String("s3ObjectTags", "", `S3 tags to be set for uploaded objects. Must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}.`)
)

func runParallel(concurrency int, parts []common.Part, f func(p common.Part) error, progress func(elapsed time.Duration)) error {
	var err error
	runWithProgress(progress, func() {
		err = runParallelInternal(concurrency, parts, f)
	})
	return err
}

func runParallelPerPath(ctx context.Context, concurrency int, perPath map[string][]common.Part, f func(parts []common.Part) error, progress func(elapsed time.Duration)) error {
	var err error
	runWithProgress(progress, func() {
		err = runParallelPerPathInternal(ctx, concurrency, perPath, f)
	})
	return err
}

func runParallelPerPathInternal(ctx context.Context, concurrency int, perPath map[string][]common.Part, f func(parts []common.Part) error) error {
	if concurrency <= 0 {
		concurrency = 1
	}
	if len(perPath) == 0 {
		return nil
	}

	// len(perPath) capacity guarantees non-blocking behavior below.
	resultCh := make(chan error, len(perPath))
	workCh := make(chan []common.Part, len(perPath))
	ctxLocal, cancelLocal := context.WithCancel(ctx)
	defer cancelLocal()

	// Start workers
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for parts := range workCh {
				select {
				case <-ctxLocal.Done():
					return
				default:
				}
				resultCh <- f(parts)
			}
		}()
	}

	// Feed workers with work.
	for _, parts := range perPath {
		workCh <- parts
	}
	close(workCh)

	// Read results.
	var err error
	for i := 0; i < len(perPath); i++ {
		err = <-resultCh
		if err != nil {
			// Stop the work.
			cancelLocal()
			break
		}
	}

	// Wait for all the workers to stop.
	wg.Wait()

	return err
}

func runParallelInternal(concurrency int, parts []common.Part, f func(p common.Part) error) error {
	if concurrency <= 0 {
		concurrency = 1
	}
	if len(parts) == 0 {
		return nil
	}

	// len(parts) capacity guarantees non-blocking behavior below.
	resultCh := make(chan error, len(parts))
	workCh := make(chan common.Part, len(parts))
	stopCh := make(chan struct{})

	// Start workers
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for p := range workCh {
				select {
				case <-stopCh:
					return
				default:
				}
				resultCh <- f(p)
			}
		}()
	}

	// Feed workers with work.
	for _, p := range parts {
		workCh <- p
	}
	close(workCh)

	// Read results.
	var err error
	for i := 0; i < len(parts); i++ {
		err = <-resultCh
		if err != nil {
			// Stop the work.
			close(stopCh)
			break
		}
	}

	// Wait for all the workers to stop.
	wg.Wait()

	return err
}

func runWithProgress(progress func(elapsed time.Duration), f func()) {
	startTime := time.Now()
	doneCh := make(chan struct{})
	go func() {
		f()
		close(doneCh)
	}()

	tc := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-doneCh:
			tc.Stop()
			// The last progress call.
			progress(time.Since(startTime))
			return
		case <-tc.C:
			progress(time.Since(startTime))
		}
	}
}

func getPartsSize(parts []common.Part) uint64 {
	n := uint64(0)
	for _, p := range parts {
		n += p.Size
	}
	return n
}

// NewRemoteFS returns new remote fs from the given path.
func NewRemoteFS(ctx context.Context, path string, extraTags map[string]string) (common.RemoteFS, error) {
	m, err := flagutil.ParseJSONMap(*objectMetadata)
	if err != nil {
		return nil, fmt.Errorf("cannot parse s3 objectMetadata %q: %w", *objectMetadata, err)
	}
	if len(path) == 0 {
		return nil, fmt.Errorf("path cannot be empty")
	}
	n := strings.Index(path, "://")
	if n < 0 {
		return nil, fmt.Errorf("missing scheme in path %q. Supported schemes: `gs://`, `s3://`, `azblob://`, `fs://`", path)
	}
	scheme := path[:n]
	dir := path[n+len("://"):]
	switch scheme {
	case "fs":
		if !filepath.IsAbs(dir) {
			return nil, fmt.Errorf("dir must be absolute; got %q", dir)
		}
		fs := &fsremote.FS{
			Dir: filepath.Clean(dir),
		}
		return fs, nil
	case "gcs", "gs":
		n := strings.Index(dir, "/")
		if n < 0 {
			return nil, fmt.Errorf("missing directory on the gcs bucket %q", dir)
		}
		bucket := dir[:n]
		dir = dir[n:]
		fs := &gcsremote.FS{
			CredsFilePath: *credsFilePath,
			Bucket:        bucket,
			Dir:           dir,
			Metadata:      m,
		}
		if err := fs.Init(ctx); err != nil {
			return nil, fmt.Errorf("cannot initialize connection to gcs: %w", err)
		}
		return fs, nil
	case "azblob":
		n := strings.Index(dir, "/")
		if n < 0 {
			return nil, fmt.Errorf("missing directory on the AZBlob container %q", dir)
		}
		bucket := dir[:n]
		dir = dir[n:]

		fs := &azremote.FS{
			Container: bucket,
			Dir:       dir,
			Metadata:  m,
		}
		if err := fs.Init(ctx); err != nil {
			return nil, fmt.Errorf("cannot initialize connection to AZBlob: %w", err)
		}
		return fs, nil
	case "s3":
		n := strings.Index(dir, "/")
		if n < 0 {
			return nil, fmt.Errorf("missing directory on the s3 bucket %q", dir)
		}
		bucket := dir[:n]
		dir = dir[n:]
		tags, err := flagutil.ParseJSONMap(*s3Tags)
		if err != nil {
			return nil, fmt.Errorf("cannot parse s3 tags %q: %w", *s3Tags, err)
		}

		if len(extraTags) > 0 {
			if tags == nil {
				tags = make(map[string]string)
			}
			maps.Copy(tags, extraTags)
		}

		fs := &s3remote.FS{
			CredsFilePath:         *credsFilePath,
			ConfigFilePath:        *configFilePath,
			CustomEndpoint:        *customS3Endpoint,
			TLSInsecureSkipVerify: *s3TLSInsecureSkipVerify,
			StorageClass:          s3remote.StringToStorageClass(*s3StorageClass),
			ChecksumAlgorithm:     s3remote.StringToChecksumAlgorithm(*s3ChecksumAlgorithm),
			S3ForcePathStyle:      *s3ForcePathStyle,
			ACL:                   s3remote.StringToObjectACL(*s3ACL),
			SSEKMSKeyId:           *s3SSEKMSKeyId,
			SSEAlgorithm:          s3remote.StringToEncryptionAlgorithm(*s3SSEAlgorithm),
			ProfileName:           *configProfile,
			Bucket:                bucket,
			Dir:                   dir,
			Metadata:              m,
			Tags:                  tags,
		}
		if err := fs.Init(ctx); err != nil {
			return nil, fmt.Errorf("cannot initialize connection to s3: %w", err)
		}
		return fs, nil
	default:
		return nil, fmt.Errorf("unsupported scheme %q", scheme)
	}
}
