package mergeset

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type partHeader struct {
	// The number of items the part contains.
	itemsCount uint64

	// The number of blocks the part contains.
	blocksCount uint64

	// The first item in the part.
	firstItem []byte

	// The last item in the part.
	lastItem []byte
}

type partHeaderJSON struct {
	ItemsCount  uint64
	BlocksCount uint64
	FirstItem   hexString
	LastItem    hexString
}

type hexString []byte

func (hs hexString) MarshalJSON() ([]byte, error) {
	h := hex.EncodeToString(hs)
	b := make([]byte, 0, len(h)+2)
	b = append(b, '"')
	b = append(b, h...)
	b = append(b, '"')
	return b, nil
}

func (hs *hexString) UnmarshalJSON(data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("too small data string: got %q; must be at least 2 bytes", data)
	}
	if data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("missing heading and/or tailing quotes in the data string %q", data)
	}
	data = data[1 : len(data)-1]
	b, err := hex.DecodeString(string(data))
	if err != nil {
		return fmt.Errorf("cannot hex-decode %q: %w", data, err)
	}
	*hs = b
	return nil
}

func (ph *partHeader) Reset() {
	ph.itemsCount = 0
	ph.blocksCount = 0
	ph.firstItem = ph.firstItem[:0]
	ph.lastItem = ph.lastItem[:0]
}

func (ph *partHeader) String() string {
	return fmt.Sprintf("partHeader{itemsCount: %d, blocksCount: %d, firstItem: %X, lastItem: %X}",
		ph.itemsCount, ph.blocksCount, ph.firstItem, ph.lastItem)
}

func (ph *partHeader) CopyFrom(src *partHeader) {
	ph.itemsCount = src.itemsCount
	ph.blocksCount = src.blocksCount
	ph.firstItem = append(ph.firstItem[:0], src.firstItem...)
	ph.lastItem = append(ph.lastItem[:0], src.lastItem...)
}

func (ph *partHeader) MustReadMetadata(partPath string) {
	ph.Reset()

	// Read ph fields from metadata.
	metadataPath := filepath.Join(partPath, metadataFilename)
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		logger.Panicf("FATAL: cannot read %q: %s", metadataPath, err)
	}

	var phj partHeaderJSON
	if err := json.Unmarshal(metadata, &phj); err != nil {
		logger.Panicf("FATAL: cannot parse %q: %s", metadataPath, err)
	}

	if phj.ItemsCount <= 0 {
		logger.Panicf("FATAL: part %q cannot contain zero items", partPath)
	}
	ph.itemsCount = phj.ItemsCount

	if phj.BlocksCount <= 0 {
		logger.Panicf("FATAL: part %q cannot contain zero blocks", partPath)
	}
	if phj.BlocksCount > phj.ItemsCount {
		logger.Panicf("FATAL: the number of blocks cannot exceed the number of items in the part %q; got blocksCount=%d, itemsCount=%d",
			partPath, phj.BlocksCount, phj.ItemsCount)
	}
	ph.blocksCount = phj.BlocksCount

	ph.firstItem = append(ph.firstItem[:0], phj.FirstItem...)
	ph.lastItem = append(ph.lastItem[:0], phj.LastItem...)
}

func (ph *partHeader) MustWriteMetadata(partPath string) {
	phj := &partHeaderJSON{
		ItemsCount:  ph.itemsCount,
		BlocksCount: ph.blocksCount,
		FirstItem:   append([]byte{}, ph.firstItem...),
		LastItem:    append([]byte{}, ph.lastItem...),
	}
	metadata, err := json.Marshal(&phj)
	if err != nil {
		logger.Panicf("BUG: cannot marshal partHeader metadata: %s", err)
	}
	metadataPath := filepath.Join(partPath, metadataFilename)
	// There is no need in calling fs.MustWriteAtomic() here,
	// since the file is created only once during part creation
	// and the part directory is synced afterward.
	fs.MustWriteSync(metadataPath, metadata)
}
