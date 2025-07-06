package s3remote

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fscommon"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	supportedStorageClasses = []s3types.StorageClass{s3types.StorageClassGlacier, s3types.StorageClassDeepArchive, s3types.StorageClassGlacierIr, s3types.StorageClassIntelligentTiering, s3types.StorageClassOnezoneIa, s3types.StorageClassOutposts, s3types.StorageClassReducedRedundancy, s3types.StorageClassStandard, s3types.StorageClassStandardIa}
)

func validateStorageClass(storageClass s3types.StorageClass) error {
	// if no storageClass set, no need to validate against supported values
	// backwards compatibility
	if len(storageClass) == 0 {
		return nil
	}

	for _, supported := range supportedStorageClasses {
		if supported == storageClass {
			return nil
		}
	}

	return fmt.Errorf("unsupported S3 storage class: %s. Supported values: %v", storageClass, supportedStorageClasses)
}

// StringToS3StorageClass converts string types to AWS S3 StorageClass type for value comparison
func StringToS3StorageClass(sc string) s3types.StorageClass {
	return s3types.StorageClass(sc)
}

// FS represents filesystem for backups in S3.
//
// Init must be called before calling other FS methods.
type FS struct {
	// Path to S3 credentials file.
	CredsFilePath string

	// Path to S3 configs file.
	ConfigFilePath string

	// S3 bucket to use.
	Bucket string

	// Directory in the bucket to write to.
	Dir string

	// Set for using S3-compatible endpoint such as MinIO etc.
	CustomEndpoint string

	// Force to use path style for s3, true by default.
	S3ForcePathStyle bool

	// Object Storage Class: https://aws.amazon.com/s3/storage-classes/
	StorageClass s3types.StorageClass

	// The name of S3 config profile to use.
	ProfileName string

	// Whether to use HTTP client with tls.InsecureSkipVerify setting
	TLSInsecureSkipVerify bool

	s3       *s3.Client
	uploader *manager.Uploader

	ctx    context.Context
	cancel context.CancelFunc

	// Metadata to be set for uploaded objects.
	Metadata map[string]string

	// S3 tags to be set for uploaded objects.
	Tags map[string]string

	// parsed Metadata to be used with aws-sdk-go-v2
	metadata map[string]*string

	// parsed Tags to be used with aws-sdk-go-v2
	tags *string
}

