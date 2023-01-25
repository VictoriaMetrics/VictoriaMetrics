package config

import (
	"flag"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config/gcs"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config/s3"
)

var (
	credsFilePath = flag.String("credsFilePath", "", "Path to file with GCS or S3 credentials. Credentials are loaded from default locations if not set.\n"+
		"See https://cloud.google.com/iam/docs/creating-managing-service-account-keys and https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html")
	configFilePath = flag.String("configFilePath", "", "Path to file with S3 configs. Configs are loaded from default location if not set.\n"+
		"See https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html")
	configProfile = flag.String("configProfile", "", "Profile name for S3 configs. If no set, the value of the environment variable will be loaded (AWS_PROFILE or AWS_DEFAULT_PROFILE), "+
		"or if both not set, DefaultSharedConfigProfile is used")
	customS3Endpoint = flag.String("customS3Endpoint", "", "Custom S3 endpoint for use with S3-compatible storages (e.g. MinIO). S3 is used if not set")
	s3ForcePathStyle = flag.Bool("s3ForcePathStyle", true, "Prefixing endpoint with bucket name when set false, true by default.")
)

// FS represent a file system abstract for reading files.
type FS interface {
	// MustStop must be called when the FS is no longer needed.
	MustStop()

	// String must return human-readable representation of FS.
	String() string

	// Read returns a list of read files in form of a map
	// where key is a file name and value is a content of read file.
	Read() (map[string][]byte, error)
}

// InitFS  inits one or more FS from the given paths.
// It is allowed to mix different FS types in paths.
func InitFS(paths []string) ([]FS, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("path cannot be empty")
	}

	var fss []FS
	for _, path := range paths {
		fs, err := newFS(path, true)
		if err != nil {
			return nil, fmt.Errorf("error reading file path %s: %w", path, err)
		}
		fss = append(fss, fs)
	}
	return fss, nil
}

// newFS creates FS based on the give path.
// Supported file systems are: fs, gcs|gs, s3.
//
// initFS defines whether FS needs to be started
// which requires actually requests to remote storages.
// initFS is used in tests to prevent actual init.
func newFS(path string, initFS bool) (FS, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("path cannot be empty")
	}

	n := strings.Index(path, "://")
	if n < 0 {
		return &fslocal.FS{Pattern: path}, nil
	}

	scheme := path[:n]
	switch scheme {
	case "fs":
		return &fslocal.FS{Pattern: path}, nil
	case "gcs", "gs":
		bucket, prefix := parseRemotePath(path)
		if bucket == "" {
			return nil, fmt.Errorf("can't parse bucket name for gcs path %q", path)
		}
		fs := &gcs.FS{
			CredsFilePath: *credsFilePath,
			Bucket:        bucket,
			Prefix:        prefix,
		}
		if !initFS {
			return fs, nil
		}
		if err := fs.Init(); err != nil {
			return nil, fmt.Errorf("cannot initialize connection to gcs: %w", err)
		}
		return fs, nil
	case "s3":
		bucket, prefix := parseRemotePath(path)
		if bucket == "" {
			return nil, fmt.Errorf("can't parse bucket name for s3 path %q", path)
		}
		fs := &s3.FS{
			CredsFilePath:    *credsFilePath,
			ConfigFilePath:   *configFilePath,
			CustomEndpoint:   *customS3Endpoint,
			S3ForcePathStyle: *s3ForcePathStyle,
			ProfileName:      *configProfile,
			Bucket:           bucket,
			Prefix:           prefix,
		}
		if !initFS {
			return fs, nil
		}
		if err := fs.Init(); err != nil {
			return nil, fmt.Errorf("cannot initialize connection to s3: %w", err)
		}
		return fs, nil
	default:
		return nil, fmt.Errorf("unsupported scheme %q", scheme)
	}
}

func parseRemotePath(path string) (string, string) {
	n := strings.Index(path, "://")
	if n == -1 {
		return "", ""
	}
	path = path[n+len("://"):]
	bucket := path
	prefix := ""
	n = strings.Index(path, "/")
	if n > 0 {
		bucket = path[:n]
		prefix = path[n:]
	}
	for strings.HasPrefix(prefix, "/") {
		prefix = prefix[1:]
	}
	for strings.HasSuffix(prefix, "/") {
		prefix = prefix[:len(prefix)-1]
	}
	return bucket, prefix
}
