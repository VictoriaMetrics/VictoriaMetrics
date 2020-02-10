package metricsql_test

import (
	"fmt"
	"log"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/metricsql"
)

func ExampleExpandWithExprs() {
	// mql can contain arbitrary MetricsQL extensions - see https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/MetricsQL
	mql := `WITH (
		commonFilters = {job="$job", instance="$instance"},
		f(a, b) = 100*(a/b),
	)
	f(disk_free_bytes{commonFilters}, disk_total_bytes{commonFilters})`

	// Convert mql to PromQL
	pql, err := metricsql.ExpandWithExprs(mql)
	if err != nil {
		log.Fatalf("cannot expand with expressions: %s", err)
	}

	fmt.Printf("%s\n", pql)

	// Output:
	// 100 * (disk_free_bytes{job="$job", instance="$instance"} / disk_total_bytes{job="$job", instance="$instance"})
}
