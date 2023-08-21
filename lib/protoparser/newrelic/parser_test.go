package newrelic

import (
	"reflect"
	"testing"

	"github.com/valyala/fastjson"
)

func TestEvents_Unmarshal(t *testing.T) {
	tests := []struct {
		name    string
		metrics []Metric
		json    string
		wantErr bool
	}{
		{
			name:    "empty json",
			metrics: []Metric{},
			json:    "",
			wantErr: true,
		},
		{
			name: "json with correct data",
			metrics: []Metric{
				{
					Timestamp: 1690286061000,
					Tags: []Tag{
						{Key: "entityKey", Value: "macbook-pro.local"},
					},
					Metric: "system_sample_disk_writes_per_second",
					Value:  0,
				},
				{
					Timestamp: 1690286061000,
					Tags: []Tag{
						{Key: "entityKey", Value: "macbook-pro.local"},
					},
					Metric: "system_sample_uptime",
					Value:  762376,
				},
			},
			json: `[
    {
      "EntityID":28257883748326179,
      "IsAgent":true,
      "Events":[
        {
          "eventType":"SystemSample",
          "timestamp":1690286061,
          "entityKey":"macbook-pro.local",
          "diskWritesPerSecond":0,
          "uptime":762376
        }
      ],
      "ReportingAgentID":28257883748326179
    }
  ]`,
			wantErr: false,
		},
		{
			name:    "empty array in json",
			metrics: []Metric{},
			json:    `[]`,
			wantErr: false,
		},
		{
			name:    "empty events in json",
			metrics: []Metric{},
			json: `[
    {
      "EntityID":28257883748326179,
      "IsAgent":true,
      "Events":[],
      "ReportingAgentID":28257883748326179
    }
  ]`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Events{Metrics: []Metric{}}

			value, err := fastjson.Parse(tt.json)
			if (err != nil) != tt.wantErr {
				t.Errorf("cannot parse json error: %s", err)
			}

			if value != nil {
				v, err := value.Array()
				if err != nil {
					t.Errorf("cannot get array from json")
				}
				if err := e.Unmarshal(v); (err != nil) != tt.wantErr {
					t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				}
				if !reflect.DeepEqual(e.Metrics, tt.metrics) {
					t.Errorf("got metrics => %v; expected = %v", e.Metrics, tt.metrics)
				}
			}
		})
	}
}
