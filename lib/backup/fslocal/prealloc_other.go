//go:build !linux

package fslocal

func preallocateFile(_ string, _ int64) error {
	return nil
}
