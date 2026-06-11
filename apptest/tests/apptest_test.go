package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestMain(m *testing.M) {
	// check if the test is run via make command
	checkApptestEnv()

	// start the integration test.
	os.Exit(m.Run())
}

func checkApptestEnv() {
	if os.Getenv("VM_APPTEST") == "" {
		logger.Warnf("executing apptest with potential outdated binaries. it's recommended to execute via `make apptest` command. check this doc for more details: https://docs.victoriametrics.com/victoriametrics/contributing/#testing")
	}
}
