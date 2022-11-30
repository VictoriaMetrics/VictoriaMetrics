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
	s := bytesutil.ToUnsafeString(b)
	s = bytesutil.InternString(s)
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
