//go:build arm || vm_ensure_xxx64_alignment

package promql

import (
	"fmt"
	"testing"
	"unsafe"
)

func TestTimeseriesByteSliceToXXX64Alignment(t *testing.T) {
	src := []byte{
		0x18, 0x2d, 0x44, 0x54, 0xfb, 0x21, 0x09, 0x40, // 0
		0x0,                                            // padding to force mis-alignment
		0x18, 0x2d, 0x44, 0x54, 0xfb, 0x21, 0x09, 0x40, // 9
		0x0,                                            // padding to force mis-alignment
		0x18, 0x2d, 0x44, 0x54, 0xfb, 0x21, 0x09, 0x40, // 18
		0x0,                                            // padding to force mis-alignment
		0x18, 0x2d, 0x44, 0x54, 0xfb, 0x21, 0x09, 0x40, // 27
		0x0,                                            // padding to force mis-alignment
		0x18, 0x2d, 0x44, 0x54, 0xfb, 0x21, 0x09, 0x40, // 36
		0x0,                                            // padding to force mis-alignment
		0x18, 0x2d, 0x44, 0x54, 0xfb, 0x21, 0x09, 0x40, // 45
		0x0,                                            // padding to force mis-alignment
		0x18, 0x2d, 0x44, 0x54, 0xfb, 0x21, 0x09, 0x40, // 54
		0x0,                                            // padding to force mis-alignment
		0x18, 0x2d, 0x44, 0x54, 0xfb, 0x21, 0x09, 0x40, // 63
		0x0,                                            // padding to come back to correct alignment
		0x18, 0x2d, 0x44, 0x54, 0xfb, 0x21, 0x09, 0x40, // 72
	}

	for i := 0; i < len(src); i += 9 {
		t.Run(fmt.Sprintf("float64/%d", i), func(t *testing.T) {
			f := byteSliceToFloat64(src[i : i+8])

			if len(f) != 1 {
				t.Fatalf("unexpected length; len(f)=%d", len(f))
			}
			if f[0] != 3.141592653589793 {
				t.Fatalf("unexpected value; f[0]=%v", f[0])
			}
			addr := uintptr(unsafe.Pointer(&f[0]))
			if addr%unsafe.Alignof(f[0]) != 0 {
				t.Fatalf("mis-aligned; &f[0]=%p; mod=%d", &f[0], addr%unsafe.Alignof(f[0]))
			}
		})
		t.Run(fmt.Sprintf("int64/%d", i), func(t *testing.T) {
			f := byteSliceToInt64(src[i : i+8])

			if len(f) != 1 {
				t.Fatalf("unexpected length; len(f)=%d", len(f))
			}
			if f[0] != 4614256656552045848 {
				t.Fatalf("unexpected value; f[0]=%v", f[0])
			}
			addr := uintptr(unsafe.Pointer(&f[0]))
			if addr%unsafe.Alignof(f[0]) != 0 {
				t.Fatalf("mis-aligned; &f[0]=%p; mod=%d", &f[0], addr%unsafe.Alignof(f[0]))
			}
		})
	}
}
