package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/formatutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// SharedRepo is a backup repository with the shared layout:
//
//	<root>/backups_index.ignore - the list of committed backups
//	<root>/store/...            - immutable files, which are uploaded once and are referenced by many backups
//	<root>/backups/<name>/...   - mutable files, manifest and metadata for the backup with the given name
//
// Only a single process may access the repository at a time.
type SharedRepo struct {
	root  common.RemoteFS
	store common.RemoteFS

	// newFS returns RemoteFS for the given subdirectory of the root.
	newFS func(subdir string) (common.RemoteFS, error)
}

// NewSharedRepo returns SharedRepo for the given remote path.
//
// MustStop must be called on the returned repo when it is no longer needed.
func NewSharedRepo(ctx context.Context, path string) (*SharedRepo, error) {
	for strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	newFS := func(subdir string) (common.RemoteFS, error) {
		return NewRemoteFS(ctx, path+"/"+subdir, nil)
	}
	root, err := NewRemoteFS(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	store, err := newFS("store")
	if err != nil {
		root.MustStop()
		return nil, err
	}
	return &SharedRepo{
		root:  root,
		store: store,
		newFS: newFS,
	}, nil
}

// MustStop stops r.
func (r *SharedRepo) MustStop() {
	r.root.MustStop()
	r.store.MustStop()
}

// String returns human-readable representation of r.
func (r *SharedRepo) String() string {
	return fmt.Sprintf("shared repo %s", r.root)
}

var backupNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

func (r *SharedRepo) openBackupFS(name string) (common.RemoteFS, error) {
	if !backupNameRegexp.MatchString(name) || name == "." || name == ".." {
		return nil, fmt.Errorf("invalid backup name %q; it must match %s", name, backupNameRegexp)
	}
	return r.newFS("backups/" + name)
}

// checkLayout returns an error if r points to a backup with the classic layout.
func (r *SharedRepo) checkLayout() error {
	ok, err := r.root.HasFile(backupnames.BackupCompleteFilename)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("%s contains a backup with the classic layout; use another location for backups with the shared layout", r.root)
	}
	return nil
}

// checkNotSharedRepo returns an error if fs is the root of SharedRepo.
//
// A backup with the classic layout deletes all the unknown parts at its destination,
// so it would delete the parts of all the backups at SharedRepo.
func checkNotSharedRepo(fs common.RemoteFS) error {
	ok, err := fs.HasFile(backupnames.SharedIndexFilename)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("%s contains backups with the shared layout; use another location for backups with the classic layout", fs)
	}
	return nil
}

type sharedIndex struct {
	// Backups contains committed backups from the oldest to the newest.
	Backups []sharedIndexEntry `json:"backups"`
}

type sharedIndexEntry struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func (idx *sharedIndex) has(name string) bool {
	return slices.ContainsFunc(idx.Backups, func(e sharedIndexEntry) bool {
		return e.Name == name
	})
}

// readIndex returns the index for r and whether it exists.
func (r *SharedRepo) readIndex() (*sharedIndex, bool, error) {
	ok, err := r.root.HasFile(backupnames.SharedIndexFilename)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return &sharedIndex{}, false, nil
	}
	data, err := r.root.ReadFile(backupnames.SharedIndexFilename)
	if err != nil {
		return nil, false, fmt.Errorf("cannot read %s at %s: %w", backupnames.SharedIndexFilename, r.root, err)
	}
	var idx sharedIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, false, fmt.Errorf("cannot parse %s at %s: %w", backupnames.SharedIndexFilename, r.root, err)
	}
	return &idx, true, nil
}

func (r *SharedRepo) writeIndex(idx *sharedIndex) error {
	data, err := json.Marshal(idx)
	if err != nil {
		return fmt.Errorf("cannot marshal index: %w", err)
	}
	if err := r.root.CreateFile(backupnames.SharedIndexFilename, data); err != nil {
		return fmt.Errorf("cannot write %s at %s: %w", backupnames.SharedIndexFilename, r.root, err)
	}
	return nil
}

