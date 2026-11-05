package main

import (
	"net/url"
	"regexp"
	"testing"

	"gopkg.in/yaml.v2"
	"sigs.k8s.io/randfill"
)

func FuzzParseRawAuthConfig(f *testing.F) {
	f.Add([]byte("users:\n- username: test\n  url_prefix: http://localhost:8428\n"))
	f.Add([]byte("unauthorized_user:\n  url_prefix: http://localhost:8428\n"))
	f.Add([]byte("unauthorized_user:\n  url_prefix: http://localhost:8428\n  metric_labels:\n    home: \"%{HOME}\"\n"))
	f.Add([]byte("users: ["))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_, _ = parseAuthConfig(data)
	})
}

func FuzzParseAuthConfigUsers(f *testing.F) {
	for mode := uint8(0); mode < 3; mode++ {
		f.Add([]byte("test"), mode, false)
		f.Add([]byte("test"), mode, true)
	}

	f.Fuzz(func(t *testing.T, data []byte, mode uint8, withPaths bool) {
		if len(data) > 4096 {
			t.Skip()
		}

		// Skip mutually exclusive credentials; JWT uses a separate parser.
		var ui UserInfo
		r := randfill.NewFromGoFuzz(data).
			NilChance(0.2).
			MaxDepth(5).
			NumElements(0, 3).
			Funcs(
				func(up *URLPrefix, c randfill.Continue) {
					up.vOriginal = "http://localhost:8428/" + url.PathEscape(c.String(20))
				},
				func(h *Header, c randfill.Continue) {
					h.sOriginal = "X-Fuzz: " + c.String(20)
				},
				func(q *QueryArg, c randfill.Continue) {
					q.sOriginal = "key=" + c.String(20)
				},
				func(re *Regex, c randfill.Continue) {
					re.sOriginal = regexp.QuoteMeta(c.String(20))
				},
			)
		r.Fill(&ui)

		b, err := yaml.Marshal(&AuthConfig{Users: []UserInfo{ui}})
		if err != nil {
			return
		}
		parsed, err := parseAuthConfig(b)
		if err != nil {
			return
		}
		_, _ = parseAuthConfigUsers(parsed)
	})
}

func FuzzUnmarshalURLPrefix(f *testing.F) {
	f.Add([]byte("http://localhost:8428"))
	f.Add([]byte("- http://localhost:8428\n- http://localhost:8429\n"))
	f.Add([]byte("[]"))
	f.Add([]byte("[http://localhost:8428, 123]"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		var up URLPrefix
		if err := up.UnmarshalYAML(func(dst any) error { return yaml.Unmarshal(data, dst) }); err != nil {
			return
		}
		if len(up.busOriginal) == 0 {
			t.Fatal("URL prefix has no parsed URLs")
		}
	})
}
