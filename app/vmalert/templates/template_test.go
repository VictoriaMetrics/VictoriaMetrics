package templates

import (
	"math"
	"strings"
	"testing"
	textTpl "text/template"
)

func TestTemplateFuncs_StringConversion(t *testing.T) {
	f := func(funcName, s, resultExpected string) {
		t.Helper()

		funcs := templateFuncs()
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
}

func TestTemplateFuncs_Match(t *testing.T) {
	funcs := templateFuncs()
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
}

func TestTemplateFuncs_Formatting(t *testing.T) {
	f := func(funcName string, p any, resultExpected string) {
		t.Helper()

		funcs := templateFuncs()
		v := funcs[funcName]
		fLocal := v.(func(s any) (string, error))
		result, err := fLocal(p)
		if err != nil {
			t.Fatalf("unexpected error for %s(%f): %s", funcName, p, err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result for %s(%f); got\n%s\nwant\n%s", funcName, p, result, resultExpected)
		}
	}

	f("humanize1024", float64(0), "0")
	f("humanize1024", math.Inf(0), "+Inf")
	f("humanize1024", math.NaN(), "NaN")
	f("humanize1024", float64(127087), "124.1ki")
	f("humanize1024", float64(130137088), "124.1Mi")
	f("humanize1024", float64(133260378112), "124.1Gi")
	f("humanize1024", float64(136458627186688), "124.1Ti")
	f("humanize1024", float64(139733634239168512), "124.1Pi")
	f("humanize1024", float64(143087241460908556288), "124.1Ei")
	f("humanize1024", float64(146521335255970361638912), "124.1Zi")
	f("humanize1024", float64(150037847302113650318245888), "124.1Yi")
	f("humanize1024", float64(153638755637364377925883789312), "1.271e+05Yi")

	f("humanize", float64(127087), "127.1k")
	f("humanize", float64(136458627186688), "136.5T")

	f("humanizeDuration", 1, "1s")
	f("humanizeDuration", 0.2, "200ms")
	f("humanizeDuration", 42000, "11h 40m 0s")
	f("humanizeDuration", 16790555, "194d 8h 2m 35s")

	f("humanizePercentage", 1, "100%")
	f("humanizePercentage", 0.8, "80%")
	f("humanizePercentage", 0.015, "1.5%")

	f("humanizeTimestamp", 1679055557, "2023-03-17 12:19:17 +0000 UTC")
}

func mkTemplate(current, replacement any) textTemplate {
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

func TestTemplatesLoad_Failure(t *testing.T) {
	f := func(pathPatterns []string, expectedErrStr string) {
		t.Helper()

		err := Load(pathPatterns, false)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}

		errStr := err.Error()
		if !strings.Contains(errStr, expectedErrStr) {
			t.Fatalf("the returned error %q doesn't contain %q", errStr, expectedErrStr)
		}
	}

	// load template with syntax error
	f([]string{
		"templates/other/nested/bad0-*.tpl",
		"templates/test/good0-*.tpl",
	}, "failed to parse template glob")
}

func TestTemplatesLoad_Success(t *testing.T) {
	f := func(initialTmpl textTemplate, pathPatterns []string, overwrite bool, expectedTmpl textTemplate) {
		t.Helper()

		masterTmplOrig := masterTmpl
		masterTmpl = initialTmpl
		defer func() {
			masterTmpl = masterTmplOrig
		}()

		if err := Load(pathPatterns, overwrite); err != nil {
			t.Fatalf("cannot load templates: %s", err)
		}

		if !equalTemplates(masterTmpl.replacement, expectedTmpl.replacement) {
			t.Fatalf("unexpected replacement template\ngot\n%+v\nwant\n%+v", masterTmpl.replacement, expectedTmpl.replacement)
		}
		if !equalTemplates(masterTmpl.current, expectedTmpl.current) {
			t.Fatalf("unexpected current template\ngot\n%+v\nwant\n%+v", masterTmpl.current, expectedTmpl.current)
		}
	}

	// non existing path undefined template override
	initialTmpl := mkTemplate(nil, nil)
	pathPatterns := []string{
		"templates/non-existing/good-*.tpl",
		"templates/absent/good-*.tpl",
	}
	overwrite := true
	expectedTmpl := mkTemplate(``, nil)
	f(initialTmpl, pathPatterns, overwrite, expectedTmpl)

	// non existing path defined template override
	initialTmpl = mkTemplate(`
		{{- define "test.1" -}}
			{{- printf "value" -}}
		{{- end -}}
	`, nil)
	pathPatterns = []string{
		"templates/non-existing/good-*.tpl",
		"templates/absent/good-*.tpl",
	}
	overwrite = true
	expectedTmpl = mkTemplate(``, nil)
	f(initialTmpl, pathPatterns, overwrite, expectedTmpl)

	// existing path undefined template override
	initialTmpl = mkTemplate(nil, nil)
	pathPatterns = []string{
		"templates/other/nested/good0-*.tpl",
		"templates/test/good0-*.tpl",
	}
	overwrite = false
	expectedTmpl = mkTemplate(`
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
	`, nil)
	f(initialTmpl, pathPatterns, overwrite, expectedTmpl)

	// existing path defined template override
	initialTmpl = mkTemplate(`
		{{- define "test.1" -}}
			{{ printf "Hello %s!" "world" }}
		{{- end -}}
	`, nil)
	pathPatterns = []string{
		"templates/other/nested/good0-*.tpl",
		"templates/test/good0-*.tpl",
	}
	overwrite = false
	expectedTmpl = mkTemplate(`
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
	`)
	f(initialTmpl, pathPatterns, overwrite, expectedTmpl)
}

func TestTemplatesReload(t *testing.T) {
	f := func(initialTmpl, expectedTmpl textTemplate) {
		t.Helper()

		masterTmplOrig := masterTmpl
		masterTmpl = initialTmpl
		defer func() {
			masterTmpl = masterTmplOrig
		}()

		Reload()

		if !equalTemplates(masterTmpl.replacement, expectedTmpl.replacement) {
			t.Fatalf("unexpected replacement template\ngot\n%+v\nwant\n%+v", masterTmpl.replacement, expectedTmpl.replacement)
		}
		if !equalTemplates(masterTmpl.current, expectedTmpl.current) {
			t.Fatalf("unexpected current template\ngot\n%+v\nwant\n%+v", masterTmpl.current, expectedTmpl.current)
		}
	}

	// empty current and replacement templates
	f(mkTemplate(nil, nil), mkTemplate(nil, nil))

	// empty current template only
	f(mkTemplate(`
		{{- define "test.1" -}}
			{{- printf "value" -}}
		{{- end -}}
	`, nil), mkTemplate(`
		{{- define "test.1" -}}
			{{- printf "value" -}}
		{{- end -}}
	`, nil))

	// empty replacement template only
	f(mkTemplate(nil, `
		{{- define "test.1" -}}
			{{- printf "value" -}}
		{{- end -}}
	`), mkTemplate(`
		{{- define "test.1" -}}
			{{- printf "value" -}}
		{{- end -}}
	`, nil))

	// defined both templates
	f(mkTemplate(`
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
	`), mkTemplate(`
		{{- define "test.1" -}}
			{{- printf "after" -}}
		{{- end -}}
	`, nil))
}
