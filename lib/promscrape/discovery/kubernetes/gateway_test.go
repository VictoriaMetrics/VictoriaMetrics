package kubernetes

import (
	"bytes"
	"slices"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestGetHostnames(t *testing.T) {
	f := func(pattern string, hosts, expected []string) {
		t.Helper()
		result := getHostnames(pattern, hosts)
		if !slices.Equal(result, expected) {
			t.Fatalf("unexpected result for matchesHostPattern(%q, %q); got %v; want %v", pattern, hosts, result, expected)
		}
	}
	f("", []string{""}, []string{""})
	f("", []string{"foo"}, []string{"foo"})
	f("foo", []string{""}, nil)
	f("localhost", []string{"localhost"}, []string{"localhost"})
	f("localhost", []string{"localhost2"}, nil)
	f("*.foo", []string{"bar"}, nil)
	f("foo.bar", []string{"foo.bar"}, []string{"foo.bar"})
	f("foo.baz", []string{"foo.bar"}, nil)
	f("a.x.yyy", []string{"b.x.yyy"}, nil)
	f("*.x.yyy", []string{"b.x.yyy", "a.x.yyy", "z.y.yyy"}, []string{"b.x.yyy", "a.x.yyy"})
}

func TestParseHTTPRouteListFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		r := bytes.NewBufferString(s)
		objectsByKey, _, err := parseObjectList[HTTPRoute](r)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if len(objectsByKey) != 0 {
			t.Fatalf("unexpected non-empty HTTPRouteList: %v", objectsByKey)
		}
	}
	f(``)
	f(`[1,23]`)
	f(`{"items":[{"metadata":1}]}`)
	f(`{"items":[{"metadata":{"labels":[1]}}]}`)
}

func TestParseHTTPRouteListSuccess(t *testing.T) {
	data := `
{
  "kind": "HTTPRouteList",
  "apiVersion": "extensions/v1",
  "metadata": {
    "selfLink": "/apis/extensions/v1/httproutes",
    "resourceVersion": "351452"
  },
  "items": [
    {
      "metadata": {
        "name": "test-httproute",
        "namespace": "default",
        "selfLink": "/apis/extensions/v1/namespaces/default/httproutes/test-httproute",
        "uid": "6d3f38f9-de89-4bc9-b273-c8faf74e8a27",
        "resourceVersion": "351445",
        "generation": 1,
        "creationTimestamp": "2020-04-13T16:43:52Z",
        "annotations": {
          "test": "value"
        }
      },
      "spec": {
        "hostnames": ["foobar"],
        "parentRefs": [{
          "group": "gateway.networking.k8s.io",
          "kind": "Gateway",
          "name": "global-http",
          "namespace": "default"
        }],
        "rules": [{
          "backendRefs": {
            "kind": "Service",
            "name": "testsvc",
            "port": 8080,
            "weight": 1
          },
          "matches": [{
            "path": {
              "type": "PathPrefix",
              "value": "/"
            }
          }]
        }]
      },
      "status": {}
    }
  ]
}`
	r := bytes.NewBufferString(data)
	objectsByKey, meta, err := parseObjectList[HTTPRoute](r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedResourceVersion := "351452"
	if meta.ResourceVersion != expectedResourceVersion {
		t.Fatalf("unexpected resource version; got %s; want %s", meta.ResourceVersion, expectedResourceVersion)
	}
	sortedLabelss := getSortedLabelss(objectsByKey)
	expectedLabelss := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__": "foobar",
			"__meta_kubernetes_httproute_annotation_test":        "value",
			"__meta_kubernetes_httproute_annotationpresent_test": "true",
			"__meta_kubernetes_httproute_host":                   "foobar",
			"__meta_kubernetes_httproute_name":                   "test-httproute",
			"__meta_kubernetes_httproute_path":                   "/",
			"__meta_kubernetes_httproute_scheme":                 "https",
			"__meta_kubernetes_namespace":                        "default",
		}),
	}
	if !areEqualLabelss(sortedLabelss, expectedLabelss) {
		t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, expectedLabelss)
	}
}
