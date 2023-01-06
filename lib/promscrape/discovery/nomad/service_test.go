package nomad

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestParseServicesFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		sns, err := parseServices([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if sns != nil {
			t.Fatalf("unexpected non-nil ServiceNodes: %v", sns)
		}
	}
	f(``)
	f(`[1,23]`)
	f(`{"items":[{"metadata":1}]}`)
}

func TestParseServiceNodesSuccess(t *testing.T) {
	data := `
	[
		{
			"ID": "_nomad-task-1a321d90-79b5-681f-e6fa-8a43c8ec6b69-web-doggo-web-http",
			"ServiceName": "doggo-web",
			"Namespace": "default",
			"NodeID": "9e02c85b-db59-45f1-ddee-40d0317bd33d",
			"Datacenter": "dc1",
			"JobID": "doggo",
			"AllocID": "1a321d90-79b5-681f-e6fa-8a43c8ec6b69",
			"Tags": [
				"doggo",
				"web"
			],
			"Address": "192.168.29.76",
			"Port": 23761,
			"CreateIndex": 402,
			"ModifyIndex": 402
		}
	]
`
	sns, err := parseServices([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(sns) != 1 {
		t.Fatalf("unexpected length of ServiceNodes; got %d; want %d", len(sns), 1)
	}
	sn := sns[0]

	// Check sn.appendTargetLabels()
	tagSeparator := ","
	labelss := sn.appendTargetLabels(nil, tagSeparator)
	expectedLabelss := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                   "192.168.29.76:23761",
			"__meta_nomad_dc":               "dc1",
			"__meta_nomad_node_id":          "9e02c85b-db59-45f1-ddee-40d0317bd33d",
			"__meta_nomad_address":          "192.168.29.76",
			"__meta_nomad_namespace":        "default",
			"__meta_nomad_service":          "doggo-web",
			"__meta_nomad_service_address":  "192.168.29.76",
			"__meta_nomad_service_alloc_id": "1a321d90-79b5-681f-e6fa-8a43c8ec6b69",
			"__meta_nomad_service_id":       "_nomad-task-1a321d90-79b5-681f-e6fa-8a43c8ec6b69-web-doggo-web-http",
			"__meta_nomad_service_job_id":   "doggo",
			"__meta_nomad_service_port":     "23761",
			"__meta_nomad_tag_doggo":        "",
			"__meta_nomad_tag_web":          "",
			"__meta_nomad_tagpresent_doggo": "true",
			"__meta_nomad_tagpresent_web":   "true",
			"__meta_nomad_tags":             ",doggo,web,",
		}),
	}
	discoveryutils.TestEqualLabelss(t, labelss, expectedLabelss)
}
