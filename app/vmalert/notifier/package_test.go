package notifier

import (
	"net/url"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	u, _ := url.Parse("https://victoriametrics.com/path")
	InitTemplateFunc(u)
	os.Exit(m.Run())
}