type sharedManifest struct {
	// StoreParts contains parts, which are located in the shared store.
	StoreParts []manifestPart `json:"store_parts"`

	// OwnParts contains parts, which are located in the backup directory.
	OwnParts []manifestPart `json:"own_parts"`
}

type manifestPart struct {
	Path     string `json:"path"`
	FileSize uint64 `json:"file_size"`
	Offset   uint64 `json:"offset"`
	Size     uint64 `json:"size"`
}

func toManifestParts(parts []common.Part) []manifestPart {
	mps := make([]manifestPart, len(parts))
	for i, p := range parts {
		mps[i] = manifestPart{
			Path:     p.Path,
			FileSize: p.FileSize,
			Offset:   p.Offset,
			Size:     p.Size,
		}
	}
	return mps
}

// fromManifestParts returns parts for mps.
//
// ActualSize is set to Size, since the size of every part is verified during the upload.
func fromManifestParts(mps []manifestPart) []common.Part {
	parts := make([]common.Part, len(mps))
	for i, mp := range mps {
		parts[i] = common.Part{
			Path:       mp.Path,
			FileSize:   mp.FileSize,
			Offset:     mp.Offset,
			Size:       mp.Size,
			ActualSize: mp.Size,
		}
	}
	return parts
}

func readManifest(fs common.RemoteFS) (*sharedManifest, error) {
	ok, err := fs.HasFile(backupnames.BackupManifestFilename)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("cannot find %s at %s", backupnames.BackupManifestFilename, fs)
	}
	data, err := fs.ReadFile(backupnames.BackupManifestFilename)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s at %s: %w", backupnames.BackupManifestFilename, fs, err)
	}
	var m sharedManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("cannot parse %s at %s: %w", backupnames.BackupManifestFilename, fs, err)
	}
	return &m, nil
}

func writeManifest(fs common.RemoteFS, m *sharedManifest) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("cannot marshal manifest: %w", err)
	}
	if err := fs.CreateFile(backupnames.BackupManifestFilename, data); err != nil {
		return fmt.Errorf("cannot write %s at %s: %w", backupnames.BackupManifestFilename, fs, err)
	}
	return nil
}

var partDirNameRegexp = regexp.MustCompile(`^[0-9A-F]{16}$`)

// isStorePath returns true if the file at path p must be located in the shared store.
//
// Files inside data part directories never change after creation,
// and part directories have unique names derived from mergeIdx.
// See lib/storage/partition.go and lib/mergeset/table.go.
// Legacy indexdb generation directories have the same name pattern,
// but they contain mutable parts.json.
func isStorePath(p string) bool {
	dir, file := path.Split(p)
	if file == "parts.json" || file == "appliedRetention.txt" {
		return false
	}
	return partDirNameRegexp.MatchString(path.Base(dir))
}

// SharedBackup makes a backup to SharedRepo.
//
// Note that the backup works only for VictoriaMetrics snapshots
// made via `/snapshot/create`. It works improperly on mutable files.
type SharedBackup struct {
	// Concurrency is the number of concurrent workers during the backup.
	Concurrency int

	// Src is backup source.
	Src *fslocal.FS

	// Repo is backup destination.
	Repo *SharedRepo

	// Name is the name for the backup. It must be unique in Repo.
	Name string

	// KeepLastBackups is the number of the newest backups to keep after the backup is complete.
	// All the backups are kept if it is set to 0.
	KeepLastBackups int

	// GCDryRun disables deletion of old backups and unreferenced parts.
	// The parts, which would be deleted, are logged instead.
	GCDryRun bool
}

