package logstorage

const (
	columnNamesFilename        = "column_names.bin"
	columnIdxsFilename         = "column_idxs.bin"
	metaindexFilename          = "metaindex.bin"
	indexFilename              = "index.bin"
	columnsHeaderIndexFilename = "columns_header_index.bin"
	columnsHeaderFilename      = "columns_header.bin"
	timestampsFilename         = "timestamps.bin"
	oldValuesFilename          = "field_values.bin"
	oldBloomFilename           = "field_bloom.bin"
	valuesFilename             = "values.bin"
	bloomFilename              = "bloom.bin"
	messageValuesFilename      = "message_values.bin"
	messageBloomFilename       = "message_bloom.bin"

	// Filenames for per-row marker index and data (e.g. delete markers).
	rowmarkerIdxFilename = "rowmarker.idx"
	rowmarkerDatFilename = "rowmarker.dat"

	metadataFilename = "metadata.json"
	partsFilename    = "parts.json"

	indexdbDirname    = "indexdb"
	datadbDirname     = "datadb"
	partitionsDirname = "partitions"
)
