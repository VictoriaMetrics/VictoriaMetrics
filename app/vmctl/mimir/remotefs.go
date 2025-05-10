package mimir

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/azremote"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fsremote"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/gcsremote"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/s3remote"
)

// NewRemoteFS returns new remote fs from the given Config.
func NewRemoteFS(ctx context.Context, cfg Config) (common.RemoteFS, error) {
	if len(cfg.Path) == 0 {
		return nil, fmt.Errorf("path cannot be empty")
	}
	n := strings.Index(cfg.Path, "://")
	if n < 0 {
		return nil, fmt.Errorf("missing scheme in path %q. Supported schemes: `gs://`, `s3://`, `azblob://`, `fs://`", cfg.Path)
	}
	scheme := cfg.Path[:n]
	dir := cfg.Path[n+len("://"):]
	switch scheme {
	case "fs":
		if !filepath.IsAbs(dir) {
			return nil, fmt.Errorf("dir must be absolute; got %q", dir)
		}
		fsr := &fsremote.FS{
			Dir: filepath.Clean(dir),
		}
		return fsr, nil
	case "gcs", "gs":
		n := strings.Index(dir, "/")
		if n < 0 {
			return nil, fmt.Errorf("missing directory on the gcs bucket %q", dir)
		}
		bucket := dir[:n]
		dir = dir[n:]
		fsr := &gcsremote.FS{
			CredsFilePath: cfg.CredsFilePath,
			Bucket:        bucket,
			Dir:           dir,
		}
		if err := fsr.Init(ctx); err != nil {
			return nil, fmt.Errorf("cannot initialize connection to gcs: %w", err)
		}
		return fsr, nil
	case "azblob":
		n := strings.Index(dir, "/")
		if n < 0 {
			return nil, fmt.Errorf("missing directory on the AZBlob container %q", dir)
		}
		bucket := dir[:n]
		dir = dir[n:]
		fsr := &azremote.FS{
			Container: bucket,
			Dir:       dir,
		}
		if err := fsr.Init(ctx); err != nil {
			return nil, fmt.Errorf("cannot initialize connection to AZBlob: %w", err)
		}
		return fsr, nil
	case "s3":
		n := strings.Index(dir, "/")
		if n < 0 {
			return nil, fmt.Errorf("missing directory on the s3 bucket %q", dir)
		}
		bucket := dir[:n]
		dir = dir[n:]
		fsr := &s3remote.FS{
			CredsFilePath:         cfg.CredsFilePath,
			ConfigFilePath:        cfg.ConfigFilePath,
			CustomEndpoint:        cfg.CustomS3Endpoint,
			TLSInsecureSkipVerify: cfg.S3TLSInsecureSkipVerify,
			S3ForcePathStyle:      cfg.S3ForcePathStyle,
			ProfileName:           cfg.ConfigProfile,
			Bucket:                bucket,
			Dir:                   dir,
		}
		if err := fsr.Init(ctx); err != nil {
			return nil, fmt.Errorf("cannot initialize connection to s3: %w", err)
		}
		return fsr, nil
	default:
		return nil, fmt.Errorf("unsupported scheme %q", scheme)
	}
}
