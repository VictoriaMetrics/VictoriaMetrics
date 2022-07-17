package templates

import (
	"strings"
	"testing"
	textTpl "text/template"
)

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
