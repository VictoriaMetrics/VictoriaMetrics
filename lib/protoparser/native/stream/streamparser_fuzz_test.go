package stream

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add(make([]byte, 16))
	f.Add([]byte{0xff, 0, 1})
	f.Add(make([]byte, 20))
	f.Add(make([]byte, 24))
	f.Add(append(make([]byte, 16), 0xff, 0xff, 0xff, 0xff))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_ = Parse(bytes.NewReader(data), "", func(*Block) error { return nil })
	})
}

func FuzzStructuredBlock(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte("cpu"), int16(0), uint8(63))
	f.Add([]byte{}, int16(-3), uint8(0))
	f.Add([]byte{}, int16(3), uint8(31))
	f.Fuzz(func(t *testing.T, data []byte, scale int16, precisionBits uint8) {
		if len(data) > 4096 {
			t.Skip()
		}

		precisionBits = 1 + precisionBits%64

		var (
			mn        storage.MetricName
			tsid      storage.TSID
			timestamp int64
			value     int64
		)
		r := randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3)
		r.Fill(&mn)
		r.Fill(&tsid)
		r.Fill(&timestamp)
		r.Fill(&value)

		var b storage.Block
		b.Init(&tsid, []int64{timestamp}, []int64{value}, scale, precisionBits)
		metric := mn.Marshal(nil)
		block := b.MarshalPortable(nil)
		payload := encoding.MarshalInt64(nil, timestamp)
		payload = encoding.MarshalInt64(payload, timestamp)
		payload = encoding.MarshalUint32(payload, uint32(len(metric)))
		payload = append(payload, metric...)
		payload = encoding.MarshalUint32(payload, uint32(len(block)))
		payload = append(payload, block...)
		count := 0
		err := Parse(bytes.NewReader(payload), "", func(*Block) error {
			count++
			return nil
		})
		if err != nil || count != 1 {
			t.Fatalf("cannot parse structured block: count=%d, err=%v", count, err)
		}
	})
}
