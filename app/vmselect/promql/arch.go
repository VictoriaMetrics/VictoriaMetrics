package promql

import "unsafe"

const maxByteSliceLen = 1<<(31+9*(unsafe.Sizeof(int(0))/8)) - 1
