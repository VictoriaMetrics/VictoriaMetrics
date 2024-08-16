package vm

import "testing"

func TestAddExtraLabelsToImportPath_Failure(t *testing.T) {
	f := func(path string, extraLabels []string) {
		t.Helper()

		_, err := AddExtraLabelsToImportPath(path, extraLabels)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// bad incorrect format for extra label
	f("/api/v1/import", []string{"label=value", "bad_label_wo_value"})
}

func TestAddExtraLabelsToImportPath_Success(t *testing.T) {
	f := func(path string, extraLabels []string, resultExpected string) {
		t.Helper()

		result, err := AddExtraLabelsToImportPath(path, extraLabels)
		if err != nil {
			t.Fatalf("AddExtraLabelsToImportPath() error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	// ok w/o extra labels
	f("/api/v1/import", nil, "/api/v1/import")

	// ok one extra label
	f("/api/v1/import", []string{"instance=host-1"}, "/api/v1/import?extra_label=instance=host-1")

	// ok two extra labels
	f("/api/v1/import", []string{"instance=host-2", "job=vmagent"}, "/api/v1/import?extra_label=instance=host-2&extra_label=job=vmagent")

	// ok two extra with exist param
	f("/api/v1/import?timeout=50", []string{"instance=host-2", "job=vmagent"}, "/api/v1/import?timeout=50&extra_label=instance=host-2&extra_label=job=vmagent")
}
