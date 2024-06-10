//go:build go1.15

package logger

import (
	// This is needed for embedding tzdata into binary, so `-loggerTimezone` could work in an app running on a scratch base Docker image.
	// The "time/tzdata" package has been appeared starting from Go1.15 - see https://golang.org/doc/go1.15#time/tzdata
	_ "time/tzdata"
)
