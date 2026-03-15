package zabbixconnector

import (
	"fmt"
	"testing"
)

func BenchmarkRowsUnmarshal(b *testing.B) {
	s := `{"host":{"host":"h1","name":"n1"},"groups":["g1"],"item_tags":[{"tag":"tn1","value":{}},{"tag":"tn2","value":""}],"itemid":1,"name":"in1","clock":1712417868,"ns":425677241,"value":1,"type":0}
{"host":{"host":"h1","name":"n1"},"groups":[],"item_tags":[],"itemid":1,"name":"in1","clock":1712417868,"ns":425677241,"value":1,"type":0}
{"host":{"host":"h1","name":"n1"},"groups":["g1"],"item_tags":[{"tag":"tn1","value":"tv1"},{"tag":"tn1","value":"tv3"},{"tag":"tn1","value":"tv4"},{"tag":"tn2","value":""},{"tag":"tn2","value":"tv2"}],"itemid":1,"name":"in1","clock":1712417868,"ns":425677241,"value":1,"type":0}
`

	*addGroupsValue = "1"
	*addEmptyTagsValue = "1"
	*addDuplicateTagsSeparator = "__"

	defer func() {
		*addGroupsValue = ""
		*addEmptyTagsValue = ""
		*addDuplicateTagsSeparator = ""
	}()

	b.SetBytes(int64(len(s)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var rows Rows
		for pb.Next() {
			rows.Unmarshal(s)
			if len(rows.Rows) != 3 {
				panic(fmt.Errorf("unexpected number of rows parsed; got %d; want 3", len(rows.Rows)))
			}
		}
	})
}
