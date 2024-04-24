package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/actions"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fsnil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/snapshot"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/snapshot/snapshotutil"
)

var (
	httpListenAddr    = flag.String("httpListenAddr", ":8420", "TCP address for exporting metrics at /metrics page")
	storageDataPath   = flag.String("storageDataPath", "victoria-metrics-data", "Path to VictoriaMetrics data. Must match -storageDataPath from VictoriaMetrics or vmstorage")
	snapshotName      = flag.String("snapshotName", "", "Name for the snapshot to backup. See https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-work-with-snapshots. There is no need in setting -snapshotName if -snapshot.createURL is set")
	snapshotCreateURL = flag.String("snapshot.createURL", "", "VictoriaMetrics create snapshot url. When this is given a snapshot will automatically be created during backup. "+
		"Example: http://victoriametrics:8428/snapshot/create . There is no need in setting -snapshotName if -snapshot.createURL is set")
	snapshotDeleteURL = flag.String("snapshot.deleteURL", "", "VictoriaMetrics delete snapshot url. Optional. Will be generated from -snapshot.createURL if not provided. "+
		"All created snapshots will be automatically deleted. Example: http://victoriametrics:8428/snapshot/delete")
	dst = flag.String("dst", "", "Where to put the backup on the remote storage. "+
		"Example: gs://bucket/path/to/backup, s3://bucket/path/to/backup, azblob://container/path/to/backup or fs:///path/to/local/backup/dir\n"+
		"-dst can point to the previous backup. In this case incremental backup is performed, i.e. only changed data is uploaded")
	origin            = flag.String("origin", "", "Optional origin directory on the remote storage with old backup for server-side copying when performing full backup. This speeds up full backups")
	concurrency       = flag.Int("concurrency", 10, "The number of concurrent workers. Higher concurrency may reduce backup duration")
	maxBytesPerSecond = flagutil.NewBytes("maxBytesPerSecond", 0, "The maximum upload speed. There is no limit if it is set to 0")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	flagutil.RegisterSecretFlag("snapshot.createURL")
	flagutil.RegisterSecretFlag("snapshot.deleteURL")
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	// Storing snapshot delete function to be able to call it in case
	// of error since logger.Fatal will exit the program without
	// calling deferred functions.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2055
	deleteSnapshot := func() {}

	if len(*snapshotCreateURL) > 0 {
		// create net/url object
		createURL, err := url.Parse(*snapshotCreateURL)
		if err != nil {
			logger.Fatalf("cannot parse snapshotCreateURL: %s", err)
		}
		if len(*snapshotName) > 0 {
			logger.Fatalf("-snapshotName shouldn't be set if -snapshot.createURL is set, since snapshots are created automatically in this case")
		}
		logger.Infof("Snapshot create url %s", createURL.Redacted())
		if len(*snapshotDeleteURL) <= 0 {
			err := flag.Set("snapshot.deleteURL", strings.Replace(*snapshotCreateURL, "/create", "/delete", 1))
			if err != nil {
				logger.Fatalf("Failed to set snapshot.deleteURL flag: %v", err)
			}
		}
		deleteURL, err := url.Parse(*snapshotDeleteURL)
		if err != nil {
			logger.Fatalf("cannot parse snapshotDeleteURL: %s", err)
		}
		logger.Infof("Snapshot delete url %s", deleteURL.Redacted())

		name, err := snapshot.Create(createURL.String())
		if err != nil {
			logger.Fatalf("cannot create snapshot: %s", err)
		}
		err = flag.Set("snapshotName", name)
		if err != nil {
			logger.Fatalf("cannot set snapshotName flag: %v", err)
		}

		deleteSnapshot = func() {
			err := snapshot.Delete(deleteURL.String(), name)
			if err != nil {
				logger.Fatalf("cannot delete snapshot: %s", err)
			}
		}
	}

	listenAddrs := []string{*httpListenAddr}
	go httpserver.Serve(listenAddrs, nil, nil)

	pushmetrics.Init()
	err := makeBackup()
	deleteSnapshot()
	if err != nil {
		logger.Fatalf("cannot create backup: %s", err)
	}
	pushmetrics.Stop()

	startTime := time.Now()
	logger.Infof("gracefully shutting down http server for metrics at %q", listenAddrs)
	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Fatalf("cannot stop http server for metrics: %s", err)
	}
	logger.Infof("successfully shut down http server for metrics in %.3f seconds", time.Since(startTime).Seconds())
}

