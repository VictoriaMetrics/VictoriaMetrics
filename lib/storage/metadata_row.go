package storage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func MarshalMetadataRaw(dst []byte, accountID, projectID uint32, m *prompb.MetricMetadata) []byte {
	dstLen := len(dst)

	// Tenant information (accountID and projectID)
	dstSize := dstLen + 8

	// 2 bytes per string + 4 bytes for type
	dstSize += 10
	dstSize += len(m.MetricFamilyName) + len(m.Help) + len(m.Unit)

	dst = bytesutil.ResizeWithCopyMayOverallocate(dst, dstSize)[:dstLen]

	dst = encoding.MarshalUint32(dst, accountID)
	dst = encoding.MarshalUint32(dst, projectID)
	dst = encoding.MarshalUint32(dst, m.Type)
	dst = marshalStringFast(dst, m.MetricFamilyName)
	dst = marshalStringFast(dst, m.Help)
	dst = marshalStringFast(dst, m.Unit)

	return dst
}

func (mr *MetricMetadataRow) UnmarshalMetadataRaw(data []byte) ([]byte, error) {
	if len(data) < 8 {
		return data, fmt.Errorf("data too short for unmarshaling metadata; got %d bytes; want at least 8 bytes", len(data))
	}
	accountID := encoding.UnmarshalUint32(data)
	projectID := encoding.UnmarshalUint32(data[4:])
	data = data[8:]
	mr.AccountID = accountID
	mr.ProjectID = projectID

	if len(data) < 4 {
		return data, fmt.Errorf("data too short for unmarshaling metadata; got %d bytes; want at least 10 bytes", len(data))
	}

	mr.Type = encoding.UnmarshalUint32(data)
	data = data[4:]

	nextString := func() ([]byte, error) {
		size := encoding.UnmarshalUint16(data)
		data = data[2:]
		if len(data) < int(size) {
			return nil, fmt.Errorf("data too short for unmarshaling metric family name; got %d bytes; want %d bytes", len(data), size)
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

func (mr *MetricMetadataRow) Reset() {
	mr.AccountID = 0
	mr.ProjectID = 0
	mr.Type = 0
	mr.MetricFamilyName = mr.MetricFamilyName[:0]
	mr.Help = mr.Help[:0]
	mr.Unit = mr.Unit[:0]
}

func (mr *MetricMetadataRow) String() string {
	return fmt.Sprintf("AccountID: %d, ProjectID: %d, Type: %d, MetricFamilyName: %q, Help: %q, Unit: %q",
		mr.AccountID, mr.ProjectID, mr.Type, mr.MetricFamilyName, mr.Help, mr.Unit)
}

type MetricMetadataRow struct {
	AccountID uint32
	ProjectID uint32

	Type             uint32
	MetricFamilyName []byte
	Help             []byte
	Unit             []byte
}

func UnmarshalMetricMetadataRows(dst []MetricMetadataRow, src []byte, maxRows int) ([]MetricMetadataRow, []byte, error) {
	for len(src) > 0 && maxRows > 0 {
		if len(dst) < cap(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, MetricMetadataRow{})
		}
		mr := &dst[len(dst)-1]
		tail, err := mr.UnmarshalMetadataRaw(src)
		if err != nil {
			return dst, tail, err
		}
		src = tail
		maxRows--
	}
	return dst, src, nil
}
