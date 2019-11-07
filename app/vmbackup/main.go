package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/actions"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	storageDataPath = flag.String("storageDataPath", "victoria-metrics-data", "Path to VictoriaMetrics data. Must match -storageDataPath from VictoriaMetrics or vmstorage")
	snapshotName    = flag.String("snapshotName", "", "Name for the snapshot to backup. See https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#how-to-work-with-snapshots")
	dst             = flag.String("dst", "", "Where to put the backup on the remote storage. "+
		"Example: gcs://bucket/path/to/backup/dir, s3://bucket/path/to/backup/dir or fs:///path/to/local/backup/dir\n"+
		"-dst can point to the previous backup. In this case incremental backup is performed, i.e. only changed data is uploaded")
	origin      = flag.String("origin", "", "Optional origin directory on the remote storage with old backup for server-side copying when performing full backup. This speeds up full backups")
	concurrency = flag.Int("concurrency", 10, "The number of concurrent workers. Higher concurrency may reduce backup duration")
)

func main() {
	flag.Usage = usage
	flag.Parse()
	buildinfo.Init()

	srcFS, err := newSrcFS()
	if err != nil {
		logger.Fatalf("%s", err)
	}
	dstFS, err := newDstFS()
	if err != nil {
		logger.Fatalf("%s", err)
	}
	originFS, err := newOriginFS()
	if err != nil {
		logger.Fatalf("%s", err)
	}
	a := &actions.Backup{
		Concurrency: *concurrency,
		Src:         srcFS,
		Dst:         dstFS,
		Origin:      originFS,
	}
	if err := a.Run(); err != nil {
		logger.Fatalf("cannot create backup: %s", err)
	}
}

func usage() {
	const s = `
vmbackup performs backups for VictoriaMetrics data from instant snapshots to gcs, s3
or local filesystem. Backed up data can be restored with vmrestore.

See the docs at https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmbackup/README.md .
`

	f := flag.CommandLine.Output()
	fmt.Fprintf(f, "%s\n", s)
	flag.PrintDefaults()
}

func newSrcFS() (*fslocal.FS, error) {
	if len(*snapshotName) == 0 {
		return nil, fmt.Errorf("`-snapshotName` cannot be empty")
	}
	snapshotPath := *storageDataPath + "/snapshots/" + *snapshotName

	// Verify the snapshot exists.
	f, err := os.Open(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open snapshot at %q: %s", snapshotPath, err)
	}
	fi, err := f.Stat()
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot stat %q: %s", snapshotPath, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("snapshot %q must be a directory", snapshotPath)
	}

	fs := &fslocal.FS{
		Dir: snapshotPath,
	}
	return fs, nil
}

func newDstFS() (common.RemoteFS, error) {
	fs, err := actions.NewRemoteFS(*dst)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `-dst`=%q: %s", *dst, err)
	}
	return fs, nil
}

func newOriginFS() (common.RemoteFS, error) {
	if len(*origin) == 0 {
		return nil, nil
	}
	fs, err := actions.NewRemoteFS(*origin)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `-origin`=%q: %s", *origin, err)
	}
	return fs, nil
}
