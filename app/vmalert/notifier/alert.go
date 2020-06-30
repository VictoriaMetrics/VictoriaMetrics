package notifier

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
)

// Alert the triggered alert
// TODO: Looks like alert name isn't unique
type Alert struct {
	GroupID     uint64
	Name        string
	Labels      map[string]string
	Annotations map[string]string
	State       AlertState

	Expr  string
	Start time.Time
	End   time.Time
	Value float64
	ID    uint64
}

// AlertState type indicates the Alert state
type AlertState int

const (
	// StateInactive is the state of an alert that is neither firing nor pending.
	StateInactive AlertState = iota
	// StatePending is the state of an alert that has been active for less than
	// the configured threshold duration.
	StatePending
	// StateFiring is the state of an alert that has been active for longer than
	// the configured threshold duration.
	StateFiring
)

// String stringer for AlertState
func (as AlertState) String() string {
	switch as {
	case StateFiring:
		return "firing"
	case StatePending:
		return "pending"
	}
	return "inactive"
}

type alertTplData struct {
	Labels map[string]string
	Value  float64
	Expr   string
}

const tplHeader = `{{ $value := .Value }}{{ $labels := .Labels }}{{ $expr := .Expr }}`

// ExecTemplate executes the Alert template for give
// map of annotations.
func (a *Alert) ExecTemplate(annotations map[string]string) (map[string]string, error) {
	tplData := alertTplData{Value: a.Value, Labels: a.Labels, Expr: a.Expr}
	return templateAnnotations(annotations, tplHeader, tplData)
}

// ValidateTemplates validate annotations for possible template error, uses empty data for template population
func ValidateTemplates(annotations map[string]string) error {
	_, err := templateAnnotations(annotations, tplHeader, alertTplData{
		Labels: map[string]string{},
		Value:  0,
	})
	return err
}

func templateAnnotations(annotations map[string]string, header string, data alertTplData) (map[string]string, error) {
	var builder strings.Builder
	var buf bytes.Buffer
	eg := new(utils.ErrGroup)
	r := make(map[string]string, len(annotations))
	for key, text := range annotations {
		r[key] = text
		buf.Reset()
		builder.Reset()
		builder.Grow(len(header) + len(text))
		builder.WriteString(header)
		builder.WriteString(text)
		if err := templateAnnotation(&buf, builder.String(), data); err != nil {
			eg.Add(fmt.Errorf("key %q, template %q: %w", key, text, err))
			continue
		}
		r[key] = buf.String()
	}
	return r, eg.Err()
}

func templateAnnotation(dst io.Writer, text string, data alertTplData) error {
	tpl, err := template.New("").Funcs(tmplFunc).Option("missingkey=zero").Parse(text)
	if err != nil {
		return fmt.Errorf("error parsing annotation: %w", err)
	}
	if err = tpl.Execute(dst, data); err != nil {
		return fmt.Errorf("error evaluating annotation template: %w", err)
	}
	return nil
}
