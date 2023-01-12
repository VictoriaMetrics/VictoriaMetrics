package logger

import (
	"strings"
	"testing"
)

func restore[T any](vp *T, v T) {
	*vp = v
}

func TestLogLevelMapping(t *testing.T) {
	defer restore(loggerLevel, *loggerLevel)
	defer restore(&minLogLevel, minLogLevel)

	for lvl := logLevel(0); lvl < levelCount; lvl++ {
		levelFlag := strings.ToUpper(lvl.String())
		loggerLevel = &levelFlag

		setLoggerLevel()
		if minLogLevel != lvl {
			t.Errorf("expected level %q, received level %q", lvl, minLogLevel)
		}
	}
}
