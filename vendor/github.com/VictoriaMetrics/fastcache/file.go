package fastcache

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/golang/snappy"
)

// SaveToFile atomically saves cache data to the given filePath using a single
// CPU core.
//
// SaveToFile may be called concurrently with other operations on the cache.
//
// The saved data may be loaded with LoadFromFile*.
//
// See also SaveToFileConcurrent for faster saving to file.
func (c *Cache) SaveToFile(filePath string) error {
	return c.SaveToFileConcurrent(filePath, 1)
}

// SaveToFileConcurrent saves cache data to the given filePath using concurrency
// CPU cores.
//
// SaveToFileConcurrent may be called concurrently with other operations
// on the cache.
//
// The saved data may be loaded with LoadFromFile*.
//
// See also SaveToFile.
func (c *Cache) SaveToFileConcurrent(filePath string, concurrency int) error {
	// Create dir if it doesn't exist.
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot stat %q: %s", dir, err)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create dir %q: %s", dir, err)
		}
	}

	// Save cache data into a temporary directory.
	tmpDir, err := ioutil.TempDir(dir, "fastcache.tmp.")
	if err != nil {
		return fmt.Errorf("cannot create temporary dir inside %q: %s", dir, err)
	}
	defer func() {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
	}()
	gomaxprocs := runtime.GOMAXPROCS(-1)
	if concurrency <= 0 || concurrency > gomaxprocs {
		concurrency = gomaxprocs
	}
	if err := c.save(tmpDir, concurrency); err != nil {
		return fmt.Errorf("cannot save cache data to temporary dir %q: %s", tmpDir, err)
	}

	// Remove old filePath contents, since os.Rename may return
	// error if filePath dir exists.
	if err := os.RemoveAll(filePath); err != nil {
		return fmt.Errorf("cannot remove old contents at %q: %s", filePath, err)
	}
	if err := os.Rename(tmpDir, filePath); err != nil {
		return fmt.Errorf("cannot move temporary dir %q to %q: %s", tmpDir, filePath, err)
	}
	tmpDir = ""
	return nil
}

// LoadFromFile loads cache data from the given filePath.
//
// See SaveToFile* for saving cache data to file.
func LoadFromFile(filePath string) (*Cache, error) {
	return load(filePath, 0)
}

// LoadFromFileOrNew tries loading cache data from the given filePath.
//
// The function falls back to creating new cache with the given maxBytes
// capacity if error occurs during loading the cache from file.
func LoadFromFileOrNew(filePath string, maxBytes int) *Cache {
	c, err := load(filePath, maxBytes)
	if err == nil {
		return c
	}
	return New(maxBytes)
}

func (c *Cache) save(dir string, workersCount int) error {
	if err := saveMetadata(c, dir); err != nil {
		return err
	}

	// Save buckets by workersCount concurrent workers.
	workCh := make(chan int, workersCount)
	results := make(chan error)
	for i := 0; i < workersCount; i++ {
		go func(workerNum int) {
			results <- saveBuckets(c.buckets[:], workCh, dir, workerNum)
		}(i)
	}
	// Feed workers with work
	for i := range c.buckets[:] {
		workCh <- i
	}
	close(workCh)

	// Read results.
	var err error
	for i := 0; i < workersCount; i++ {
		result := <-results
		if result != nil && err == nil {
			err = result
		}
	}
	return err
}

func load(filePath string, maxBytes int) (*Cache, error) {
	maxBucketChunks, err := loadMetadata(filePath)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 {
		maxBucketBytes := uint64((maxBytes + bucketsCount - 1) / bucketsCount)
		expectedBucketChunks := (maxBucketBytes + chunkSize - 1) / chunkSize
		if maxBucketChunks != expectedBucketChunks {
			return nil, fmt.Errorf("cache file %s contains maxBytes=%d; want %d", filePath, maxBytes, expectedBucketChunks*chunkSize*bucketsCount)
		}
	}

	// Read bucket files from filePath dir.
	d, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %s", filePath, err)
	}
	defer func() {
		_ = d.Close()
	}()
	fis, err := d.Readdir(-1)
	if err != nil {
		return nil, fmt.Errorf("cannot read files from %q: %s", filePath, err)
	}
	results := make(chan error)
	workersCount := 0
	var c Cache
	for _, fi := range fis {
		fn := fi.Name()
		if fi.IsDir() || !dataFileRegexp.MatchString(fn) {
			continue
		}
		workersCount++
		go func(dataPath string) {
			results <- loadBuckets(c.buckets[:], dataPath, maxBucketChunks)
		}(filePath + "/" + fn)
	}
	err = nil
	for i := 0; i < workersCount; i++ {
		result := <-results
		if result != nil && err == nil {
			err = result
		}
	}
	if err != nil {
		return nil, err
	}
	// Initialize buckets, which could be missing due to incomplete or corrupted files in the cache.
	// It is better initializing such buckets instead of returning error, since the rest of buckets
	// contain valid data.
	for i := range c.buckets[:] {
		b := &c.buckets[i]
		if len(b.chunks) == 0 {
			b.chunks = make([][]byte, maxBucketChunks)
			b.m = make(map[uint64]uint64)
		}
	}
	return &c, nil
}

