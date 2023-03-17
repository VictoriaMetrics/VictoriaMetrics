package log

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestOutput(t *testing.T) {
	testOutput := &bytes.Buffer{}
	logger.SetOutputForTests(testOutput)
	defer logger.ResetOutputForTest()

	log := &Logger{}

	mustMatch := func(exp string) {
		t.Helper()
		if exp == "" {
			if testOutput.String() != "" {
				t.Errorf("expected output to be empty; got %q", testOutput.String())
				return
			}
		}
		if !strings.Contains(testOutput.String(), exp) {
			t.Errorf("output %q should contain %q", testOutput.String(), exp)
		}
		fmt.Println(testOutput.String())
		testOutput.Reset()
	}

	log.Warnf("foo")
	mustMatch("foo")

	log.Infof("info %d", 2)
	mustMatch("info 2")

	log.Errorf("error %s %d", "baz", 5)
	mustMatch("error baz 5")

	log.Suppress(true)

	log.Warnf("foo")
	mustMatch("")

	log.Infof("info %d", 2)
	mustMatch("")

	log.Errorf("error %q %d", "baz", 5)
	mustMatch("")

}
