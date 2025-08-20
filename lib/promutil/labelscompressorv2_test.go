package promutil

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestLabelsCompressorV2(t *testing.T) {
	lc := NewLabelsCompressorV2()

	labels1 := []prompb.Label{
		{Name: "label1", Value: "value1"},
		{Name: "label2", Value: "value2"},
		{Name: "label3", Value: "value3"},
	}
	labels2 := []prompb.Label{
		{Name: "label3", Value: "value3"},
		{Name: "label4", Value: "value4"},
		{Name: "label5", Value: "value5"},
	}

	compressed1 := lc.Compress(labels1)
	compressed2 := lc.Compress(labels2)

	runtime.GC()
	cleaned := lc.Cleanup()
	if cleaned != 0 {
		t.Fatalf("lc.Cleanup() should've cleaned zero unused labels, got %d", cleaned)
	}

	decompressed1 := compressed1.Decompress()
	if !reflect.DeepEqual(labels1, decompressed1) {
		t.Fatalf("decompressed labels1 do not match original: got %+v, want %+v", decompressed1, labels1)
	}

	compressed1 = Key{}
	runtime.GC()
	cleaned = lc.Cleanup()
	if cleaned != 2 {
		t.Fatalf("lc.Cleanup() should've cleaned two unused labels, got %d", cleaned)
	}

	decompressed2 := compressed2.Decompress()
	if !reflect.DeepEqual(labels2, decompressed2) {
		t.Fatalf("decompressed labels2 do not match original: got %+v, want %+v", decompressed2, labels2)
	}

	compressed2 = Key{}
	runtime.GC()
	cleaned = lc.Cleanup()
	if cleaned != 3 {
		t.Fatalf("lc.Cleanup() should've cleaned two unused labels, got %d", cleaned)
	}
}
