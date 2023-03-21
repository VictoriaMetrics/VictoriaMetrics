//go:build aix || linux || solaris || zos
// +build aix linux solaris zos

package terminal

import "golang.org/x/sys/unix"

const ioctlReadTermios = unix.TCGETS
