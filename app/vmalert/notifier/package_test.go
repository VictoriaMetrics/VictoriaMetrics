package notifier

import (
	"net/url"
	"os"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
)

func TestMain(m *testing.M) {
	if err := templates.Load([]string{"testdata/templates/*good.tmpl"}, url.URL{}); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}
