package notifier

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	textTpl "text/template"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
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
	// ActiveAt defines the moment of time when Alert has become active
	ActiveAt time.Time
	// Start defines the moment of time when Alert has become firing
	Start time.Time
	// End defines the moment of time when Alert supposed to expire
	End time.Time
	// ResolvedAt defines the moment when Alert was switched from Firing to Inactive
	ResolvedAt time.Time
	// LastSent defines the moment when Alert was sent last time
	LastSent time.Time
	// KeepFiringSince defines the moment when StateFiring was kept because of `keep_firing_for` instead of real alert
	KeepFiringSince time.Time
	// Value stores the value returned from evaluating expression from Expr field
	Value float64
	// ID is the unique identifier for the Alert
	ID uint64
	// Restored is true if Alert was restored after restart
	Restored bool
	// For defines for how long Alert needs to be active to become StateFiring
	For time.Duration
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
	Labels   map[string]string
	Value    float64
	Expr     string
	AlertID  uint64
	GroupID  uint64
	ActiveAt time.Time
	For      time.Duration
}

var tplHeaders = []string{
	"{{ $value := .Value }}",
	"{{ $labels := .Labels }}",
	"{{ $expr := .Expr }}",
	"{{ $externalLabels := .ExternalLabels }}",
	"{{ $externalURL := .ExternalURL }}",
	"{{ $alertID := .AlertID }}",
	"{{ $groupID := .GroupID }}",
	"{{ $activeAt := .ActiveAt }}",
	"{{ $for := .For }}",
}

// ExecTemplate executes the Alert template for given
// map of annotations.
// Every alert could have a different datasource, so function
// requires a queryFunction as an argument.
func (a *Alert) ExecTemplate(q templates.QueryFn, labels, annotations map[string]string) (map[string]string, error) {
	tplData := AlertTplData{
		Value:    a.Value,
		Labels:   labels,
		Expr:     a.Expr,
		AlertID:  a.ID,
		GroupID:  a.GroupID,
		ActiveAt: a.ActiveAt,
		For:      a.For,
	}
	return ExecTemplate(q, annotations, tplData)
}

// ExecTemplate executes the given template for given annotations map.
func ExecTemplate(q templates.QueryFn, annotations map[string]string, tplData AlertTplData) (map[string]string, error) {
	tmpl, err := templates.GetWithFuncs(templates.FuncsWithQuery(q))
	if err != nil {
		return nil, fmt.Errorf("error cloning template: %w", err)
	}
	return templateAnnotations(annotations, tplData, tmpl, true)
}

// ValidateTemplates validate annotations for possible template error, uses empty data for template population
func ValidateTemplates(annotations map[string]string) error {
	tmpl, err := templates.Get()
	if err != nil {
		return err
	}
	_, err = templateAnnotations(annotations, AlertTplData{
		Labels: map[string]string{},
		Value:  0,
	}, tmpl, false)
	return err
}

func templateAnnotations(annotations map[string]string, data AlertTplData, tmpl *textTpl.Template, execute bool) (map[string]string, error) {
	var builder strings.Builder
	var buf bytes.Buffer
	eg := new(utils.ErrGroup)
	r := make(map[string]string, len(annotations))
	tData := tplData{data, externalLabels, externalURL}
	header := strings.Join(tplHeaders, "")
	for key, text := range annotations {
		buf.Reset()
		builder.Reset()
		builder.Grow(len(header) + len(text))
		builder.WriteString(header)
		builder.WriteString(text)
		if err := templateAnnotation(&buf, builder.String(), tData, tmpl, execute); err != nil {
			r[key] = text
			eg.Add(fmt.Errorf("key %q, template %q: %w", key, text, err))
			continue
		}
		r[key] = buf.String()
	}
	return r, eg.Err()
}

type tplData struct {
	AlertTplData
	ExternalLabels map[string]string
	ExternalURL    string
}

func templateAnnotation(dst io.Writer, text string, data tplData, tmpl *textTpl.Template, execute bool) error {
	tpl, err := tmpl.Clone()
	if err != nil {
		return fmt.Errorf("error cloning template before parse annotation: %w", err)
	}
	// Clone() doesn't copy tpl Options, so we set them manually
	tpl = tpl.Option("missingkey=zero")
	tpl, err = tpl.Parse(text)
	if err != nil {
		return fmt.Errorf("error parsing annotation template: %w", err)
	}
	if !execute {
		return nil
	}
	if err = tpl.Execute(dst, data); err != nil {
		return fmt.Errorf("error evaluating annotation template: %w", err)
	}
	return nil
}

func (a Alert) toPromLabels(relabelCfg *promrelabel.ParsedConfigs) []prompbmarshal.Label {
	var labels []prompbmarshal.Label
	for k, v := range a.Labels {
		labels = append(labels, prompbmarshal.Label{
			Name:  k,
			Value: v,
		})
	}
	if relabelCfg != nil {
		labels = relabelCfg.Apply(labels, 0)
	}
	promrelabel.SortLabels(labels)
	return labels
}
