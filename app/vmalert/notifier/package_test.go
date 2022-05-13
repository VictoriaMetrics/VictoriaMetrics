package notifier

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := templates.Load([]string{"testdata/templates/*good.tmpl"}, true); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}