func saveMetadata(c *Cache, dir string) error {
	metadataPath := dir + "/metadata.bin"
	metadataFile, err := os.Create(metadataPath)
	if err != nil {
		return fmt.Errorf("cannot create %q: %s", metadataPath, err)
	}
	defer func() {
		_ = metadataFile.Close()
	}()
	maxBucketChunks := uint64(cap(c.buckets[0].chunks))
	if err := writeUint64(metadataFile, maxBucketChunks); err != nil {
		return fmt.Errorf("cannot write maxBucketChunks=%d to %q: %s", maxBucketChunks, metadataPath, err)
	}
	return nil
}

func loadMetadata(dir string) (uint64, error) {
	metadataPath := dir + "/metadata.bin"
	metadataFile, err := os.Open(metadataPath)
	if err != nil {
		return 0, fmt.Errorf("cannot open %q: %s", metadataPath, err)
	}
	defer func() {
		_ = metadataFile.Close()
	}()
	maxBucketChunks, err := readUint64(metadataFile)
	if err != nil {
		return 0, fmt.Errorf("cannot read maxBucketChunks from %q: %s", metadataPath, err)
	}
	if maxBucketChunks == 0 {
		return 0, fmt.Errorf("invalid maxBucketChunks=0 read from %q", metadataPath)
	}
	return maxBucketChunks, nil
}

var dataFileRegexp = regexp.MustCompile(`^data\.\d+\.bin$`)

