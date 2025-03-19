package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
)

func TestRecordingToApi(t *testing.T) {
	fq := &datasource.FakeQuerier{}
	fq.Add(datasource.Metric{
		Values: []float64{1}, Timestamps: []int64{0},
	})
	entriesLimit := 44
	g := rule.NewGroup(config.Group{
		Name:        "group",
		File:        "rules.yaml",
		Concurrency: 1,
		Rules: []config.Rule{
			{
				ID:                 1248,
				Record:             "record_name",
				Expr:               "up",
				Labels:             map[string]string{"label": "value"},
				UpdateEntriesLimit: &entriesLimit,
			},
		},
	}, fq, 1*time.Minute, nil)
	rr := g.Rules[0].(*rule.RecordingRule)

	expectedRes := apiRule{
		Name:           "record_name",
		Query:          "up",
		Labels:         map[string]string{"label": "value"},
		Health:         "ok",
		Type:           ruleTypeRecording,
		DatasourceType: "prometheus",
		ID:             "1248",
		GroupID:        fmt.Sprintf("%d", g.CreateID()),
		GroupName:      "group",
		File:           "rules.yaml",
		MaxUpdates:     44,
		Updates:        make([]rule.StateEntry, 0),
	}

	res := recordingToAPI(rr)

	if !reflect.DeepEqual(res, expectedRes) {
		t.Fatalf("expected to have: \n%v;\ngot: \n%v", expectedRes, res)
	}
}

func TestUrlValuesToStrings(t *testing.T) {
	mapQueryParams := map[string][]string{
		"param1": {"param1"},
		"param2": {"anotherparam"},
	}
	expectedRes := []string{"param1=param1", "param2=anotherparam"}
	res := urlValuesToStrings(mapQueryParams)

	if len(res) != len(expectedRes) {
		t.Fatalf("Expected length %d, but got %d", len(expectedRes), len(res))
	}
	for ind, val := range expectedRes {
		if val != res[ind] {
			t.Fatalf("Expected %v; but got %v", val, res[ind])
		}
	}
}
