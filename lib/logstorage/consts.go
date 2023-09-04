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
