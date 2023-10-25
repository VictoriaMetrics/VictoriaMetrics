package logstorage

// maxUncompressedIndexBlockSize contains the maximum length of uncompressed block with blockHeader entries aka index block.
//
// The real block length can exceed this value by a small percentage because of the block write details.
const maxUncompressedIndexBlockSize = 128 * 1024

// maxUncompressedBlockSize is the maximum size of uncompressed block in bytes.
//
// The real uncompressed block can exceed this value by up to 2 times because of block merge details.
const maxUncompressedBlockSize = 2 * 1024 * 1024

// maxRowsPerBlock is the maximum number of log entries a single block can contain.
const maxRowsPerBlock = 8 * 1024 * 1024

// maxColumnsPerBlock is the maximum number of columns per block.
const maxColumnsPerBlock = 10000

// MaxFieldNameSize is the maximum size in bytes for field name.
//
// Longer field names are truncated during data ingestion to MaxFieldNameSize length.
const MaxFieldNameSize = 128

// maxConstColumnValueSize is the maximum size in bytes for const column value.
//
// Const column values are stored in columnsHeader, which is read every time the corresponding block is scanned during search queries.
// So it is better to store bigger values in regular columns in order to speed up search speed.
const maxConstColumnValueSize = 256

// maxIndexBlockSize is the maximum size of the block with blockHeader entries (aka indexBlock)
const maxIndexBlockSize = 8 * 1024 * 1024

// maxTimestampsBlockSize is the maximum size of timestamps block
const maxTimestampsBlockSize = 8 * 1024 * 1024

// maxValuesBlockSize is the maximum size of values block
const maxValuesBlockSize = 8 * 1024 * 1024

// maxBloomFilterBlockSize is the maximum size of bloom filter block
const maxBloomFilterBlockSize = 8 * 1024 * 1024

// maxColumnsHeaderSize is the maximum size of columnsHeader block
const maxColumnsHeaderSize = 8 * 1024 * 1024

// maxDictSizeBytes is the maximum length of all the keys in the valuesDict.
//
// Dict is stored in columnsHeader, which is read every time the corresponding block is scanned during search qieries.
// So it is better to store bigger values in regular columns in order to speed up search speed.
const maxDictSizeBytes = 256

// maxDictLen is the maximum number of entries in the valuesDict.
//
// it shouldn't exceed 255, since the dict len is marshaled into a single byte.
const maxDictLen = 8
