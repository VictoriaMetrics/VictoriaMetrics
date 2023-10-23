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

// labelAndAnnotation holds labels and annotations
type labelAndAnnotation struct {
	Labels      datasource.Labels
	Annotations datasource.Labels
}

func (la *labelAndAnnotation) String() string {
	return "Labels:" + la.Labels.String() + "\nAnnotations:" + la.Annotations.String()
}

// labelsAndAnnotations is collection of LabelAndAnnotation
type labelsAndAnnotations []labelAndAnnotation

func (la labelsAndAnnotations) Len() int { return len(la) }

func (la labelsAndAnnotations) Swap(i, j int) { la[i], la[j] = la[j], la[i] }
func (la labelsAndAnnotations) Less(i, j int) bool {
	diff := datasource.LabelCompare(la[i].Labels, la[j].Labels)
	if diff != 0 {
		return diff < 0
	}
	return datasource.LabelCompare(la[i].Annotations, la[j].Annotations) < 0
}

func (la labelsAndAnnotations) String() string {
	if len(la) == 0 {
		return "[]"
	}
	s := "[\n0:" + indentLines("\n"+la[0].String(), "  ")
	for i, l := range la[1:] {
		s += ",\n" + fmt.Sprintf("%d", i+1) + ":" + indentLines("\n"+l.String(), "  ")
	}
	s += "\n]"

	return s
}

// indentLines prefixes each line in the supplied string with the given "indent" string.
func indentLines(lines, indent string) string {
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
