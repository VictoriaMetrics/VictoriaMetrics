package notifier

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"
)

// Alert the triggered alert
// TODO: Looks like alert name isn't unique
type Alert struct {
	Group       string
	Name        string
	Labels      map[string]string
	Annotations map[string]string
	State       AlertState

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
}

const tplHeader = `{{ $value := .Value }}{{ $labels := .Labels }}`

// ExecTemplate executes the Alert template for give
// map of annotations.
func (a *Alert) ExecTemplate(annotations map[string]string) (map[string]string, error) {
	tplData := alertTplData{Value: a.Value, Labels: a.Labels}
	return templateAnnotations(annotations, tplHeader, tplData)
}

// ValidateAnnotations validate annotations for possible template error, uses empty data for template population
func ValidateAnnotations(annotations map[string]string) error {
	_, err := templateAnnotations(annotations, tplHeader, alertTplData{
		Labels: map[string]string{},
		Value:  0,
	})
	return err
}

func templateAnnotations(annotations map[string]string, header string, data alertTplData) (map[string]string, error) {
	var builder strings.Builder
	var buf bytes.Buffer
	eg := errGroup{}
	r := make(map[string]string, len(annotations))
	for key, text := range annotations {
		r[key] = text
		buf.Reset()
		builder.Reset()
		builder.Grow(len(header) + len(text))
		builder.WriteString(header)
		builder.WriteString(text)
		if err := templateAnnotation(&buf, builder.String(), data); err != nil {
			eg.errs = append(eg.errs, fmt.Sprintf("key %s, template %s:%s", key, text, err))
			continue
		}
		r[key] = buf.String()
	}
	return r, eg.err()
}

func templateAnnotation(dst io.Writer, text string, data alertTplData) error {
	tpl, err := template.New("").Funcs(tmplFunc).Option("missingkey=zero").Parse(text)
	if err != nil {
		return fmt.Errorf("error parsing annotation:%w", err)
	}
	if err = tpl.Execute(dst, data); err != nil {
		return fmt.Errorf("error evaluating annotation template:%w", err)
	}
	return nil
}

type errGroup struct {
	errs []string
}

func (eg *errGroup) err() error {
	if eg == nil || len(eg.errs) == 0 {
		return nil
	}
	return eg
}

func (eg *errGroup) Error() string {
	return fmt.Sprintf("errors:%s", strings.Join(eg.errs, "\n"))
}
