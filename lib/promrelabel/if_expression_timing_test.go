package promrelabel

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkIfExpression(b *testing.B) {
	const maxLabels = 100
	labels := make([]prompb.Label, maxLabels)
	for i := range maxLabels {
		label := prompb.Label{
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

	labels[maxLabels-1] = prompb.Label{
		Name:  "__name__",
		Value: "foo",
	}
	b.Run("equal __name__: last", func(b *testing.B) {
		ifExpr := `foo`
		benchIfExpr(b, ifExpr, labels)
	})

	labels[maxLabels/2] = prompb.Label{
		Name:  "__name__",
		Value: "foo",
	}
	b.Run("equal __name__: middle", func(b *testing.B) {
		ifExpr := `foo`
		benchIfExpr(b, ifExpr, labels)
	})

	labels[0] = prompb.Label{
		Name:  "__name__",
		Value: "foo",
	}
	b.Run("equal __name__: first", func(b *testing.B) {
		ifExpr := `foo`
		benchIfExpr(b, ifExpr, labels)
	})
}

func benchIfExpr(b *testing.B, expr string, labels []prompb.Label) {
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

func BenchmarkIfExpressionAlternatives(b *testing.B) {
	type benchmarkCase struct {
		name    string
		ifExprs []string
		labels  []prompb.Label
		result  bool
	}
	var testCases []benchmarkCase
	for _, labelsCount := range []int{16, 48} {
		testCases = append(testCases, benchmarkCase{
			name:    fmt.Sprintf("single_exact_match/labels_%d", labelsCount),
			ifExprs: []string{"metric_0"},
			labels:  newIfExpressionBenchmarkLabels("metric_0", labelsCount),
			result:  true,
		})
	}
	for _, alternativesCount := range []int{6, 32} {
		exact := newIfExpressionBenchmarkExpressions(alternativesCount, "metric_%d")
		mixedExactCount := alternativesCount * 7 / 8
		mixed := append([]string{}, newIfExpressionBenchmarkExpressions(mixedExactCount, "metric_%d")...)
		mixed = append(mixed, newIfExpressionBenchmarkExpressions(alternativesCount-mixedExactCount, `{missing_%d="yes"}`)...)
		generic := newIfExpressionBenchmarkExpressions(alternativesCount, `{missing_%d="yes"}`)
		for _, labelsCount := range []int{16, 48} {
			testCases = append(testCases,
				benchmarkCase{
					name:    fmt.Sprintf("distinct_exact_%d_miss/labels_%d", alternativesCount, labelsCount),
					ifExprs: exact,
					labels:  newIfExpressionBenchmarkLabels("other", labelsCount),
				},
				benchmarkCase{
					name:    fmt.Sprintf("generic_%d_miss/labels_%d", alternativesCount, labelsCount),
					ifExprs: generic,
					labels:  newIfExpressionBenchmarkLabels("other", labelsCount),
				},
				benchmarkCase{
					name:    fmt.Sprintf("mixed_%d_miss/labels_%d", alternativesCount, labelsCount),
					ifExprs: mixed,
					labels:  newIfExpressionBenchmarkLabels("other", labelsCount),
				},
			)
		}
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			ie := mustNewIfExpressionForBenchmark(b, tc.ifExprs)
			for b.Loop() {
				if result := ie.Match(tc.labels); result != tc.result {
					b.Fatalf("unexpected match result; got %v; want %v", result, tc.result)
				}
			}
		})
	}
}

func mustNewIfExpressionForBenchmark(b *testing.B, ifExprs []string) *IfExpression {
	b.Helper()
	v := make([]any, len(ifExprs))
	for i, ifExpr := range ifExprs {
		v[i] = ifExpr
	}
	var ie IfExpression
	if err := ie.unmarshalFromInterface(v); err != nil {
		b.Fatalf("cannot unmarshal if expressions: %s", err)
	}
	return &ie
}

func newIfExpressionBenchmarkExpressions(n int, format string) []string {
	ifExprs := make([]string, n)
	for i := range ifExprs {
		ifExprs[i] = fmt.Sprintf(format, i)
	}
	return ifExprs
}

func newIfExpressionBenchmarkLabels(metricName string, labelsCount int) []prompb.Label {
	labels := make([]prompb.Label, 0, labelsCount)
	labels = append(labels, prompb.Label{Name: "__name__", Value: metricName})
	for i := range labelsCount - 1 {
		labels = append(labels, prompb.Label{
			Name:  fmt.Sprintf("label_%d", i),
			Value: fmt.Sprintf("value_%d", i),
		})
	}
	return labels
}