// Run runs b with the provided settings.
func (b *SharedBackup) Run() error {
	startTime := time.Now()
	concurrency := b.Concurrency
	src := b.Src
	r := b.Repo

	if err := r.checkLayout(); err != nil {
		return err
	}
	idx, idxExists, err := r.readIndex()
	if err != nil {
		return err
	}
	if idx.has(b.Name) {
		return fmt.Errorf("backup %q already exists at %s", b.Name, r)
	}
	storeParts, err := r.store.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list parts at %s: %w", r.store, err)
	}
	if !idxExists && len(storeParts) > 0 {
		// Without the index, GC would delete the parts of all the existing backups.
		return fmt.Errorf("cannot find %s at %s, while %s contains %d parts; the repository is corrupted",
			backupnames.SharedIndexFilename, r.root, r.store, len(storeParts))
	}

	logger.Infof("starting backup %q from %s to %s", b.Name, src, r)
	srcParts, err := src.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list src parts: %w", err)
	}
	var srcStoreParts, srcOwnParts []common.Part
	for _, p := range srcParts {
		if isStorePath(p.Path) {
			srcStoreParts = append(srcStoreParts, p)
		} else {
			srcOwnParts = append(srcOwnParts, p)
		}
	}

	bfs, err := r.openBackupFS(b.Name)
	if err != nil {
		return err
	}
	defer bfs.MustStop()
	if err := bfs.DeleteFile(backupnames.BackupCompleteFilename); err != nil {
		return fmt.Errorf("cannot delete `backup complete` file at %s: %w", bfs, err)
	}
	// The backup directory may contain parts from the previous failed attempt with the same name.
	ownParts, err := bfs.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list parts at %s: %w", bfs, err)
	}

	storeUploadParts := common.PartsDifference(srcStoreParts, storeParts)
	if err := uploadParts(src, r.store, storeUploadParts, concurrency); err != nil {
		return err
	}
	if err := deleteDstParts(bfs, common.PartsDifference(ownParts, srcOwnParts), concurrency); err != nil {
		return fmt.Errorf("cannot delete unneeded parts at %s: %w", bfs, err)
	}
	ownUploadParts := common.PartsDifference(srcOwnParts, ownParts)
	if err := uploadParts(src, bfs, ownUploadParts, concurrency); err != nil {
		return err
	}

	md, err := storeMetadata(src, bfs)
	if err != nil {
		return fmt.Errorf("cannot store backup metadata: %w", err)
	}
	m := &sharedManifest{
		StoreParts: toManifestParts(srcStoreParts),
		OwnParts:   toManifestParts(srcOwnParts),
	}
	if err := writeManifest(bfs, m); err != nil {
		return err
	}
	if err := bfs.CreateFile(backupnames.BackupCompleteFilename, nil); err != nil {
		return fmt.Errorf("cannot create `backup complete` file at %s: %w", bfs, err)
	}
	// The index update commits the backup.
	idx.Backups = append(idx.Backups, sharedIndexEntry{
		Name:      b.Name,
		CreatedAt: md.CreatedAt,
	})
	if err := r.writeIndex(idx); err != nil {
		return err
	}

	backupSize := getPartsSize(srcParts)
	uploadSize := getPartsSize(storeUploadParts) + getPartsSize(ownUploadParts)
	logger.Infof("backup %q from %s to %s is complete; backed up %s in %.3f seconds; uploaded %s",
		b.Name, src, r, formatutil.HumanizeBytes(float64(backupSize)), time.Since(startTime).Seconds(), formatutil.HumanizeBytes(float64(uploadSize)))

	return r.runGC(b.KeepLastBackups, concurrency, b.GCDryRun)
}

