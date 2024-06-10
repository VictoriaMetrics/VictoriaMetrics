package kubernetes

import (
	"bytes"
	"fmt"
	"testing"
)

func BenchmarkPodGetTargetLabels(b *testing.B) {
	r := bytes.NewBufferString(testPodsList)
	objectsByKey, _, err := parsePodList(r)
	if err != nil {
		panic(fmt.Errorf("BUG: unexpected error: %w", err))
	}
	var o object
	for _, srcObject := range objectsByKey {
		o = srcObject
		break
	}
	if o == nil {
		panic(fmt.Errorf("BUG: expecting at least a single pod object"))
	}
	gw := newTestGroupWatcher()
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			labelss := o.getTargetLabels(gw)
			if len(labelss) != 1 {
				panic(fmt.Errorf("BUG: unexpected number of labelss returned: %d; want 1", len(labelss)))
			}
			putLabelssToPool(labelss)
		}
	})
}