func saveBuckets(buckets []bucket, workCh <-chan int, dir string, workerNum int) error {
	dataPath := fmt.Sprintf("%s/data.%d.bin", dir, workerNum)
	dataFile, err := os.Create(dataPath)
	if err != nil {
		return fmt.Errorf("cannot create %q: %s", dataPath, err)
	}
	defer func() {
		_ = dataFile.Close()
	}()
	zw := snappy.NewBufferedWriter(dataFile)
	for bucketNum := range workCh {
		if err := writeUint64(zw, uint64(bucketNum)); err != nil {
			return fmt.Errorf("cannot write bucketNum=%d to %q: %s", bucketNum, dataPath, err)
		}
		if err := buckets[bucketNum].Save(zw); err != nil {
			return fmt.Errorf("cannot save bucket[%d] to %q: %s", bucketNum, dataPath, err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("cannot close snappy.Writer for %q: %s", dataPath, err)
	}
	return nil
}

func loadBuckets(buckets []bucket, dataPath string, maxChunks uint64) error {
	dataFile, err := os.Open(dataPath)
	if err != nil {
		return fmt.Errorf("cannot open %q: %s", dataPath, err)
	}
	defer func() {
		_ = dataFile.Close()
	}()
	zr := snappy.NewReader(dataFile)
	for {
		bucketNum, err := readUint64(zr)
		if err == io.EOF {
			// Reached the end of file.
			return nil
		}
		if bucketNum >= uint64(len(buckets)) {
			return fmt.Errorf("unexpected bucketNum read from %q: %d; must be smaller than %d", dataPath, bucketNum, len(buckets))
		}
		if err := buckets[bucketNum].Load(zr, maxChunks); err != nil {
			return fmt.Errorf("cannot load bucket[%d] from %q: %s", bucketNum, dataPath, err)
		}
	}
}

func (b *bucket) Save(w io.Writer) error {
	b.Clean()

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Store b.idx, b.gen and b.m to w.

	bIdx := b.idx
	bGen := b.gen
	chunksLen := 0
	for _, chunk := range b.chunks {
		if chunk == nil {
			break
		}
		chunksLen++
	}
	kvs := make([]byte, 0, 2*8*len(b.m))
	var u64Buf [8]byte
	for k, v := range b.m {
		binary.LittleEndian.PutUint64(u64Buf[:], k)
		kvs = append(kvs, u64Buf[:]...)
		binary.LittleEndian.PutUint64(u64Buf[:], v)
		kvs = append(kvs, u64Buf[:]...)
	}

	if err := writeUint64(w, bIdx); err != nil {
		return fmt.Errorf("cannot write b.idx: %s", err)
	}
	if err := writeUint64(w, bGen); err != nil {
		return fmt.Errorf("cannot write b.gen: %s", err)
	}
	if err := writeUint64(w, uint64(len(kvs))/2/8); err != nil {
		return fmt.Errorf("cannot write len(b.m): %s", err)
	}
	if _, err := w.Write(kvs); err != nil {
		return fmt.Errorf("cannot write b.m: %s", err)
	}

	// Store b.chunks to w.
	if err := writeUint64(w, uint64(chunksLen)); err != nil {
		return fmt.Errorf("cannot write len(b.chunks): %s", err)
	}
	for chunkIdx := 0; chunkIdx < chunksLen; chunkIdx++ {
		chunk := b.chunks[chunkIdx][:chunkSize]
		if _, err := w.Write(chunk); err != nil {
			return fmt.Errorf("cannot write b.chunks[%d]: %s", chunkIdx, err)
		}
	}

	return nil
}

func (b *bucket) Load(r io.Reader, maxChunks uint64) error {
	if maxChunks == 0 {
		return fmt.Errorf("the number of chunks per bucket cannot be zero")
	}
	bIdx, err := readUint64(r)
	if err != nil {
		return fmt.Errorf("cannot read b.idx: %s", err)
	}
	bGen, err := readUint64(r)
	if err != nil {
		return fmt.Errorf("cannot read b.gen: %s", err)
	}
	kvsLen, err := readUint64(r)
	if err != nil {
		return fmt.Errorf("cannot read len(b.m): %s", err)
	}
	kvsLen *= 2 * 8
	kvs := make([]byte, kvsLen)
	if _, err := io.ReadFull(r, kvs); err != nil {
		return fmt.Errorf("cannot read b.m: %s", err)
	}
	m := make(map[uint64]uint64, kvsLen/2/8)
	for len(kvs) > 0 {
		k := binary.LittleEndian.Uint64(kvs)
		kvs = kvs[8:]
		v := binary.LittleEndian.Uint64(kvs)
		kvs = kvs[8:]
		m[k] = v
	}

	maxBytes := maxChunks * chunkSize
	if maxBytes >= maxBucketSize {
		return fmt.Errorf("too big maxBytes=%d; should be smaller than %d", maxBytes, maxBucketSize)
	}
	chunks := make([][]byte, maxChunks)
	chunksLen, err := readUint64(r)
	if err != nil {
		return fmt.Errorf("cannot read len(b.chunks): %s", err)
	}
	if chunksLen > uint64(maxChunks) {
		return fmt.Errorf("chunksLen=%d cannot exceed maxChunks=%d", chunksLen, maxChunks)
	}
	currChunkIdx := bIdx / chunkSize
	if currChunkIdx > 0 && currChunkIdx >= chunksLen {
		return fmt.Errorf("too big bIdx=%d; should be smaller than %d", bIdx, chunksLen*chunkSize)
	}
	for chunkIdx := uint64(0); chunkIdx < chunksLen; chunkIdx++ {
		chunk := getChunk()
		chunks[chunkIdx] = chunk
		if _, err := io.ReadFull(r, chunk); err != nil {
			// Free up allocated chunks before returning the error.
			for _, chunk := range chunks {
				if chunk != nil {
					putChunk(chunk)
				}
			}
			return fmt.Errorf("cannot read b.chunks[%d]: %s", chunkIdx, err)
		}
	}
	// Adjust len for the chunk pointed by currChunkIdx.
	if chunksLen > 0 {
		chunkLen := bIdx % chunkSize
		chunks[currChunkIdx] = chunks[currChunkIdx][:chunkLen]
	}

	b.mu.Lock()
	for _, chunk := range b.chunks {
		putChunk(chunk)
	}
	b.chunks = chunks
	b.m = m
	b.idx = bIdx
	b.gen = bGen
	b.mu.Unlock()

	return nil
}

func writeUint64(w io.Writer, u uint64) error {
	var u64Buf [8]byte
	binary.LittleEndian.PutUint64(u64Buf[:], u)
	_, err := w.Write(u64Buf[:])
	return err
}

func readUint64(r io.Reader) (uint64, error) {
	var u64Buf [8]byte
	if _, err := io.ReadFull(r, u64Buf[:]); err != nil {
		return 0, err
	}
	u := binary.LittleEndian.Uint64(u64Buf[:])
	return u, nil
}
