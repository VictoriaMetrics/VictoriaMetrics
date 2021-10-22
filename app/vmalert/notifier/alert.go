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
	// GroupID contains the ID of the parent rules group
	GroupID uint64
	// Name represents Alert name
	Name string
	// Labels is the list of label-value pairs attached to the Alert
	Labels map[string]string
	// Annotations is the list of annotations generated on Alert evaluation
	Annotations map[string]string
	// State represents the current state of the Alert
	State AlertState
	// Expr contains expression that was executed to generate the Alert
	Expr string
	// Start defines the moment of time when Alert has triggered
	Start time.Time
	// End defines the moment of time when Alert supposed to expire
	End time.Time
	// Value stores the value returned from evaluating expression from Expr field
	Value float64
	// ID is the unique identifer for the Alert
	ID uint64
	// Restored is true if Alert was restored after restart
	Restored bool
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

// AlertTplData is used to execute templating
type AlertTplData struct {
	Labels map[string]string
	Value  float64
	Expr   string
}

const tplHeader = `{{ $value := .Value }}{{ $labels := .Labels }}{{ $expr := .Expr }}`

// ExecTemplate executes the Alert template for given
// map of annotations.
// Every alert could have a different datasource, so function
// requires a queryFunction as an argument.
func (a *Alert) ExecTemplate(q QueryFn, annotations map[string]string) (map[string]string, error) {
	tplData := AlertTplData{Value: a.Value, Labels: a.Labels, Expr: a.Expr}
	return templateAnnotations(annotations, tplData, funcsWithQuery(q))
}

// ExecTemplate executes the given template for given annotations map.
func ExecTemplate(q QueryFn, annotations map[string]string, tpl AlertTplData) (map[string]string, error) {
	return templateAnnotations(annotations, tpl, funcsWithQuery(q))
}

// ValidateTemplates validate annotations for possible template error, uses empty data for template population
func ValidateTemplates(annotations map[string]string) error {
	_, err := templateAnnotations(annotations, AlertTplData{
		Labels: map[string]string{},
		Value:  0,
	}, tmplFunc)
	return err
}

func templateAnnotations(annotations map[string]string, data AlertTplData, funcs template.FuncMap) (map[string]string, error) {
	var builder strings.Builder
	var buf bytes.Buffer
	eg := new(utils.ErrGroup)
	r := make(map[string]string, len(annotations))
	for key, text := range annotations {
		buf.Reset()
		builder.Reset()
		builder.Grow(len(tplHeader) + len(text))
		builder.WriteString(tplHeader)
		builder.WriteString(text)
		if err := templateAnnotation(&buf, builder.String(), data, funcs); err != nil {
			r[key] = text
			eg.Add(fmt.Errorf("key %q, template %q: %w", key, text, err))
			continue
		}
		r[key] = buf.String()
	}
	return r, eg.Err()
}

func templateAnnotation(dst io.Writer, text string, data AlertTplData, funcs template.FuncMap) error {
	t := template.New("").Funcs(funcs).Option("missingkey=zero")
	tpl, err := t.Parse(text)
	if err != nil {
		return fmt.Errorf("error parsing annotation: %w", err)
	}
	if err = tpl.Execute(dst, data); err != nil {
		return fmt.Errorf("error evaluating annotation template: %w", err)
	}
	return nil
}
