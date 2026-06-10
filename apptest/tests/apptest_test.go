package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestMain(m *testing.M) {
	// check if necessary binaries are there.
	checkBinaryRequirement("../../bin/victoria-metrics-race")
	checkBinaryRequirement("../../bin/vmagent-race")
	checkBinaryRequirement("../../bin/vmauth-race")
	checkBinaryRequirement("../../bin/vmctl-race")
	checkBinaryRequirement("../../bin/vmbackup-race")
	checkBinaryRequirement("../../bin/vmrestore-race")

	// check if the test is run via make command
	checkIntegrationTestEnv()

	// start the integration test.
	os.Exit(m.Run())
}

// checkBinaryRequirement panic if required binary not exist.
func checkBinaryRequirement(path string) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			panic(fmt.Sprintf("integration test failed: %s not found. please run `make integration-test` to execute integration tests. check how different tests are executed: https://docs.victoriametrics.com/victoriametrics/contributing/#testing", path))
		}
	}
}

func checkIntegrationTestEnv() {
	if os.Getenv("VM_INTEGRATION_TEST") == "" {
		logger.Warnf("executing integration tests with potential outdated binaries. it's recommended to execute via `make integration-test` command. check this doc for more details: https://docs.victoriametrics.com/victoriametrics/contributing/#testing")
	}
}
