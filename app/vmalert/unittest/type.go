package unittest

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
)

// parsedSample is a sample with parsed Labels
type parsedSample struct {
	Labels labels
	Value  float64
}

func (ps *parsedSample) String() string {
	return ps.Labels.String() + " " + strconv.FormatFloat(ps.Value, 'E', -1, 64)
}

func parsedSamplesString(pss []parsedSample) string {
	if len(pss) == 0 {
		return "nil"
	}
	s := pss[0].String()
	for _, ps := range pss[1:] {
		s += ", " + ps.String()
	}
	return s
}

type labels []datasource.Label

func (ls labels) Len() int           { return len(ls) }
func (ls labels) Swap(i, j int)      { ls[i], ls[j] = ls[j], ls[i] }
func (ls labels) Less(i, j int) bool { return ls[i].Name < ls[j].Name }

func (ls labels) String() string {
	var b bytes.Buffer

	b.WriteByte('{')
	for i, l := range ls {
		if i > 0 {
			b.WriteByte(',')
			b.WriteByte(' ')
		}
		b.WriteString(l.Name)
		b.WriteByte('=')
		b.WriteString(strconv.Quote(l.Value))
	}
	b.WriteByte('}')
	return b.String()
}

// ConvertToLabels convert map to labels
func ConvertToLabels(m map[string]string) (labelset labels) {
	for k, v := range m {
		labelset = append(labelset, datasource.Label{
			Name:  k,
			Value: v,
		})
	}
	// sort label
	slices.SortFunc(labelset, func(a, b datasource.Label) bool { return a.Name < b.Name })
	return
}

// LabelAndAnnotation holds labels and annotations
type LabelAndAnnotation struct {
	Labels      labels
	Annotations labels
}

func (la *LabelAndAnnotation) String() string {
	return "Labels:" + la.Labels.String() + "\nAnnotations:" + la.Annotations.String()
}

// LabelsAndAnnotations is collection of LabelAndAnnotation
type LabelsAndAnnotations []LabelAndAnnotation

func (la LabelsAndAnnotations) Len() int { return len(la) }

func (la LabelsAndAnnotations) Swap(i, j int) { la[i], la[j] = la[j], la[i] }
func (la LabelsAndAnnotations) Less(i, j int) bool {
	diff := labelCompare(la[i].Labels, la[j].Labels)
	if diff != 0 {
		return diff < 0
	}
	return labelCompare(la[i].Annotations, la[j].Annotations) < 0
}

func (la LabelsAndAnnotations) String() string {
	if len(la) == 0 {
		return "[]"
	}
	s := "[\n0:" + IndentLines("\n"+la[0].String(), "  ")
	for i, l := range la[1:] {
		s += ",\n" + fmt.Sprintf("%d", i+1) + ":" + IndentLines("\n"+l.String(), "  ")
	}
	s += "\n]"

	return s
}

func labelCompare(a, b labels) int {
	l := len(a)
	if len(b) < l {
		l = len(b)
	}

	for i := 0; i < l; i++ {
		if a[i].Name != b[i].Name {
			if a[i].Name < b[i].Name {
				return -1
			}
			return 1
		}
		if a[i].Value != b[i].Value {
			if a[i].Value < b[i].Value {
				return -1
			}
			return 1
		}
	}
	// if all labels so far were in common, the set with fewer labels comes first.
	return len(a) - len(b)
}

// IndentLines prefixes each line in the supplied string with the given "indent" string.
func IndentLines(lines, indent string) string {
	sb := strings.Builder{}
	n := strings.Split(lines, "\n")
	for i, l := range n {
		if i > 0 {
			sb.WriteString(indent)
		}
		sb.WriteString(l)
		if i != len(n)-1 {
			sb.WriteRune('\n')
		}
	}
	return sb.String()
}
