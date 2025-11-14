package metricsmetadata

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// Row represents time series metadata record
type Row struct {
	lastWriteTime uint64
	heapIdx       int

	MetricFamilyName []byte
	Help             []byte
	Unit             []byte

	AccountID uint32
	ProjectID uint32
	Type      uint32
}

// MarshalTo serializes Row into provided buffer and returns result
func (mr *Row) MarshalTo(dst []byte) []byte {
	dstLen := len(dst)
	// tenant information (accountID and projectID)
	dstSize := dstLen + 8
	// 2 bytes per string + 4 bytes for type
	dstSize += 10
	dstSize += len(mr.MetricFamilyName) + len(mr.Help) + len(mr.Unit)

	dst = bytesutil.ResizeWithCopyMayOverallocate(dst, dstSize)[:dstLen]

	dst = encoding.MarshalUint32(dst, mr.AccountID)
	dst = encoding.MarshalUint32(dst, mr.ProjectID)
	dst = encoding.MarshalUint32(dst, mr.Type)
	dst = marshalBytesFast(dst, mr.MetricFamilyName)
	dst = marshalBytesFast(dst, mr.Help)
	dst = marshalBytesFast(dst, mr.Unit)
	return dst
}

// Unmarshal parses Row from provided buffer and returns tail buffer
func (mr *Row) Unmarshal(data []byte) ([]byte, error) {
	// accountID + projectID + type + metricFamilyName + help + unit
	// 4 + 4 + 4 + 2 + len(metricFamilyName) + 2 + len(help) + 2 + len(unit)
	if len(data) < 18 {
		return data, fmt.Errorf("data too short for unmarshaling metadata; got %d bytes; want at least 18 bytes", len(data))
	}
	accountID := encoding.UnmarshalUint32(data)
	projectID := encoding.UnmarshalUint32(data[4:])
	data = data[8:]
	mr.AccountID = accountID
	mr.ProjectID = projectID

	mr.Type = encoding.UnmarshalUint32(data)
	data = data[4:]

	nextString := func() ([]byte, error) {
		size := encoding.UnmarshalUint16(data)
		data = data[2:]
		if len(data) < int(size) {
			return nil, fmt.Errorf("string data too short; got %d bytes; want %d bytes", len(data), size)
		}
		val := data[:size]
		data = data[size:]
		return val, nil
	}
	var err error
	mr.MetricFamilyName, err = nextString()
	if err != nil {
		return data, fmt.Errorf("cannot unmarshal metric family name: %w", err)
	}
	mr.Help, err = nextString()
	if err != nil {
		return data, fmt.Errorf("cannot unmarshal help: %w", err)
	}
	mr.Unit, err = nextString()
	if err != nil {
		return data, fmt.Errorf("cannot unmarshal unit: %w", err)
	}

	return data, nil
}

// Reset resets Row
func (mr *Row) Reset() {
	mr.AccountID = 0
	mr.ProjectID = 0
	mr.Type = 0
	mr.MetricFamilyName = mr.MetricFamilyName[:0]
	mr.Help = mr.Help[:0]
	mr.Unit = mr.Unit[:0]
	mr.heapIdx = 0
	mr.lastWriteTime = 0
}

// String implements Stringer interface
func (mr *Row) String() string {
	return fmt.Sprintf("AccountID: %d, ProjectID: %d, Type: %d, MetricFamilyName: %q, Help: %q, Unit: %q",
		mr.AccountID, mr.ProjectID, mr.Type, mr.MetricFamilyName, mr.Help, mr.Unit)
}

// UnmarshalRows parses Rows from provided buffer according to the maxRows
//
// returns parsed Rows and tails buffer if maxRows value was reached
func UnmarshalRows(dst []Row, src []byte, maxRows int) ([]Row, []byte, error) {
	for len(src) > 0 && maxRows > 0 {
		if len(dst) < cap(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, Row{})
		}
		mr := &dst[len(dst)-1]
		tail, err := mr.Unmarshal(src)
		if err != nil {
			return dst, tail, err
		}
		src = tail
		maxRows--
	}
	return dst, src, nil
}

func marshalBytesFast(dst []byte, s []byte) []byte {
	dst = encoding.MarshalUint16(dst, uint16(len(s)))
	dst = append(dst, s...)
	return dst
}
