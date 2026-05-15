package unittest

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestUnitTest_InvalidExternalURL(t *testing.T) {
	if os.Getenv("VMALERT_TOOL_TEST_HELPER") == "1" {
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestUnitTest_InvalidExternalURLHelper")
	cmd.Env = append(os.Environ(), "VMALERT_TOOL_TEST_HELPER=1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected subprocess to fail")
	}

	output := out.String()
	want := `failed to parse external URL: parse "http://%": invalid URL escape "%"`
	if !strings.Contains(output, want) {
		t.Fatalf("unexpected output %q; want it to contain %q", output, want)
	}
	if strings.Contains(output, "%!w(") {
		t.Fatalf("unexpected broken formatting in output %q", output)
	}
}

func TestUnitTest_InvalidExternalURLHelper(t *testing.T) {
	if os.Getenv("VMALERT_TOOL_TEST_HELPER") != "1" {
		return
	}
	UnitTest(nil, false, nil, "http://%", "", "")
	t.Fatal("unreachable")
}

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