// Init initializes fs.
//
// The returned fs must be stopped when no long needed with MustStop call.
func (fs *FS) Init(ctx context.Context) error {
	if fs.s3 != nil {
		logger.Panicf("BUG: Init is already called")
	}
	for strings.HasPrefix(fs.Dir, "/") {
		fs.Dir = fs.Dir[1:]
	}
	if !strings.HasSuffix(fs.Dir, "/") {
		fs.Dir += "/"
	}
	configOpts := []func(*config.LoadOptions) error{
		config.WithDefaultRegion("us-east-1"),
		config.WithRetryer(func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.Backoff = retry.NewExponentialJitterBackoff(3 * time.Minute)
				o.MaxAttempts = 10
				o.Retryables = append(retry.DefaultRetryables, retry.RetryableErrorCode{
					Codes: map[string]struct{}{
						"IncompleteBody": {},
						// Tolerate token expiration as it might be handled by token rotation automatically
						// when using EKS Pod Identity or similar.
						// See: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9280
						"ExpiredToken": {},
					},
				})
			})
		}),
	}

	if len(fs.ProfileName) > 0 {
		configOpts = append(configOpts, config.WithSharedConfigProfile(fs.ProfileName))
	}
	if len(fs.ConfigFilePath) > 0 {
		configOpts = append(configOpts, config.WithSharedConfigFiles([]string{
			fs.ConfigFilePath,
		}))
	}

	if len(fs.CredsFilePath) > 0 {
		configOpts = append(configOpts, config.WithSharedCredentialsFiles([]string{
			fs.CredsFilePath,
		}))
	}

	fs.ctx, fs.cancel = context.WithCancel(ctx)
	cfg, err := config.LoadDefaultConfig(fs.ctx,
		configOpts...,
	)
	if err != nil {
		return fmt.Errorf("cannot load S3 config: %w", err)
	}

	if err = validateStorageClass(fs.StorageClass); err != nil {
		return err
	}

	tr := httputil.NewTransport(true, "vmbackup_s3_client")
	if fs.TLSInsecureSkipVerify {
		tr.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	cfg.HTTPClient = &http.Client{
		Transport: tr,
	}

	var outerErr error
	fs.s3 = s3.NewFromConfig(cfg, func(o *s3.Options) {
		if len(fs.CustomEndpoint) > 0 {
			logger.Infof("Using provided custom S3 endpoint: %q", fs.CustomEndpoint)
			o.UsePathStyle = fs.S3ForcePathStyle
			o.BaseEndpoint = &fs.CustomEndpoint
		} else {
			region, err := manager.GetBucketRegion(fs.ctx, s3.NewFromConfig(cfg), fs.Bucket)
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

	fs.uploader = manager.NewUploader(fs.s3, func(u *manager.Uploader) {
		// We manage upload concurrency by ourselves.
		u.Concurrency = 1
	})

	m := make(map[string]*string)
	for k, v := range fs.Metadata {
		m[k] = &v
	}
	fs.metadata = m

	if len(fs.Tags) > 0 {
		tags := make([]string, 0, len(fs.Tags))
		for k, v := range fs.Tags {
			tags = append(tags, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(tags)
		tagsString := strings.Join(tags, "&")
		fs.tags = &tagsString
	}

	return nil
}

// MustStop stops fs.
func (fs *FS) MustStop() {
	if fs.cancel != nil {
		fs.cancel()
	}
	fs.s3 = nil
	fs.uploader = nil
}

// String returns human-readable description for fs.
func (fs *FS) String() string {
	return fmt.Sprintf("S3{bucket: %q, dir: %q}", fs.Bucket, fs.Dir)
}

// ListParts returns all the parts for fs.
func (fs *FS) ListParts() ([]common.Part, error) {
	dir := fs.Dir

	var parts []common.Part

	paginator := s3.NewListObjectsV2Paginator(fs.s3, &s3.ListObjectsV2Input{
		Bucket: aws.String(fs.Bucket),
		Prefix: aws.String(dir),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(fs.ctx)
		if err != nil {
			return nil, fmt.Errorf("unexpected pagination error: %w", err)
		}

		for _, o := range page.Contents {
			file := *o.Key
			if !strings.HasPrefix(file, dir) {
				return nil, fmt.Errorf("unexpected prefix for s3 key %q; want %q", file, dir)
			}
			if fscommon.IgnorePath(file) {
				continue
			}
			var p common.Part
			if !p.ParseFromRemotePath(file[len(dir):]) {
				logger.Infof("skipping unknown object %q", file)
				continue
			}

			p.ActualSize = uint64(*o.Size)
			parts = append(parts, p)
		}

	}

	return parts, nil
}

// DeletePart deletes part p from fs.
func (fs *FS) DeletePart(p common.Part) error {
	path := fs.path(p)
	return fs.delete(path)
}

// RemoveEmptyDirs recursively removes empty dirs in fs.
func (fs *FS) RemoveEmptyDirs() error {
	// S3 has no directories, so nothing to remove.
	return nil
}

// CopyPart copies p from srcFS to fs.
func (fs *FS) CopyPart(srcFS common.OriginFS, p common.Part) error {
	src, ok := srcFS.(*FS)
	if !ok {
		return fmt.Errorf("cannot perform server-side copying from %s to %s: both of them must be S3", srcFS, fs)
	}
	srcPath := src.path(p)
	dstPath := fs.path(p)
	copySource := fmt.Sprintf("/%s/%s", src.Bucket, srcPath)

	input := &s3.CopyObjectInput{
		Bucket:            aws.String(fs.Bucket),
		CopySource:        aws.String(copySource),
		Key:               aws.String(dstPath),
		StorageClass:      fs.StorageClass,
		Metadata:          fs.Metadata,
		MetadataDirective: s3types.MetadataDirectiveReplace,
		Tagging:           fs.tags,
	}

	_, err := fs.s3.CopyObject(fs.ctx, input)
	if err != nil {
		return fmt.Errorf("cannot copy %q from %s to %s (copySource %q): %w", p.Path, src, fs, copySource, err)
	}
	return nil
}

// DownloadPart downloads part p from fs to w.
func (fs *FS) DownloadPart(p common.Part, w io.Writer) error {
	path := fs.path(p)
	input := &s3.GetObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(path),
	}
	o, err := fs.s3.GetObject(fs.ctx, input)
	if err != nil {
		return fmt.Errorf("cannot open %q at %s (remote path %q): %w", p.Path, fs, path, err)
	}
	r := o.Body
	n, err := io.Copy(w, r)
	if err1 := r.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		return fmt.Errorf("cannot download %q from at %s (remote path %q): %w", p.Path, fs, path, err)
	}
	if uint64(n) != p.Size {
		return fmt.Errorf("wrong data size downloaded from %q at %s; got %d bytes; want %d bytes", p.Path, fs, n, p.Size)
	}
	return nil
}

// UploadPart uploads part p from r to fs.
func (fs *FS) UploadPart(p common.Part, r io.Reader) error {
	path := fs.path(p)
	sr := &statReader{
		r: r,
	}
	input := &s3.PutObjectInput{
		Bucket:       aws.String(fs.Bucket),
		Key:          aws.String(path),
		Body:         sr,
		StorageClass: fs.StorageClass,
		Metadata:     fs.Metadata,
		Tagging:      fs.tags,
	}

	_, err := fs.uploader.Upload(fs.ctx, input)
	if err != nil {
		return fmt.Errorf("cannot upload data to %q at %s (remote path %q): %w", p.Path, fs, path, err)
	}
	if uint64(sr.size) != p.Size {
		return fmt.Errorf("wrong data size uploaded to %q at %s; got %d bytes; want %d bytes", p.Path, fs, sr.size, p.Size)
	}
	return nil
}

// DeleteFile deletes filePath from fs if it exists.
//
// The function does nothing if the file doesn't exist.
func (fs *FS) DeleteFile(filePath string) error {
	// It looks like s3 may return `AccessDenied: Access Denied` instead of `s3.ErrCodeNoSuchKey`
	// on an attempt to delete non-existing file.
	// so just check whether the filePath exists before deleting it.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/284 for details.
	ok, err := fs.HasFile(filePath)
	if err != nil {
		return err
	}
	if !ok {
		// Missing file - nothing to delete.
		return nil
	}

	path := path.Join(fs.Dir, filePath)
	return fs.delete(path)
}

func (fs *FS) delete(path string) error {
	if *common.DeleteAllObjectVersions {
		return fs.deleteObjectWithVersions(path)
	}
	return fs.deleteObject(path)
}

// deleteObject deletes object at path.
// It does not specify a version ID, so it will delete the latest version of the object.
func (fs *FS) deleteObject(path string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(path),
	}
	if _, err := fs.s3.DeleteObject(fs.ctx, input); err != nil {
		return fmt.Errorf("cannot delete %q at %s: %w", path, fs, err)
	}
	return nil
}

// deleteObjectWithVersions deletes object at path and all its versions.
func (fs *FS) deleteObjectWithVersions(path string) error {
	versions, err := fs.s3.ListObjectVersions(fs.ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(fs.Bucket),
		Prefix: aws.String(path),
	})
	if err != nil {
		return fmt.Errorf("cannot list versions for %q at %s: %w", path, fs, err)
	}

	for _, version := range versions.Versions {
		input := &s3.DeleteObjectInput{
			Bucket:    aws.String(fs.Bucket),
			Key:       version.Key,
			VersionId: version.VersionId,
		}
		if _, err := fs.s3.DeleteObject(fs.ctx, input); err != nil {
			return fmt.Errorf("cannot delete %q at %s: %w", path, fs, err)
		}
	}

	return nil
}

// CreateFile creates filePath at fs and puts data into it.
//
// The file is overwritten if it already exists.
func (fs *FS) CreateFile(filePath string, data []byte) error {
	path := path.Join(fs.Dir, filePath)
	sr := &statReader{
		r: bytes.NewReader(data),
	}
	input := &s3.PutObjectInput{
		Bucket:       aws.String(fs.Bucket),
		Key:          aws.String(path),
		Body:         sr,
		StorageClass: fs.StorageClass,
		Metadata:     fs.Metadata,
		Tagging:      fs.tags,
	}
	_, err := fs.uploader.Upload(fs.ctx, input)
	if err != nil {
		return fmt.Errorf("cannot upload data to %q at %s (remote path %q): %w", filePath, fs, path, err)
	}
	l := int64(len(data))
	if sr.size != l {
		return fmt.Errorf("wrong data size uploaded to %q at %s; got %d bytes; want %d bytes", filePath, fs, sr.size, l)
	}
	return nil
}

// HasFile returns true if filePath exists at fs.
func (fs *FS) HasFile(filePath string) (bool, error) {
	path := path.Join(fs.Dir, filePath)
	input := &s3.GetObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(path),
	}
	o, err := fs.s3.GetObject(fs.ctx, input)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") {
			return false, nil
		}
		return false, fmt.Errorf("cannot open %q at %s (remote path %q): %w", filePath, fs, path, err)
	}
	if err := o.Body.Close(); err != nil {
		return false, fmt.Errorf("cannot close %q at %s (remote path %q): %w", filePath, fs, path, err)
	}
	return true, nil
}

// ReadFile returns the content of filePath at fs.
func (fs *FS) ReadFile(filePath string) ([]byte, error) {
	p := path.Join(fs.Dir, filePath)
	input := &s3.GetObjectInput{
		Bucket: aws.String(fs.Bucket),
		Key:    aws.String(p),
	}
	o, err := fs.s3.GetObject(fs.ctx, input)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q at %s (remote path %q): %w", filePath, fs, p, err)
	}
	defer o.Body.Close()
	b, err := io.ReadAll(o.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q at %s (remote path %q): %w", filePath, fs, p, err)
	}
	return b, nil
}

func (fs *FS) path(p common.Part) string {
	return p.RemotePath(fs.Dir)
}

type statReader struct {
	r    io.Reader
	size int64
}

func (sr *statReader) Read(p []byte) (int, error) {
	n, err := sr.r.Read(p)
	sr.size += int64(n)
	return n, err
}