// runGC deletes the oldest backups above keepLast and then deletes the store parts,
// which aren't referenced by the remaining backups.
//
// All the backups are kept if keepLast is 0.
func (r *SharedRepo) runGC(keepLast, concurrency int, dryRun bool) error {
	startTime := time.Now()
	idx, ok, err := r.readIndex()
	if err != nil {
		return err
	}
	if !ok || len(idx.Backups) == 0 {
		return fmt.Errorf("refusing to run GC at %s, since it has no committed backups", r)
	}

	if keepLast > 0 && len(idx.Backups) > keepLast {
		n := len(idx.Backups) - keepLast
		expired := idx.Backups[:n]
		idx.Backups = idx.Backups[n:]
		if dryRun {
			for _, e := range expired {
				logger.Infof("GC dry run: would delete backup %q at %s", e.Name, r)
			}
		} else {
			// Update the index before the deletion, so the partially deleted backups are never visible.
			if err := r.writeIndex(idx); err != nil {
				return err
			}
			for _, e := range expired {
				if err := r.deleteBackup(e.Name, concurrency); err != nil {
					return err
				}
			}
		}
	}

	var referenced []common.Part
	for _, e := range idx.Backups {
		bfs, err := r.openBackupFS(e.Name)
		if err != nil {
			return err
		}
		m, err := readManifest(bfs)
		bfs.MustStop()
		if err != nil {
			return fmt.Errorf("cannot read manifest for backup %q: %w", e.Name, err)
		}
		referenced = append(referenced, fromManifestParts(m.StoreParts)...)
	}

	storeParts, err := r.store.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list parts at %s: %w", r.store, err)
	}
	if missing := common.PartsDifference(referenced, storeParts); len(missing) > 0 {
		return fmt.Errorf("%s misses %d parts referenced by backups, for example %s; the repository is corrupted or was modified concurrently",
			r.store, len(missing), &missing[0])
	}

	// Broken parts have ActualSize != Size, so they are never referenced and are deleted here too.
	unreferenced := common.PartsDifference(storeParts, referenced)
	unreferencedSize := formatutil.HumanizeBytes(float64(getPartsSize(unreferenced)))
	if dryRun {
		for _, p := range unreferenced {
			logger.Infof("GC dry run: would delete %s from %s", &p, r.store)
		}
		logger.Infof("GC dry run at %s is complete; would delete %d parts with %s", r, len(unreferenced), unreferencedSize)
		return nil
	}
	if err := deleteDstParts(r.store, unreferenced, concurrency); err != nil {
		return fmt.Errorf("cannot delete unreferenced parts at %s: %w", r.store, err)
	}
	logger.Infof("GC at %s is complete in %.3f seconds; deleted %d parts with %s",
		r, time.Since(startTime).Seconds(), len(unreferenced), unreferencedSize)
	return nil
}

func (r *SharedRepo) deleteBackup(name string, concurrency int) error {
	logger.Infof("deleting backup %q from %s", name, r)
	bfs, err := r.openBackupFS(name)
	if err != nil {
		return err
	}
	defer bfs.MustStop()
	for _, f := range []string{backupnames.BackupCompleteFilename, backupnames.BackupManifestFilename, backupnames.BackupMetadataFilename} {
		if err := bfs.DeleteFile(f); err != nil {
			return fmt.Errorf("cannot delete %s at %s: %w", f, bfs, err)
		}
	}
	parts, err := bfs.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list parts at %s: %w", bfs, err)
	}
	if err := deleteDstParts(bfs, parts, concurrency); err != nil {
		return err
	}
	return bfs.RemoveEmptyDirs()
}

