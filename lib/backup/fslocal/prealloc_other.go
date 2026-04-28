//go:build !linux

package fslocal

const canPreallocate = false

func preallocateFile(_ string, _ int64) error {
	return nil
}
