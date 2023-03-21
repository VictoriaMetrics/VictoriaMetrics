package templates

import (
	"math"
	"strings"
	"testing"
	textTpl "text/template"
)

func TestTemplateFuncs(t *testing.T) {
	funcs := templateFuncs()
	f := func(funcName, s, resultExpected string) {
		t.Helper()
		v := funcs[funcName]
		fLocal := v.(func(s string) string)
		result := fLocal(s)
		if result != resultExpected {
			t.Fatalf("unexpected result for %s(%q); got\n%s\nwant\n%s", funcName, s, result, resultExpected)
		}
	}
	f("title", "foo bar", "Foo Bar")
	f("toUpper", "foo", "FOO")
	f("toLower", "FOO", "foo")
	f("pathEscape", "foo/bar\n+baz", "foo%2Fbar%0A+baz")
	f("queryEscape", "foo+bar\n+baz", "foo%2Bbar%0A%2Bbaz")
	f("jsonEscape", `foo{bar="baz"}`+"\n + 1", `"foo{bar=\"baz\"}\n + 1"`)
	f("quotesEscape", `foo{bar="baz"}`+"\n + 1", `foo{bar=\"baz\"}\n + 1`)
	f("htmlEscape", "foo < 10\nabc", "foo &lt; 10\nabc")
	f("crlfEscape", "foo\nbar\rx", `foo\nbar\rx`)
	f("stripPort", "foo", "foo")
	f("stripPort", "foo:1234", "foo")
	f("stripDomain", "foo.bar.baz", "foo")
	f("stripDomain", "foo.bar:123", "foo:123")

	// check "match" func
	matchFunc := funcs["match"].(func(pattern, s string) (bool, error))
	if _, err := matchFunc("invalid[regexp", "abc"); err == nil {
		t.Fatalf("expecting non-nil error on invalid regexp")
	}
	ok, err := matchFunc("abc", "def")
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ok {
		t.Fatalf("unexpected match")
	}
	ok, err = matchFunc("a.+b", "acsdb")
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if !ok {
		t.Fatalf("unexpected mismatch")
	}

	formatting := func(funcName string, p interface{}, resultExpected string) {
		t.Helper()
		v := funcs[funcName]
		fLocal := v.(func(s interface{}) (string, error))
		result, err := fLocal(p)
		if err != nil {
			t.Fatalf("unexpected error for %s(%f): %s", funcName, p, err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result for %s(%f); got\n%s\nwant\n%s", funcName, p, result, resultExpected)
		}
	}
	formatting("humanize1024", float64(0), "0")
	formatting("humanize1024", math.Inf(0), "+Inf")
	formatting("humanize1024", math.NaN(), "NaN")
	formatting("humanize1024", float64(127087), "124.1ki")
	formatting("humanize1024", float64(130137088), "124.1Mi")
	formatting("humanize1024", float64(133260378112), "124.1Gi")
	formatting("humanize1024", float64(136458627186688), "124.1Ti")
	formatting("humanize1024", float64(139733634239168512), "124.1Pi")
	formatting("humanize1024", float64(143087241460908556288), "124.1Ei")
	formatting("humanize1024", float64(146521335255970361638912), "124.1Zi")
	formatting("humanize1024", float64(150037847302113650318245888), "124.1Yi")
	formatting("humanize1024", float64(153638755637364377925883789312), "1.271e+05Yi")

	formatting("humanize", float64(127087), "127.1k")
	formatting("humanize", float64(136458627186688), "136.5T")

	formatting("humanizeDuration", 1, "1s")
	formatting("humanizeDuration", 0.2, "200ms")
	formatting("humanizeDuration", 42000, "11h 40m 0s")
	formatting("humanizeDuration", 16790555, "194d 8h 2m 35s")

	formatting("humanizePercentage", 1, "100%")
	formatting("humanizePercentage", 0.8, "80%")
	formatting("humanizePercentage", 0.015, "1.5%")

	formatting("humanizeTimestamp", 1679055557, "2023-03-17 12:19:17 +0000 UTC")
}

func mkTemplate(current, replacement interface{}) textTemplate {
	tmpl := textTemplate{}
	if current != nil {
		switch val := current.(type) {
		case string:
			tmpl.current = textTpl.Must(newTemplate().Parse(val))
		}
	}
	if replacement != nil {
		switch val := replacement.(type) {
		case string:
			tmpl.replacement = textTpl.Must(newTemplate().Parse(val))
		}
	}
	return tmpl
}

func equalTemplates(tmpls ...*textTpl.Template) bool {
	var cmp *textTpl.Template
	for i, tmpl := range tmpls {
		if i == 0 {
			cmp = tmpl
		} else {
			if cmp == nil || tmpl == nil {
				if cmp != tmpl {
					return false
				}
				continue
			}
			if len(tmpl.Templates()) != len(cmp.Templates()) {
				return false
			}
			for _, t := range tmpl.Templates() {
				tp := cmp.Lookup(t.Name())
				if tp == nil {
					return false
				}
				if tp.Root.String() != t.Root.String() {
					return false
				}
			}
		}
	}
	return true
}

func TestTemplates_Load(t *testing.T) {
	testCases := []struct {
		name             string
		initialTemplate  textTemplate
		pathPatterns     []string
		overwrite        bool
		expectedTemplate textTemplate
		expErr           string
	}{
		{
			"non existing path undefined template override",
			mkTemplate(nil, nil),
			[]string{
				"templates/non-existing/good-*.tpl",
				"templates/absent/good-*.tpl",
			},
			true,
			mkTemplate(``, nil),
			"",
		},
		{
			"non existing path defined template override",
			mkTemplate(`
				{{- define "test.1" -}}
					{{- printf "value" -}}
				{{- end -}}
			`, nil),
			[]string{
				"templates/non-existing/good-*.tpl",
				"templates/absent/good-*.tpl",
			},
			true,
			mkTemplate(``, nil),
			"",
		},
		{
			"existing path undefined template override",
			mkTemplate(nil, nil),
			[]string{
				"templates/other/nested/good0-*.tpl",
				"templates/test/good0-*.tpl",
			},
			false,
			mkTemplate(`
				{{- define "good0-test.tpl" -}}{{- end -}}
				{{- define "test.0" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
				{{- define "test.1" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
				{{- define "test.2" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
				{{- define "test.3" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
			`, nil),
			"",
		},
		{
			"existing path defined template override",
			mkTemplate(`
				{{- define "test.1" -}}
					{{ printf "Hello %s!" "world" }}
				{{- end -}}
			`, nil),
			[]string{
				"templates/other/nested/good0-*.tpl",
				"templates/test/good0-*.tpl",
			},
			false,
			mkTemplate(`
				{{- define "good0-test.tpl" -}}{{- end -}}
				{{- define "test.0" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
				{{- define "test.1" -}}
					{{ printf "Hello %s!" "world" }}
				{{- end -}}
				{{- define "test.2" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
				{{- define "test.3" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
				`, `
				{{- define "good0-test.tpl" -}}{{- end -}}
				{{- define "test.0" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
				{{- define "test.1" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
				{{- define "test.2" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
				{{- define "test.3" -}}
					{{ printf "Hello %s!" externalURL }}
				{{- end -}}
			`),
			"",
		},
		{
			"load template with syntax error",
			mkTemplate(`
				{{- define "test.1" -}}
					{{ printf "Hello %s!" "world" }}
				{{- end -}}
			`, nil),
			[]string{
				"templates/other/nested/bad0-*.tpl",
				"templates/test/good0-*.tpl",
			},
			false,
			mkTemplate(`
				{{- define "test.1" -}}
					{{ printf "Hello %s!" "world" }}
				{{- end -}}
			`, nil),
			"failed to parse template glob",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			masterTmpl = tc.initialTemplate
			err := Load(tc.pathPatterns, tc.overwrite)
			if tc.expErr == "" && err != nil {
				t.Error("happened error that wasn't expected: %w", err)
			}
			if tc.expErr != "" && err == nil {
				t.Error("%+w", err)
				t.Error("expected error that didn't happened")
			}
			if err != nil && !strings.Contains(err.Error(), tc.expErr) {
				t.Error("%+w", err)
				t.Error("expected string doesn't exist in error message")
			}
			if !equalTemplates(masterTmpl.replacement, tc.expectedTemplate.replacement) {
				t.Fatalf("replacement template is not as expected")
			}
			if !equalTemplates(masterTmpl.current, tc.expectedTemplate.current) {
				t.Fatalf("current template is not as expected")
			}
		})
	}
}

