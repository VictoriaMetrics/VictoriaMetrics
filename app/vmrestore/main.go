package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/actions"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	src = flag.String("src", "", "Source path with backup on the remote storage. "+
		"Example: gcs://bucket/path/to/backup/dir, s3://bucket/path/to/backup/dir or fs:///path/to/local/backup/dir")
	storageDataPath = flag.String("storageDataPath", "victoria-metrics-data", "Destination path where backup must be restored. "+
		"VictoriaMetrics must be stopped when restoring from backup. -storageDataPath dir can be non-empty. In this case the contents of -storageDataPath dir "+
		"is synchronized with -src contents, i.e. it works like 'rsync --delete'")
	concurrency             = flag.Int("concurrency", 10, "The number of concurrent workers. Higher concurrency may reduce restore duration")
	maxBytesPerSecond       = flag.Int("maxBytesPerSecond", 0, "The maximum download speed. There is no limit if it is set to 0")
	skipBackupCompleteCheck = flag.Bool("skipBackupCompleteCheck", false, "Whether to skip checking for 'backup complete' file in -src. This may be useful for restoring from old backups, which were created without 'backup complete' file")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()

	srcFS, err := newSrcFS()
	if err != nil {
		logger.Fatalf("%s", err)
	}
	dstFS, err := newDstFS()
	if err != nil {
		logger.Fatalf("%s", err)
	}
	a := &actions.Restore{
		Concurrency:             *concurrency,
		Src:                     srcFS,
		Dst:                     dstFS,
		SkipBackupCompleteCheck: *skipBackupCompleteCheck,
	}
	if err := a.Run(); err != nil {
		logger.Fatalf("cannot restore from backup: %s", err)
	}
}

func usage() {
	const s = `
vmrestore restores VictoriaMetrics data from backups made by vmbackup.

See the docs at https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmrestore/README.md .
`

	f := flag.CommandLine.Output()
	fmt.Fprintf(f, "%s\n", s)
	flag.PrintDefaults()
}

func newDstFS() (*fslocal.FS, error) {
	if len(*storageDataPath) == 0 {
		return nil, fmt.Errorf("`-storageDataPath` cannot be empty")
	}
	fs := &fslocal.FS{
		Dir:               *storageDataPath,
		MaxBytesPerSecond: *maxBytesPerSecond,
	}
	if err := fs.Init(); err != nil {
		return nil, fmt.Errorf("cannot initialize local fs: %w", err)
	}
	return fs, nil
}

func newSrcFS() (common.RemoteFS, error) {
	fs, err := actions.NewRemoteFS(*src)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `-src`=%q: %w", *src, err)
	}
	return fs, nil
}
