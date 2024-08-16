package promrelabel

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkIfExpression(b *testing.B) {
	const maxLabels = 100
	labels := make([]prompbmarshal.Label, maxLabels)
	for i := 0; i < maxLabels; i++ {
		label := prompbmarshal.Label{
			Name:  fmt.Sprintf("foo%d", i),
			Value: fmt.Sprintf("bar%d", i),
		}
		labels[i] = label
	}

	b.Run("equal label: last", func(b *testing.B) {
		n := maxLabels - 1
		ifExpr := fmt.Sprintf(`'{foo%d="bar%d"}'`, n, n)
		benchIfExpr(b, ifExpr, labels)
	})
	b.Run("equal label: middle", func(b *testing.B) {
		n := maxLabels / 2
		ifExpr := fmt.Sprintf(`'{foo%d="bar%d"}'`, n, n)
		benchIfExpr(b, ifExpr, labels)
	})
	b.Run("equal label: first", func(b *testing.B) {
		ifExpr := fmt.Sprintf(`'{foo%d="bar%d"}'`, 0, 0)
		benchIfExpr(b, ifExpr, labels)
	})

	labels[maxLabels-1] = prompbmarshal.Label{
		Name:  "__name__",
		Value: "foo",
	}
	b.Run("equal __name__: last", func(b *testing.B) {
		ifExpr := `foo`
		benchIfExpr(b, ifExpr, labels)
	})

	labels[maxLabels/2] = prompbmarshal.Label{
		Name:  "__name__",
		Value: "foo",
	}
	b.Run("equal __name__: middle", func(b *testing.B) {
		ifExpr := `foo`
		benchIfExpr(b, ifExpr, labels)
	})

	labels[0] = prompbmarshal.Label{
		Name:  "__name__",
		Value: "foo",
	}
	b.Run("equal __name__: first", func(b *testing.B) {
		ifExpr := `foo`
		benchIfExpr(b, ifExpr, labels)
	})
}

func benchIfExpr(b *testing.B, expr string, labels []prompbmarshal.Label) {
	b.Helper()
	var ie IfExpression
	if err := yaml.UnmarshalStrict([]byte(expr), &ie); err != nil {
		b.Fatalf("unexpected error during unmarshal: %s", err)
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !ie.Match(labels) {
				panic(fmt.Sprintf("expected to have a match for %q", expr))
			}
		}
	})
}