func TestTemplates_Reload(t *testing.T) {
	testCases := []struct {
		name             string
		initialTemplate  textTemplate
		expectedTemplate textTemplate
	}{
		{
			"empty current and replacement templates",
			mkTemplate(nil, nil),
			mkTemplate(nil, nil),
		},
		{
			"empty current template only",
			mkTemplate(`
				{{- define "test.1" -}}
					{{- printf "value" -}}
				{{- end -}}
			`, nil),
			mkTemplate(`
				{{- define "test.1" -}}
					{{- printf "value" -}}
				{{- end -}}
			`, nil),
		},
		{
			"empty replacement template only",
			mkTemplate(nil, `
				{{- define "test.1" -}}
					{{- printf "value" -}}
				{{- end -}}
			`),
			mkTemplate(`
				{{- define "test.1" -}}
					{{- printf "value" -}}
				{{- end -}}
			`, nil),
		},
		{
			"defined both templates",
			mkTemplate(`
				{{- define "test.0" -}}
					{{- printf "value" -}}
				{{- end -}}
				{{- define "test.1" -}}
					{{- printf "before" -}}
				{{- end -}}
			`, `
				{{- define "test.1" -}}
					{{- printf "after" -}}
				{{- end -}}
			`),
			mkTemplate(`
				{{- define "test.1" -}}
					{{- printf "after" -}}
				{{- end -}}
			`, nil),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			masterTmpl = tc.initialTemplate
			Reload()
			if !equalTemplates(masterTmpl.replacement, tc.expectedTemplate.replacement) {
				t.Fatalf("replacement template is not as expected")
			}
			if !equalTemplates(masterTmpl.current, tc.expectedTemplate.current) {
				t.Fatalf("current template is not as expected")
			}
		})
	}
}
