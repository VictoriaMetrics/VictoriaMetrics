package stringsutil

import (
	"sync/atomic"
	"testing"
)

func BenchmarkAppendLowercase(b *testing.B) {
	b.Run("ascii-full-lowercase", func(b *testing.B) {
		data := `started kubernetes log collector for node "gke-sandbox-e2-standard-8-20250715071-5b0a2ce9-vyko"`
		benchmarkToLower(b, data)
	})
	b.Run("ascii-partial-lowercase", func(b *testing.B) {
		data := `started Kubernetes log collector for Node "gke-sandbox-e2-standard-8-20250715071-5b0a2ce9-vyko"`
		benchmarkToLower(b, data)
	})
	b.Run("ascii-full-uppercase", func(b *testing.B) {
		data := `STARTED KUBERNETES LOG COLLECTOR FOR NODE "GKE-SANDBOX-E2-STANDARD-8-20250715071-5B0A2CE9-VYKO"`
		benchmarkToLower(b, data)
	})
	b.Run("ascii-partial-uppercase", func(b *testing.B) {
		data := `started KUBERNETES log collector FOR NODE "gke-sandbox-e2-standard-8-20250715071-5b0a2ce9-vyko"`
		benchmarkToLower(b, data)
	})
	b.Run("ascii-full-title", func(b *testing.B) {
		data := `Started Kubernetes Log Collector For Node "Gke-Sandbox-E2-Standard-8-20250715071-5b0a2ce9-Vyko"`
		benchmarkToLower(b, data)
	})
	b.Run("ascii-partial-title", func(b *testing.B) {
		data := `started Kubernetes log Collector for Node "gke-sandbox-e2-standard-8-20250715071-5b0a2ce9-vyko"`
		benchmarkToLower(b, data)
	})
	b.Run("ascii-mixcase", func(b *testing.B) {
		data := `Started Kubernetes log COLLECTOR for nodE "GKE-Sandbox-E2-Standard-8-20250715071-5b0a2ce9-VYKO"`
		benchmarkToLower(b, data)
	})

	b.Run("unicode-full-lowercase", func(b *testing.B) {
		data := `запущен кубернетес лог коллектор на ноде гке-сендбокс-е2-стандарт-8-20250715071-5в0а2се9-вико`
		benchmarkToLower(b, data)
	})
	b.Run("unicode-partial-lowercase", func(b *testing.B) {
		data := `запущен КубернеТЕС лОг кОллектор нА НодЕ гке-сендбокс-е2-стандарт-8-20250715071-5в0а2се9-вико`
		benchmarkToLower(b, data)
	})
	b.Run("unicode-full-uppercase", func(b *testing.B) {
		data := `ЗАПУЩЕН КУБЕРНЕТЕС ЛОГ КОЛЛЕКТОР НА НОДЕ ГКЕ-СЕНДБОКС-Е2-СТАНДАРТ-8-20250715071-5В0А2СЕ9-ВИКО`
		benchmarkToLower(b, data)
	})
	b.Run("unicode-partial-uppercase", func(b *testing.B) {
		data := `запущен КУБЕРНЕТЕС лог коллектор НА НОДЕ гке-сендбокс-е2-стандарт-8-20250715071-5в0а2се9-вико`
		benchmarkToLower(b, data)
	})
	b.Run("unicode-full-title", func(b *testing.B) {
		data := `Запущен Кубернетес Лог Коллектор На Ноде Гке-Сендбокс-Е2-Стандарт-8-20250715071-5В0а2се9-Вико`
		benchmarkToLower(b, data)
	})
	b.Run("unicode-partial-title", func(b *testing.B) {
		data := `запущен Кубернетес лог Коллектор на Ноде гке-сендбокс-е2-стандарт-8-20250715071-5в0а2се9-вико`
		benchmarkToLower(b, data)
	})
	b.Run("unicode-mixcase", func(b *testing.B) {
		data := `Запущен Кубернетес лог КОЛЛЕКТОР на нодЕ гке-Сендбокс-Е2-Стандарт-8-20250715071-5В0а2се9-ВИКО`
		benchmarkToLower(b, data)
	})
}

func benchmarkToLower(b *testing.B, s string) {
	b.Helper()

	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		for pb.Next() {
			buf = AppendLowercase(buf[:0], s)
		}
		GlobalSink.Add(uint64(len(buf)))
	})
}

var GlobalSink atomic.Uint64

func BenchmarkIsLowercase(b *testing.B) {
	b.Run("ascii-mismatch", func(b *testing.B) {
		data := `started kubernetes log collector for node "gke-sandbox-e2-standard-8-20250715071-5b0a2ce9-vyko"`
		benchmarkIsLowercase(b, data, true)
	})
	b.Run("ascii-match-start", func(b *testing.B) {
		data := `started Kubernetes log collector for Node "gke-sandbox-e2-standard-8-20250715071-5b0a2ce9-vyko"`
		benchmarkIsLowercase(b, data, false)
	})
	b.Run("ascii-match-middle", func(b *testing.B) {
		data := `started kubernetes log collector for Node "gke-sandbox-e2-standard-8-20250715071-5b0a2ce9-vyko"`
		benchmarkIsLowercase(b, data, false)
	})
	b.Run("ascii-match-end", func(b *testing.B) {
		data := `started kubernetes log collector for node "gke-sandbox-e2-standard-8-20250715071-5b0a2ce9-vyKo"`
		benchmarkIsLowercase(b, data, false)
	})

	b.Run("unicode-mismatch", func(b *testing.B) {
		data := `запущен кубернетес лог коллектор на ноде гке-сендбокс-е2-стандарт-8-20250715071-5в0а2се9-вико`
		benchmarkIsLowercase(b, data, true)
	})
	b.Run("unicode-match-start", func(b *testing.B) {
		data := `запущен Кубернетес лог коллектор на ноде гке-сендбокс-е2-стандарт-8-20250715071-5в0а2се9-вико`
		benchmarkIsLowercase(b, data, false)
	})
	b.Run("unicode-match-middle", func(b *testing.B) {
		data := `запущен кубернетес лог коллектор на Ноде гке-сендбокс-е2-стандарт-8-20250715071-5в0а2се9-вико`
		benchmarkIsLowercase(b, data, false)
	})
	b.Run("unicode-match-end", func(b *testing.B) {
		data := `запущен кубернетес лог коллектор на ноде гке-сендбокс-е2-стандарт-8-20250715071-5в0а2се9-виКо`
		benchmarkIsLowercase(b, data, false)
	})
}

func benchmarkIsLowercase(b *testing.B, s string, expected bool) {
	b.Helper()

	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if IsLowercase(s) != expected {
				b.Fatalf("expected IsLower(%q) to return %v", s, expected)
			}
		}
	})
}
