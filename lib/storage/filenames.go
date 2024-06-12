package storage

const (
	metaindexFilename  = "metaindex.bin"
	indexFilename      = "index.bin"
	valuesFilename     = "values.bin"
	timestampsFilename = "timestamps.bin"
	partsFilename      = "parts.json"
	metadataFilename   = "metadata.json"

	appliedRetentionFilename    = "appliedRetention.txt"
	resetCacheOnStartupFilename = "reset_cache_on_startup"

	nodeIDFilename = "node_id.bin"
)

const (
	smallDirname = "small"
	bigDirname   = "big"

	indexdbDirname   = "indexdb"
	dataDirname      = "data"
	metadataDirname  = "metadata"
	snapshotsDirname = "snapshots"
	cacheDirname     = "cache"
)
