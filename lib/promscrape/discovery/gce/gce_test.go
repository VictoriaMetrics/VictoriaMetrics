package gce

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestMarshallingSDConfigWithZoneYAML(t *testing.T) {
	sdConfig := SDConfig{
		Project: "test-project",
		Zone: ZoneYAML{
			Zones: []string{"zone-a", "zone-b"},
		},
	}

	data, err := yaml.Marshal(sdConfig)
	if err != nil {
		t.Fatalf("unexpected non-nil error")
	}

	strData := string(data)
	expected := "project: test-project\nzone:\n- zone-a\n- zone-b\n"
	if strData != expected {
		t.Fatalf("unexpected marshal:\ngot \n%vwant\n%v", strData, expected)
	}
}