func makeBackup() error {
	dstFS, err := newDstFS()
	if err != nil {
		return err
	}
	if *snapshotName == "" {
		// Make server-side copy from -origin to -dst
		originFS, err := newRemoteOriginFS()
		if err != nil {
			return err
		}
		a := &actions.RemoteBackupCopy{
			Concurrency: *concurrency,
			Src:         originFS,
			Dst:         dstFS,
		}
		if err := a.Run(); err != nil {
			return err
		}
		originFS.MustStop()
	} else {
		// Make backup from srcFS to -dst
		srcFS, err := newSrcFS()
		if err != nil {
			return err
		}
		originFS, err := newOriginFS()
		if err != nil {
			return err
		}
		a := &actions.Backup{
			Concurrency: *concurrency,
			Src:         srcFS,
			Dst:         dstFS,
			Origin:      originFS,
		}
		if err := a.Run(); err != nil {
			return err
		}
		srcFS.MustStop()
		originFS.MustStop()
	}
	dstFS.MustStop()
	return nil
}

func usage() {
	const s = `
vmbackup performs backups for VictoriaMetrics data from instant snapshots to gcs, s3, azblob
or local filesystem. Backed up data can be restored with vmrestore.

See the docs at https://docs.victoriametrics.com/vmbackup/ .
`
	flagutil.Usage(s)
}

func newSrcFS() (*fslocal.FS, error) {
	if err := snapshotutil.Validate(*snapshotName); err != nil {
		return nil, fmt.Errorf("invalid -snapshotName=%q: %w", *snapshotName, err)
	}
	snapshotPath := filepath.Join(*storageDataPath, "snapshots", *snapshotName)

	// Verify the snapshot exists.
	f, err := os.Open(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open snapshot at %q: %w", snapshotPath, err)
	}
	fi, err := f.Stat()
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot stat %q: %w", snapshotPath, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("snapshot %q must be a directory", snapshotPath)
	}

	fs := &fslocal.FS{
		Dir:               snapshotPath,
		MaxBytesPerSecond: maxBytesPerSecond.IntN(),
	}
	if err := fs.Init(); err != nil {
		return nil, fmt.Errorf("cannot initialize fs: %w", err)
	}
	return fs, nil
}

func newDstFS() (common.RemoteFS, error) {
	fs, err := actions.NewRemoteFS(*dst)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `-dst`=%q: %w", *dst, err)
	}
	if hasFilepathPrefix(*dst, *storageDataPath) {
		return nil, fmt.Errorf("-dst=%q can not point to the directory with VictoriaMetrics data (aka -storageDataPath=%q)", *dst, *storageDataPath)
	}
	return fs, nil
}

func hasFilepathPrefix(path, prefix string) bool {
	if !strings.HasPrefix(path, "fs://") {
		return false
	}
	path = path[len("fs://"):]
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	prefixAbs, err := filepath.Abs(prefix)
	if err != nil {
		return false
	}
	if prefixAbs == pathAbs {
		return true
	}
	rel, err := filepath.Rel(prefixAbs, pathAbs)
	if err != nil {
		// if paths can't be related - they don't match
		return false
	}
	if i := strings.Index(rel, "."); i == 0 {
		// if path can be related only with . as first char - they still don't match
		return false
	}
	// if paths are related - it is a match
	return true
}

func newOriginFS() (common.OriginFS, error) {
	if len(*origin) == 0 {
		return &fsnil.FS{}, nil
	}
	fs, err := actions.NewRemoteFS(*origin)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `-origin`=%q: %w", *origin, err)
	}
	return fs, nil
}

func newRemoteOriginFS() (common.RemoteFS, error) {
	if len(*origin) == 0 {
		return nil, fmt.Errorf("-origin cannot be empty when -snapshotName and -snapshot.createURL aren't set")
	}
	fs, err := actions.NewRemoteFS(*origin)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `-origin`=%q: %w", *origin, err)
	}
	return fs, nil
}
