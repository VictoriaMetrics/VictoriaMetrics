package backupnames

const (
	// RestoreInProgressFilename is the filename for "restore in progress" file
	//
	// This file is created at the beginning of the restore process and is deleted at the end of the restore process.
	// If this file exists, then it is unsafe to read the storage data, since it can be incomplete.
	RestoreInProgressFilename = "restore-in-progress"

	// RestoreMarkFileName is the filename for "restore mark" file.
	// This file is created in backupmanager for starting restore process.
	// It is deleted after successful restore.
	RestoreMarkFileName = "backup_restore.ignore"

	// ProtectMarkFileName is the filename for "protection mark" file.
	// This file is created in backupmanager for protecting backup from deletion via retention policy.
	ProtectMarkFileName = "backup_locked.ignore"

	// BackupCompleteFilename is a filename, which is created in the destination fs when backup is complete.
	BackupCompleteFilename = "backup_complete.ignore"

	// BackupMetadataFilename is a filename, which contains metadata for the backup.
	BackupMetadataFilename = "backup_metadata.ignore"
)
