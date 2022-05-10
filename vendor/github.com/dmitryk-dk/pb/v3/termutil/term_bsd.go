//go:build (darwin || freebsd || netbsd || openbsd || dragonfly) && !appengine

package termutil

import "syscall"

const ioctlReadTermios = syscall.TIOCGETA
const ioctlWriteTermios = syscall.TIOCSETA
