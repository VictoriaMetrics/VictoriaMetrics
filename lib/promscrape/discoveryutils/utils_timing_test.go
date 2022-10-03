package discoveryutils

import (
	"fmt"
	"testing"
)

func BenchmarkSanitizeLabelName(b *testing.B) {
	labelName := "foo-bar/baz/aaaa+bbb"
	expectedLabelNameSanitized := "foo_bar_baz_aaaa_bbb"
	b.SetBytes(1)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			labelNameSanitized := SanitizeLabelName(labelName)
			if labelNameSanitized != expectedLabelNameSanitized {
				panic(fmt.Errorf("unexpected sanitized label name; got %q; want %q", labelNameSanitized, expectedLabelNameSanitized))
			}
		}
	})
}
