package unittest

import (
	"net"
	"testing"
)

func TestUnitTest_Failure(t *testing.T) {
	f := func(files []string) {
		t.Helper()

		failed := UnitTest(files, false, nil, "", "", "")
		if !failed {
			t.Fatalf("expecting failed test")
		}
	}

	f([]string{"./testdata/failed-test-with-missing-rulefile.yaml"})

	f([]string{"./testdata/failed-test.yaml"})
}

func TestUnitTest_Success(t *testing.T) {
	f := func(disableGroupLabel bool, files []string, externalLabels []string, externalURL, httpPort string) {
		t.Helper()

		assertPortFree(t, httpPort)

		failed := UnitTest(files, disableGroupLabel, externalLabels, externalURL, httpPort, "")
		if failed {
			t.Fatalf("unexpected failed test")
		}
	}

	// run multi files with random http port
	f(false, []string{"./testdata/test1.yaml", "./testdata/test2.yaml"}, []string{"cluster=prod"}, "http://grafana:3000", "")

	// disable group label
	// template with null external values
	// specify httpListenAddr
	f(true, []string{"./testdata/disable-group-label.yaml"}, nil, "", "8880")
}

func assertPortFree(t *testing.T, httpPort string) {
	// port would be selected from available ports, no need to check it
	if httpPort == "" {
		return
	}

	if conn, err := net.Dial("tcp", ":"+httpPort); err == nil {
		_ = conn.Close()
		t.Fatalf("port %s is already in use; test cannot continue. Ensure no other process (e.g., vmalert) is listening on this port.", httpPort)
	}
}
