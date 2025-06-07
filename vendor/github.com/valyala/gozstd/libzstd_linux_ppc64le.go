//go:build !musl
// +build !musl

package gozstd

/*
#cgo LDFLAGS: ${SRCDIR}/libzstd_linux_ppc64le.a
*/
import "C"
