package templates

import (
	"bytes"
	"fmt"
	"strings"
	textTpl "text/template"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// supported variables are list in https://docs.victoriametrics.com/vmalert/#templating.
const tplHeaders = `{{ $value := .Value }}{{ $labels := .Labels }}{{ $expr := .Expr }}{{ $externalLabels := .ExternalLabels }}{{ $externalURL := .ExternalURL }}{{ $alertID := .AlertID }}{{ $groupID := .GroupID }}{{ $activeAt := .ActiveAt }}{{ $for := .For }}`

// AlertTplData is used to execute templating
type AlertTplData struct {
	Labels   map[string]string
	Value    float64
	Expr     string
	AlertID  uint64
	GroupID  uint64
	ActiveAt time.Time
	For      time.Duration
}

// ValidateTemplates validates the given annotations,
// mock the `query` function during validation.
func ValidateTemplates(annotations map[string]string) error {
	// it's ok to reuse one template for multiple text validations.
	tmpl := GetCurrentTmpl()
	tmpl = tmpl.Funcs(FuncsWithQuery(nil))
	for _, v := range annotations {
		_, err := tmpl.Parse(tplHeaders + v)
		if err != nil {
			return fmt.Errorf("failed to parse text %q into template: %w", v, err)
		}
	}
	return nil
}

// GetCurrentTmpl returns a copy of the current global template
func GetCurrentTmpl() *textTpl.Template {
	tplMu.RLock()
	defer tplMu.RUnlock()
	tmpl, err := masterTmpl.Clone()
	if err != nil {
		logger.Panicf("failed to clone current rule template: %w", err)
	}
	return tmpl
}

type tplData struct {
	AlertTplData
	ExternalLabels map[string]string
	ExternalURL    string
}

// ParseWithFixedHeader parses the text with the fixed tplHeaders into the given template
func ParseWithFixedHeader(text string, tpl *textTpl.Template) (*textTpl.Template, error) {
	return tpl.Parse(tplHeaders + text)
}

// ExecuteWithoutTemplate retrieves the current global templates, parses the text and executes with the given data
func ExecuteWithoutTemplate(q QueryFn, text string, data AlertTplData) (string, error) {
	if !strings.Contains(text, "{{") || !strings.Contains(text, "}}") {
		return text, nil
	}

	var err error
	tmpl := GetCurrentTmpl()
	tmpl = tmpl.Funcs(FuncsWithQuery(q))
	tmpl, err = tmpl.Parse(tplHeaders + text)
	if err != nil {
		return "", fmt.Errorf("failed to parse text %q into template: %w", text, err)

	}
	return ExecuteWithTemplate(data, tmpl)
}

// ExecuteWithTemplate executes with the given template and data
func ExecuteWithTemplate(data AlertTplData, tpl *textTpl.Template) (string, error) {
	fullData := tplData{
		data,
		externalLabels,
		externalURL.String(),
	}
	var buf bytes.Buffer
	// returns the zero value for the map type's element
	tpl.Option("missingkey=zero")
	if err := tpl.Execute(&buf, fullData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}
