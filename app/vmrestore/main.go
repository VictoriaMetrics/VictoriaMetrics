package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/actions"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
)

var (
	httpListenAddr = flag.String("httpListenAddr", ":8421", "TCP address for exporting metrics at /metrics page")
	src            = flag.String("src", "", "Source path with backup on the remote storage. "+
		"Example: gs://bucket/path/to/backup, s3://bucket/path/to/backup, azblob://container/path/to/backup or fs:///path/to/local/backup\n"+
		"Note: If custom S3 endpoint is used, URL should contain only name of the bucket, while hostname of S3 server must be specified via the -customS3Endpoint command-line flag.")
	storageDataPath = flag.String("storageDataPath", "victoria-metrics-data", "Destination path where backup must be restored. "+
		"VictoriaMetrics must be stopped when restoring from backup. -storageDataPath dir can be non-empty. In this case the contents of -storageDataPath dir "+
		"is synchronized with -src contents, i.e. it works like 'rsync --delete'")
	concurrency             = flag.Int("concurrency", 10, "The number of concurrent workers. Higher concurrency may reduce restore duration")
	maxBytesPerSecond       = flagutil.NewBytes("maxBytesPerSecond", 0, "The maximum download speed. There is no limit if it is set to 0")
	skipBackupCompleteCheck = flag.Bool("skipBackupCompleteCheck", false, "Whether to skip checking for 'backup complete' file in -src. This may be useful for restoring from old backups, which were created without 'backup complete' file")
	SkipPreallocation       = flag.Bool("skipFilePreallocation", false, "Whether to skip pre-allocated files. This will likely be slower in most cases, but allows restores to resume mid file on failure")
	restoreSince            = flagutil.NewRetentionDuration("restoreSince", "", "If set, only partitions containing data newer than now()-restoreSince are restored. "+
		"This reduces the download size when only recent data is needed and helps avoid over-provisioning disk space. "+
		"For example, -restoreSince=5d restores only partitions that contain data from the last 5 days. "+
		"Supports s (second), h (hour), d (day), w (week), M (month), y (year) suffixes.")
	restorePartitions = flag.String("restorePartitions", "", "Comma-separated list of partition names in YYYY_MM format to restore from the backup. "+
		"Partitions not in the list are skipped. Non-partition files (metadata, etc.) are always restored. "+
		"Example: -restorePartitions=2024_01,2024_02. If not set, all partitions are restored.")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	listenAddrs := []string{*httpListenAddr}
	go httpserver.Serve(listenAddrs, nil, httpserver.ServeOptions{})

	ctx, cancelFunc := context.WithCancel(context.Background())
	go func() {
		procutil.WaitForSigterm()
		logger.Infof("received stop signal, canceling restore operation")
		cancelFunc()
	}()

	srcFS, err := newSrcFS(ctx)
	if err != nil {
		logger.Fatalf("%s", err)
	}
	dstFS, err := newDstFS()
	if err != nil {
		logger.Fatalf("%s", err)
	}
	var partitionList []string
	if *restorePartitions != "" {
		for _, name := range strings.Split(*restorePartitions, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				partitionList = append(partitionList, name)
			}
		}
	}
	a := &actions.Restore{
		Concurrency:             *concurrency,
		Src:                     srcFS,
		Dst:                     dstFS,
		SkipBackupCompleteCheck: *skipBackupCompleteCheck,
		SkipPreallocation:       *SkipPreallocation,
		RestoreSince:            restoreSince.Duration(),
		RestorePartitions:       partitionList,
	}
	pushmetrics.Init()
	if err := a.Run(ctx); err != nil {
		logger.Fatalf("cannot restore from backup: %s", err)
	}
	pushmetrics.StopAndPush()
	srcFS.MustStop()
	dstFS.MustStop()

	startTime := time.Now()
	logger.Infof("gracefully shutting down http server for metrics at %q", listenAddrs)
	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Fatalf("cannot stop http server for metrics: %s", err)
	}
	logger.Infof("successfully shut down http server for metrics in %.3f seconds", time.Since(startTime).Seconds())
}

func usage() {
	const s = `
vmrestore restores VictoriaMetrics data from backups made by vmbackup.

See the docs at https://docs.victoriametrics.com/victoriametrics/vmrestore/ .
`
	flagutil.Usage(s)
}

func newDstFS() (*fslocal.FS, error) {
	if len(*storageDataPath) == 0 {
		return nil, fmt.Errorf("`-storageDataPath` cannot be empty")
	}
	fs := &fslocal.FS{
		Dir:               *storageDataPath,
		MaxBytesPerSecond: maxBytesPerSecond.IntN(),
	}
	if err := fs.Init(); err != nil {
		return nil, fmt.Errorf("cannot initialize local fs: %w", err)
	}
	return fs, nil
}

func newSrcFS(ctx context.Context) (common.RemoteFS, error) {
	fs, err := actions.NewRemoteFS(ctx, *src, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `-src`=%q: %w", *src, err)
	}
	return fs, nil
}
