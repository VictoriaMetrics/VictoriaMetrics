package kubernetes

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/VictoriaMetrics/easyproto"
)

func BenchmarkParsePodList(b *testing.B) {
	const itemsMultiplier = 1000
	jsonData := newTestJSONPodListWithRepeatedItems(b, mustReadTestFile(b, "pods_list.json"), itemsMultiplier)
	pbData := newTestProtobufPodListWithRepeatedItems(b, mustReadTestFile(b, "pods_list.pb"), itemsMultiplier)

	f := func(name string, parse parseObjectListFunc, data []byte) {
		objectsByKey, _, err := parse(bytes.NewReader(data))
		if err != nil {
			b.Fatalf("cannot parse list: %s", err)
		}
		if len(objectsByKey) != 3*itemsMultiplier {
			b.Fatalf("unexpected number of items; got %d; want %d", len(objectsByKey), 3*itemsMultiplier)
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ReportMetric(float64(len(data)), "response-bytes")
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if _, _, err := parse(bytes.NewReader(data)); err != nil {
						panic(err)
					}
				}
			})
		})
	}
	f("json", parsePodList, jsonData)
	f("protobuf", parsePodListProtobuf, pbData)
}

// newTestJSONPodListWithRepeatedItems returns JSON-encoded list with items from data repeated n times.
//
// Every repeated item gets unique name, so all the items are stored in the parsed list.
func newTestJSONPodListWithRepeatedItems(b *testing.B, data []byte, n int) []byte {
	var list struct {
		Kind       string            `json:"kind"`
		APIVersion string            `json:"apiVersion"`
		Metadata   json.RawMessage   `json:"metadata"`
		Items      []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		b.Fatalf("cannot parse JSON list: %s", err)
	}
	items := list.Items
	list.Items = nil
	for i := 0; i < n; i++ {
		for _, item := range items {
			var m map[string]any
			if err := json.Unmarshal(item, &m); err != nil {
				b.Fatalf("cannot parse JSON item: %s", err)
			}
			metadata := m["metadata"].(map[string]any)
			metadata["name"] = metadata["name"].(string) + "-" + strconv.Itoa(i)
			itemNew, err := json.Marshal(m)
			if err != nil {
				b.Fatalf("cannot marshal JSON item: %s", err)
			}
			list.Items = append(list.Items, itemNew)
		}
	}
	result, err := json.Marshal(&list)
	if err != nil {
		b.Fatalf("cannot marshal JSON list: %s", err)
	}
	return result
}

// newTestProtobufPodListWithRepeatedItems returns protobuf-encoded list with items from data repeated n times.
//
// Every repeated item gets unique name in the same way as newTestJSONPodListWithRepeatedItems does.
func newTestProtobufPodListWithRepeatedItems(b *testing.B, data []byte, n int) []byte {
	kind, raw, err := unwrapProtobufObject(data)
	if err != nil {
		b.Fatalf("cannot unwrap protobuf list: %s", err)
	}
	var listMeta []byte
	var items [][]byte
	var fc easyproto.FieldContext
	for len(raw) > 0 {
		raw, err = fc.NextField(raw)
		if err != nil {
			b.Fatalf("cannot read protobuf list field: %s", err)
		}
		msg, ok := fc.MessageData()
		if !ok {
			b.Fatalf("unexpected protobuf list field #%d", fc.FieldNum)
		}
		switch fc.FieldNum {
		case 1:
			listMeta = msg
		case 2:
			items = append(items, msg)
		}
	}

	var listData []byte
	listData = appendProtobufBytesField(listData, 1, listMeta)
	for i := 0; i < n; i++ {
		suffix := "-" + strconv.Itoa(i)
		for _, item := range items {
			var p Pod
			if err := p.unmarshalProtobuf(item); err != nil {
				b.Fatalf("cannot parse protobuf item: %s", err)
			}
			// Protobuf parsers use the last value for non-repeated fields,
			// so appending the name field to ObjectMeta overrides the name.
			nameField := appendProtobufStringField(nil, 1, p.Metadata.Name+suffix)
			var itemNew []byte
			src := item
			for len(src) > 0 {
				fieldStart := src
				src, err = fc.NextField(src)
				if err != nil {
					b.Fatalf("cannot read protobuf item field: %s", err)
				}
				if fc.FieldNum == 1 {
					metadata, _ := fc.MessageData()
					itemNew = appendProtobufBytesField(itemNew, 1, append(append([]byte{}, metadata...), nameField...))
				} else {
					itemNew = append(itemNew, fieldStart[:len(fieldStart)-len(src)]...)
				}
			}
			listData = appendProtobufBytesField(listData, 2, itemNew)
		}
	}

	var mp easyproto.MarshalerPool
	m := mp.Get()
	defer mp.Put(m)
	mm := m.MessageMarshaler()
	typeMeta := mm.AppendMessage(1)
	typeMeta.AppendString(1, "v1")
	typeMeta.AppendString(2, kind)
	mm.AppendBytes(2, listData)
	mm.AppendString(3, "")
	mm.AppendString(4, "")
	return m.Marshal(append([]byte{}, protobufMagic...))
}

func appendProtobufBytesField(dst []byte, fieldNum uint32, data []byte) []byte {
	var mp easyproto.MarshalerPool
	m := mp.Get()
	defer mp.Put(m)
	m.MessageMarshaler().AppendBytes(fieldNum, data)
	return m.Marshal(dst)
}

func appendProtobufStringField(dst []byte, fieldNum uint32, s string) []byte {
	var mp easyproto.MarshalerPool
	m := mp.Get()
	defer mp.Put(m)
	m.MessageMarshaler().AppendString(fieldNum, s)
	return m.Marshal(dst)
}
