package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

func TestMain(m *testing.M) {
	u, _ := url.Parse("https://victoriametrics.com/path")
	notifier.InitTemplateFunc(u)
	os.Exit(m.Run())
}

func TestParseGood(t *testing.T) {
	if _, err := Parse([]string{"testdata/*good.rules", "testdata/dir/*good.*"}, true); err != nil {
		t.Errorf("error parsing files %s", err)
	}
}

func TestParseGroupInterval(t *testing.T) {
	groups, err := Parse([]string{"testdata/dir/rules3-group-interval-5m-good.rules"}, true)
	if err != nil {
		t.Errorf("error parsing files %s", err)
	}
<<<<<<< HEAD

=======
>>>>>>> Add Group Interval Update
	for _, group := range groups {
		if strings.Contains(group.Name, "Without") {
			if group.Interval != *evaluationInterval {
				t.Error(fmt.Sprintf("group %q expected to have default interval %v; got %v", group.Name, *evaluationInterval, group.Interval))
			}
		} else if group.Interval != 5*time.Minute {
			t.Error(fmt.Sprintf("group %q expected to have interval %v; got %v", group.Name, 5*time.Minute, group.Interval))
		}
	}
}

func TestParseBad(t *testing.T) {
	if _, err := Parse([]string{"testdata/rules0-bad.rules"}, true); err == nil {
		t.Errorf("expected syntaxt error")
	}
	if _, err := Parse([]string{"testdata/dir/rules0-bad.rules"}, true); err == nil {
		t.Errorf("expected template annotation error")
	}
	if _, err := Parse([]string{"testdata/dir/rules1-bad.rules"}, true); err == nil {
		t.Errorf("expected same group error")
	}
	if _, err := Parse([]string{"testdata/dir/rules2-bad.rules"}, true); err == nil {
		t.Errorf("expected template label error")
	}
	if _, err := Parse([]string{"testdata/*.yaml"}, true); err == nil {
		t.Errorf("expected empty group")
	}
}
