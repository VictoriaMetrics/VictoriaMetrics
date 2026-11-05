package notifier

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseConfig(f *testing.F) {
	f.Add([]byte("static_configs:\n- targets: [localhost:9093]\n"))
	f.Add([]byte("static_configs:\n- targets: [localhost:9093]\n  alert_relabel_configs:\n  - action: keep\n    regex: up\n"))
	f.Add([]byte("dns_sd_configs:\n- names: [localhost]\n  type: A\n"))
	f.Add([]byte("consul_sd_configs:\n- server: localhost:8500\n  services: [alertmanager]\n"))
	f.Add([]byte("static_configs: ["))
	f.Add([]byte("null"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		path := filepath.Join(t.TempDir(), "notifier.yml")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _ = parseConfig(path)
	})
}