// NewRestoreSrcFS returns read-only RemoteFS for restoring the backup at the given remote path.
//
// The path may point to:
//   - a backup with the classic layout;
//   - the root of SharedRepo. The newest backup is used in this case;
//   - a backup directory inside SharedRepo, e.g. <root>/backups/<name>.
//
// MustStop must be called on the returned fs when it is no longer needed.
func NewRestoreSrcFS(ctx context.Context, path string) (common.RemoteFS, error) {
	fs, err := NewRemoteFS(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	isRepoRoot, err := fs.HasFile(backupnames.SharedIndexFilename)
	if err != nil {
		fs.MustStop()
		return nil, err
	}
	isSharedBackup := false
	if !isRepoRoot {
		isSharedBackup, err = fs.HasFile(backupnames.BackupManifestFilename)
		if err != nil {
			fs.MustStop()
			return nil, err
		}
	}
	if !isRepoRoot && !isSharedBackup {
		return fs, nil
	}
	fs.MustStop()

	repoPath := path
	name := ""
	if isSharedBackup {
		var ok bool
		repoPath, name, ok = splitSharedBackupPath(path)
		if !ok {
			return nil, fmt.Errorf("backup with the shared layout must be located at <root>/backups/<name>; got %q", path)
		}
	}
	repo, err := NewSharedRepo(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name, err = repo.latestBackupName()
		if err != nil {
			repo.MustStop()
			return nil, err
		}
	}
	bfs, err := repo.openBackup(name)
	if err != nil {
		repo.MustStop()
		return nil, err
	}
	bfs.repo = repo
	return bfs, nil
}

// splitSharedBackupPath splits path in the form <root>/backups/<name> into <root> and <name>.
func splitSharedBackupPath(path string) (string, string, bool) {
	for strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	n := strings.LastIndexByte(path, '/')
	if n < 0 {
		return "", "", false
	}
	name := path[n+1:]
	path = path[:n]
	if !strings.HasSuffix(path, "/backups") {
		return "", "", false
	}
	return path[:len(path)-len("/backups")], name, true
}

func (r *SharedRepo) latestBackupName() (string, error) {
	idx, _, err := r.readIndex()
	if err != nil {
		return "", err
	}
	if len(idx.Backups) == 0 {
		return "", fmt.Errorf("%s has no backups", r)
	}
	names := make([]string, len(idx.Backups))
	for i, e := range idx.Backups {
		names[i] = e.Name
	}
	name := names[len(names)-1]
	logger.Infof("using the newest backup %q at %s; available backups: %s; restore other backup by pointing to <root>/backups/<name>",
		name, r, strings.Join(names, ", "))
	return name, nil
}

func (r *SharedRepo) openBackup(name string) (*sharedBackupFS, error) {
	idx, _, err := r.readIndex()
	if err != nil {
		return nil, err
	}
	if !idx.has(name) {
		return nil, fmt.Errorf("cannot find backup %q in %s at %s; the backup is either incomplete or deleted", name, backupnames.SharedIndexFilename, r)
	}
	bfs, err := r.openBackupFS(name)
	if err != nil {
		return nil, err
	}
	m, err := readManifest(bfs)
	if err != nil {
		bfs.MustStop()
		return nil, err
	}
	ownPaths := make(map[string]bool, len(m.OwnParts))
	for _, mp := range m.OwnParts {
		ownPaths[mp.Path] = true
	}
	parts := append(fromManifestParts(m.StoreParts), fromManifestParts(m.OwnParts)...)
	return &sharedBackupFS{
		name:     name,
		store:    r.store,
		backup:   bfs,
		parts:    parts,
		ownPaths: ownPaths,
	}, nil
}

// sharedBackupFS is a read-only RemoteFS for a single backup at SharedRepo.
type sharedBackupFS struct {
	name     string
	store    common.RemoteFS
	backup   common.RemoteFS
	parts    []common.Part
	ownPaths map[string]bool

	// repo is stopped together with the fs if it is set.
	repo *SharedRepo
}

var errSharedBackupReadOnly = errors.New("backup with the shared layout is read-only")

func (fs *sharedBackupFS) MustStop() {
	fs.backup.MustStop()
	if fs.repo != nil {
		fs.repo.MustStop()
	}
}

func (fs *sharedBackupFS) String() string {
	return fmt.Sprintf("shared backup %q at %s", fs.name, fs.backup)
}

func (fs *sharedBackupFS) ListParts() ([]common.Part, error) {
	return slices.Clone(fs.parts), nil
}

func (fs *sharedBackupFS) DownloadPart(p common.Part, w io.Writer) error {
	if fs.ownPaths[p.Path] {
		return fs.backup.DownloadPart(p, w)
	}
	return fs.store.DownloadPart(p, w)
}

func (fs *sharedBackupFS) HasFile(filePath string) (bool, error) {
	return fs.backup.HasFile(filePath)
}

func (fs *sharedBackupFS) ReadFile(filePath string) ([]byte, error) {
	return fs.backup.ReadFile(filePath)
}

func (fs *sharedBackupFS) DeletePart(_ common.Part) error {
	return errSharedBackupReadOnly
}

func (fs *sharedBackupFS) RemoveEmptyDirs() error {
	return errSharedBackupReadOnly
}

func (fs *sharedBackupFS) CopyPart(_ common.OriginFS, _ common.Part) error {
	return errSharedBackupReadOnly
}

func (fs *sharedBackupFS) UploadPart(_ common.Part, _ io.Reader) error {
	return errSharedBackupReadOnly
}

func (fs *sharedBackupFS) DeleteFile(_ string) error {
	return errSharedBackupReadOnly
}

func (fs *sharedBackupFS) CreateFile(_ string, _ []byte) error {
	return errSharedBackupReadOnly
}
