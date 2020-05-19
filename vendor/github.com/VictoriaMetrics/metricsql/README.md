[![GoDoc](https://godoc.org/github.com/VictoriaMetrics/metricsql?status.svg)](http://godoc.org/github.com/VictoriaMetrics/metricsql)
[![Go Report](https://goreportcard.com/badge/github.com/VictoriaMetrics/metricsql)](https://goreportcard.com/report/github.com/VictoriaMetrics/metricsql)


# metricsql

Package metricsql implements [MetricsQL](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/MetricsQL)
and [PromQL](https://medium.com/@valyala/promql-tutorial-for-beginners-9ab455142085) parser in Go.

### Usage

```go
    expr, err := metricsql.Parse(`sum(rate(foo{bar="baz"}[5m])) by (job)`)
    if err != nil {
        // parse error
    }
    // Now expr contains parsed MetricsQL as `*Expr` structs.
    // See Parse examples for more details.
```

See [docs](https://godoc.org/github.com/VictoriaMetrics/metricsql) for more details.
