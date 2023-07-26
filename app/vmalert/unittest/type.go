package unittest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
)

// parsedSample is a sample with parsed Labels
type parsedSample struct {
	Labels datasource.Labels
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

// LabelAndAnnotation holds labels and annotations
type LabelAndAnnotation struct {
	Labels      datasource.Labels
	Annotations datasource.Labels
}

func (la *LabelAndAnnotation) String() string {
	return "Labels:" + la.Labels.String() + "\nAnnotations:" + la.Annotations.String()
}

// LabelsAndAnnotations is collection of LabelAndAnnotation
type LabelsAndAnnotations []LabelAndAnnotation

func (la LabelsAndAnnotations) Len() int { return len(la) }

func (la LabelsAndAnnotations) Swap(i, j int) { la[i], la[j] = la[j], la[i] }
func (la LabelsAndAnnotations) Less(i, j int) bool {
	diff := datasource.LabelCompare(la[i].Labels, la[j].Labels)
	if diff != 0 {
		return diff < 0
	}
	return datasource.LabelCompare(la[i].Annotations, la[j].Annotations) < 0
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
