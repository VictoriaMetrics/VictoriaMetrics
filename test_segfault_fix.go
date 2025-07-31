package main

import (
	"context"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
)

func main() {
	fmt.Println("Testing segfault fix...")

	// Create a mock querier that returns an error (like in the original segfault)
	querier := &datasource.FakeQuerier{
		QueryFn: func(ctx context.Context, query string, ts time.Time) ([]datasource.Metric, error) {
			// Simulate the parse error from the original stack trace
			return nil, fmt.Errorf("labelFilters: unexpected token \"\"; want \",\", \"or\", \"}\"; unparsed data: \"\"")
		},
	}

	// Create a group similar to how vmalert-tool does it
	group := &rule.Group{
		Name:     "test-group",
		Interval: time.Minute,
		Type:     config.NewPrometheusType(),
	}

	// Create an AlertingRule without calling registerMetrics() - this simulates the unittest scenario
	cfg := config.Rule{
		Alert: "BlobbyUserNearBytesQuota",
		Expr:  "blobby:cluster_disk_bytes_usage{cluster='sha_dev_1',user='a_hail'",
	}

	qb := &datasource.FakeQuerierBuilder{Querier: querier}
	ar := rule.NewAlertingRule(qb, group, cfg)

	fmt.Printf("Created AlertingRule: %s\n", ar.Name)
	fmt.Println("Note: metrics field is nil (not registered)")

	// This should NOT segfault anymore with our fix
	fmt.Println("Executing rule (this used to cause segfault)...")
	_, err := ar.Exec(context.Background(), time.Now(), 0)

	if err != nil {
		fmt.Printf("✅ Rule execution failed as expected: %v\n", err)
	} else {
		fmt.Println("✅ Rule execution completed without error")
	}

	fmt.Println("✅ No segfault occurred - fix is working!")

	// Test RecordingRule too
	recordCfg := config.Rule{
		Record: "blobby:cluster_disk_bytes_usage",
		Expr:   "sum(blobby_shepherd_disk_bytes_usage) by (cluster, user)",
	}

	rr := rule.NewRecordingRule(qb, group, recordCfg)
	fmt.Printf("Created RecordingRule: %s\n", rr.Name)

	_, err = rr.Exec(context.Background(), time.Now(), 0)
	if err != nil {
		fmt.Printf("✅ RecordingRule execution failed as expected: %v\n", err)
	} else {
		fmt.Println("✅ RecordingRule execution completed without error")
	}

	fmt.Println("✅ Both AlertingRule and RecordingRule work without segfault!")
}
