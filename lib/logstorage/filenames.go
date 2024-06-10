package logstorage

const (
	metaindexFilename     = "metaindex.bin"
	indexFilename         = "index.bin"
	columnsHeaderFilename = "columns_header.bin"
	timestampsFilename    = "timestamps.bin"
	fieldValuesFilename   = "field_values.bin"
	fieldBloomFilename    = "field_bloom.bin"
	messageValuesFilename = "message_values.bin"
	messageBloomFilename  = "message_bloom.bin"

	metadataFilename = "metadata.json"
	partsFilename    = "parts.json"

	streamIDCacheFilename = "stream_id.bin"

	indexdbDirname    = "indexdb"
	datadbDirname     = "datadb"
	cacheDirname      = "cache"
	partitionsDirname = "partitions"
)
