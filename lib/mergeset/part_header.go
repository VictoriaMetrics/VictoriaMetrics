package mergeset

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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

func (ph *partHeader) ParseFromPath(partPath string) error {
	ph.Reset()

	partPath = filepath.Clean(partPath)

	// Extract encoded part name.
	n := strings.LastIndexByte(partPath, '/')
	if n < 0 {
		return fmt.Errorf("cannot find encoded part name in the path %q", partPath)
	}
	partName := partPath[n+1:]

	// PartName must have the following form:
	// itemsCount_blocksCount_Garbage
	a := strings.Split(partName, "_")
	if len(a) != 3 {
		return fmt.Errorf("unexpected number of substrings in the part name %q: got %d; want %d", partName, len(a), 3)
	}

	// Read itemsCount from partName.
	itemsCount, err := strconv.ParseUint(a[0], 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse itemsCount from partName %q: %w", partName, err)
	}
	ph.itemsCount = itemsCount
	if ph.itemsCount <= 0 {
		return fmt.Errorf("part %q cannot contain zero items", partPath)
	}

	// Read blocksCount from partName.
	blocksCount, err := strconv.ParseUint(a[1], 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse blocksCount from partName %q: %w", partName, err)
	}
	ph.blocksCount = blocksCount
	if ph.blocksCount <= 0 {
		return fmt.Errorf("part %q cannot contain zero blocks", partPath)
	}
	if ph.blocksCount > ph.itemsCount {
		return fmt.Errorf("the number of blocks cannot exceed the number of items in the part %q; got blocksCount=%d, itemsCount=%d",
			partPath, ph.blocksCount, ph.itemsCount)
	}

	// Read other ph fields from metadata.
	metadataPath := partPath + "/metadata.json"
	metadata, err := ioutil.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("cannot read %q: %w", metadataPath, err)
	}

	var phj partHeaderJSON
	if err := json.Unmarshal(metadata, &phj); err != nil {
		return fmt.Errorf("cannot parse %q: %w", metadataPath, err)
	}
	if ph.itemsCount != phj.ItemsCount {
		return fmt.Errorf("invalid ItemsCount in %q; got %d; want %d", metadataPath, phj.ItemsCount, ph.itemsCount)
	}
	ph.blocksCount = phj.BlocksCount
	if ph.blocksCount != phj.BlocksCount {
		return fmt.Errorf("invalid BlocksCount in %q; got %d; want %d", metadataPath, phj.BlocksCount, ph.blocksCount)
	}

	ph.firstItem = append(ph.firstItem[:0], phj.FirstItem...)
	ph.lastItem = append(ph.lastItem[:0], phj.LastItem...)

	return nil
}

func (ph *partHeader) Path(tablePath string, suffix uint64) string {
	tablePath = filepath.Clean(tablePath)
	return fmt.Sprintf("%s/%d_%d_%016X", tablePath, ph.itemsCount, ph.blocksCount, suffix)
}

func (ph *partHeader) WriteMetadata(partPath string) error {
	phj := &partHeaderJSON{
		ItemsCount:  ph.itemsCount,
		BlocksCount: ph.blocksCount,
		FirstItem:   append([]byte{}, ph.firstItem...),
		LastItem:    append([]byte{}, ph.lastItem...),
	}
	metadata, err := json.MarshalIndent(&phj, "", "\t")
	if err != nil {
		return fmt.Errorf("cannot marshal metadata: %w", err)
	}
	metadataPath := partPath + "/metadata.json"
	if err := fs.WriteFileAtomically(metadataPath, metadata); err != nil {
		return fmt.Errorf("cannot create %q: %w", metadataPath, err)
	}
	return nil
}
