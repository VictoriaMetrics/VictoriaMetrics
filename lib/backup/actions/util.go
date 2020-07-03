package actions

import (
	"flag"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fsremote"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/gcsremote"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/s3remote"
)

var (
	credsFilePath = flag.String("credsFilePath", "", "Path to file with GCS or S3 credentials. Credentials are loaded from default locations if not set.\n"+
		"See https://cloud.google.com/iam/docs/creating-managing-service-account-keys and https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html")
	configFilePath = flag.String("configFilePath", "", "Path to file with S3 configs. Configs are loaded from default location if not set.\n"+
		"See https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html")
	configProfile    = flag.String("configProfile", "default", "Profile name for S3 configs")
	customS3Endpoint = flag.String("customS3Endpoint", "", "Custom S3 endpoint for use with S3-compatible storages (e.g. MinIO). S3 is used if not set")
)

func runParallel(concurrency int, parts []common.Part, f func(p common.Part) error, progress func(elapsed time.Duration)) error {
	var err error
	runWithProgress(progress, func() {
		err = runParallelInternal(concurrency, parts, f)
	})
	return err
}

func runParallelPerPath(concurrency int, perPath map[string][]common.Part, f func(parts []common.Part) error, progress func(elapsed time.Duration)) error {
	var err error
	runWithProgress(progress, func() {
		err = runParallelPerPathInternal(concurrency, perPath, f)
	})
	return err
}

func runParallelPerPathInternal(concurrency int, perPath map[string][]common.Part, f func(parts []common.Part) error) error {
	if concurrency <= 0 {
		concurrency = 1
	}
	if len(perPath) == 0 {
		return nil
	}

	// len(perPath) capacity guarantees non-blocking behavior below.
	resultCh := make(chan error, len(perPath))
	workCh := make(chan []common.Part, len(perPath))
	stopCh := make(chan struct{})

	// Start workers
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for parts := range workCh {
				select {
				case <-stopCh:
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
			close(stopCh)
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
func NewRemoteFS(path string) (common.RemoteFS, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("path cannot be empty")
	}
	n := strings.Index(path, "://")
	if n < 0 {
		return nil, fmt.Errorf("Missing scheme in path %q. Supported schemes: `gcs://`, `s3://`, `fs://`", path)
	}
	scheme := path[:n]
	dir := path[n+len("://"):]
	switch scheme {
	case "fs":
		if !strings.HasPrefix(dir, "/") {
			return nil, fmt.Errorf("dir must be absolute; got %q", dir)
		}
		fs := &fsremote.FS{
			Dir: dir,
		}
		return fs, nil
	case "gcs":
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
		}
		if err := fs.Init(); err != nil {
			return nil, fmt.Errorf("cannot initialize connection to gcs: %w", err)
		}
		return fs, nil
	case "s3":
		n := strings.Index(dir, "/")
		if n < 0 {
			return nil, fmt.Errorf("missing directory on the s3 bucket %q", dir)
		}
		bucket := dir[:n]
		dir = dir[n:]
		fs := &s3remote.FS{
			CredsFilePath:  *credsFilePath,
			ConfigFilePath: *configFilePath,
			CustomEndpoint: *customS3Endpoint,
			ProfileName:    *configProfile,
			Bucket:         bucket,
			Dir:            dir,
		}
		if err := fs.Init(); err != nil {
			return nil, fmt.Errorf("cannot initialize connection to s3: %w", err)
		}
		return fs, nil
	default:
		return nil, fmt.Errorf("unsupported scheme %q", scheme)
	}
}
