package backupnames

const (
	// RestoreInProgressFilename is the filename for "restore in progress" file
	//
	// This file is created at the beginning of the restore process and is deleted at the end of the restore process.
	// If this file exists, then it is unsafe to read the storage data, since it can be incomplete.
	RestoreInProgressFilename = "restore-in-progress"
)
