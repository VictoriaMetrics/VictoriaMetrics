package discoveryutils

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// SanitizeLabelName replaces anything that doesn't match
// client_label.LabelNameRE with an underscore.
//
// This has been copied from Prometheus sources at util/strutil/strconv.go
func SanitizeLabelName(name string) string {
	return labelNamesSanitizer.Transform(name)
}

var labelNamesSanitizer = bytesutil.NewFastStringTransformer(func(s string) string {
	return invalidLabelCharRE.ReplaceAllString(s, "_")
})

var invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// JoinHostPort returns host:port.
//
// Host may be dns name, ipv4 or ipv6 address.
func JoinHostPort(host string, port int) string {
	bb := bbPool.Get()
	b := bb.B[:0]
	isIPv6 := strings.IndexByte(host, ':') >= 0
	if isIPv6 {
		b = append(b, '[')
	}
	b = append(b, host...)
	if isIPv6 {
		b = append(b, ']')
	}
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(port), 10)
	s := bytesutil.InternBytes(b)
	bb.B = b
	bbPool.Put(bb)
	return s
}

var bbPool bytesutil.ByteBufferPool

// TestEqualLabelss tests whether got are equal to want.
func TestEqualLabelss(t *testing.T, got, want []*promutils.Labels) {
	t.Helper()
	var gotCopy []*promutils.Labels
	for _, labels := range got {
		labels = labels.Clone()
		labels.Sort()
		gotCopy = append(gotCopy, labels)
	}
	if !reflect.DeepEqual(gotCopy, want) {
		t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", gotCopy, want)
	}
}

// AddTagsToLabels adds <prefix>_tags (separated with tagSeparator) to labels
// and exposes individual tags via <prefix>_tag_* labels, so users could move all the tags
// into the discovered scrape target with the following relabeling rule in the way similar to kubernetes_sd_configs:
//
//   - action: labelmap
//     regex: <prefix>_tag_(.+)
//
// This solves https://stackoverflow.com/questions/44339461/relabeling-in-prometheus
func AddTagsToLabels(m *promutils.Labels, tags []string, prefix, tagSeparator string) {
	// We surround the separated list with the separator as well. This way regular expressions
	// in relabeling rules don't have to consider tag positions.
	m.Add(prefix+"tags", tagSeparator+strings.Join(tags, tagSeparator)+tagSeparator)

	for _, tag := range tags {
		k := tag
		v := ""
		if n := strings.IndexByte(tag, '='); n >= 0 {
			k = tag[:n]
			v = tag[n+1:]
		}
		m.Add(SanitizeLabelName(prefix+"tag_"+k), v)
		m.Add(SanitizeLabelName(prefix+"tagpresent_"+k), "true")
	}
}
