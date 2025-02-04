package logstorage

import (
	"testing"
)

func BenchmarkStreamIDMarshalString(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)

	sid := &streamID{
		tenantID: TenantID{
			AccountID: 123,
			ProjectID: 456,
		},
		id: u128{
			lo: 89,
			hi: 344334,
		},
	}
	b.RunParallel(func(pb *testing.PB) {
		var b []byte
		for pb.Next() {
			b = sid.marshalString(b[:0])
		}
	})
}
