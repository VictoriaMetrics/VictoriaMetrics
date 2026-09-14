//go:build !musl
// +build !musl

package gozstd

/*
#cgo LDFLAGS: ${SRCDIR}/libzstd_linux_riscv64.a
*/
import "C"