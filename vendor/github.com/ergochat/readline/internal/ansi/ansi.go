//go:build !windows

package ansi

func EnableANSI() error {
	return nil
}
