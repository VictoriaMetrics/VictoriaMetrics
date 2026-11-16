package kubernetes

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
	"sigs.k8s.io/randfill"
)

func FuzzKubeConfig(f *testing.F) {
	f.Add([]byte("current-context: test\ncontexts:\n- name: test\n  context: {cluster: test, user: test}\nclusters:\n- name: test\n  cluster: {server: https://localhost:6443}\nusers:\n- name: test\n  user: {token: test}\n"))
	f.Add([]byte("current-context: test\ncontexts: ["))
	f.Add([]byte("null"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		path := filepath.Join(t.TempDir(), "kubeconfig.yml")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _ = newKubeConfig(path)
	})
}

func FuzzBuildKubeConfig(f *testing.F) {
	f.Add("test", "https://localhost:6443")
	f.Add("", "http://localhost:8080")
	f.Add("YQ==", "https://localhost:6443")
	f.Add("invalid", "https://localhost:6443")
	f.Add("4", "https://localhost:6443")
	f.Add("5", "https://localhost:6443")
	f.Add("6", "https://localhost:6443")
	f.Add("7", "https://localhost:6443")

	f.Fuzz(func(t *testing.T, token, server string) {
		if len(token)+len(server) > 4096 {
			t.Skip()
		}

		cfg := Config{
			CurrentContext: "fuzz",
			Contexts:       []configContext{{Name: "fuzz", Context: &Context{Cluster: "fuzz", AuthInfo: "fuzz"}}},
			Clusters:       []configCluster{{Name: "fuzz", Cluster: &Cluster{Server: server}}},
			AuthInfos:      []authInfo{{Name: "fuzz", AuthInfo: &AuthInfo{Token: token}}},
		}
		var mode uint8
		randfill.NewFromGoFuzz([]byte(token)).Fill(&mode)
		switch mode % 8 {
		case 0:
			cfg.Clusters[0].Cluster.CertificateAuthorityData = token
		case 1:
			cfg.AuthInfos[0].AuthInfo.ClientCertificateData = token
		case 2:
			cfg.AuthInfos[0].AuthInfo.ClientKeyData = token
		case 3:
			cfg.AuthInfos[0].AuthInfo.Username = token
		case 4:
			cfg.AuthInfos[0].AuthInfo.Exec = &ExecConfig{}
		case 5:
			cfg.AuthInfos[0].AuthInfo.ImpersonateUID = token
		case 6:
			cfg.AuthInfos[0].AuthInfo.ImpersonateGroups = []string{token}
		case 7:
			cfg.AuthInfos[0].AuthInfo.Password = token
		}
		data, err := yaml.Marshal(&cfg)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "kubeconfig.yml")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _ = newKubeConfig(path)
	})
}
