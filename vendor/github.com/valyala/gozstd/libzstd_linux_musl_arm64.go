//go:build musl
// +build musl

package gozstd

/*
#cgo LDFLAGS: ${SRCDIR}/libzstd_linux_musl_arm64.a
*/
import "C"
