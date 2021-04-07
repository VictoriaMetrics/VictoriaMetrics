package kubernetes

import (
	"reflect"
	"testing"
)

func TestGetAPIPathsWithNamespaces(t *testing.T) {
	f := func(role string, namespaces []string, selectors []Selector, expectedPaths, expectedNamespaces []string) {
		t.Helper()
		paths, resultNamespaces := getAPIPathsWithNamespaces(role, namespaces, selectors)
		if !reflect.DeepEqual(paths, expectedPaths) {
			t.Fatalf("unexpected paths; got\n%q\nwant\n%q", paths, expectedPaths)
		}
		if !reflect.DeepEqual(resultNamespaces, expectedNamespaces) {
			t.Fatalf("unexpected namespaces; got\n%q\nwant\n%q", resultNamespaces, expectedNamespaces)
		}
	}

	// role=node
	f("node", nil, nil, []string{"/api/v1/nodes"}, []string{""})
	f("node", []string{"foo", "bar"}, nil, []string{"/api/v1/nodes"}, []string{""})
	f("node", nil, []Selector{
		{
			Role:  "pod",
			Label: "foo",
			Field: "bar",
		},
	}, []string{"/api/v1/nodes"}, []string{""})
	f("node", nil, []Selector{
		{
			Role:  "node",
			Label: "foo",
			Field: "bar",
		},
	}, []string{"/api/v1/nodes?labelSelector=foo&fieldSelector=bar"}, []string{""})
	f("node", []string{"x", "y"}, []Selector{
		{
			Role:  "node",
			Label: "foo",
			Field: "bar",
		},
	}, []string{"/api/v1/nodes?labelSelector=foo&fieldSelector=bar"}, []string{""})

	// role=pod
	f("pod", nil, nil, []string{"/api/v1/pods"}, []string{""})
	f("pod", []string{"foo", "bar"}, nil, []string{
		"/api/v1/namespaces/foo/pods",
		"/api/v1/namespaces/bar/pods",
	}, []string{"foo", "bar"})
	f("pod", nil, []Selector{
		{
			Role:  "node",
			Label: "foo",
		},
	}, []string{"/api/v1/pods"}, []string{""})
	f("pod", nil, []Selector{
		{
			Role:  "pod",
			Label: "foo",
		},
		{
			Role:  "pod",
			Label: "x",
			Field: "y",
		},
	}, []string{"/api/v1/pods?labelSelector=foo%2Cx&fieldSelector=y"}, []string{""})
	f("pod", []string{"x", "y"}, []Selector{
		{
			Role:  "pod",
			Label: "foo",
		},
		{
			Role:  "pod",
			Label: "x",
			Field: "y",
		},
	}, []string{
		"/api/v1/namespaces/x/pods?labelSelector=foo%2Cx&fieldSelector=y",
		"/api/v1/namespaces/y/pods?labelSelector=foo%2Cx&fieldSelector=y",
	}, []string{"x", "y"})

	// role=service
	f("service", nil, nil, []string{"/api/v1/services"}, []string{""})
	f("service", []string{"x", "y"}, nil, []string{
		"/api/v1/namespaces/x/services",
		"/api/v1/namespaces/y/services",
	}, []string{"x", "y"})
	f("service", nil, []Selector{
		{
			Role:  "node",
			Label: "foo",
		},
		{
			Role:  "service",
			Field: "bar",
		},
	}, []string{"/api/v1/services?fieldSelector=bar"}, []string{""})
	f("service", []string{"x", "y"}, []Selector{
		{
			Role:  "service",
			Label: "abc=de",
		},
	}, []string{
		"/api/v1/namespaces/x/services?labelSelector=abc%3Dde",
		"/api/v1/namespaces/y/services?labelSelector=abc%3Dde",
	}, []string{"x", "y"})

	// role=endpoints
	f("endpoints", nil, nil, []string{"/api/v1/endpoints"}, []string{""})
	f("endpoints", []string{"x", "y"}, nil, []string{
		"/api/v1/namespaces/x/endpoints",
		"/api/v1/namespaces/y/endpoints",
	}, []string{"x", "y"})
	f("endpoints", []string{"x", "y"}, []Selector{
		{
			Role:  "endpoints",
			Label: "bbb",
		},
		{
			Role:  "node",
			Label: "aa",
		},
	}, []string{
		"/api/v1/namespaces/x/endpoints?labelSelector=bbb",
		"/api/v1/namespaces/y/endpoints?labelSelector=bbb",
	}, []string{"x", "y"})

	// role=endpointslices
	f("endpointslices", nil, nil, []string{"/apis/discovery.k8s.io/v1beta1/endpointslices"}, []string{""})
	f("endpointslices", []string{"x", "y"}, []Selector{
		{
			Role:  "endpointslices",
			Field: "field",
			Label: "label",
		},
	}, []string{
		"/apis/discovery.k8s.io/v1beta1/namespaces/x/endpointslices?labelSelector=label&fieldSelector=field",
		"/apis/discovery.k8s.io/v1beta1/namespaces/y/endpointslices?labelSelector=label&fieldSelector=field",
	}, []string{"x", "y"})

	// role=ingress
	f("ingress", nil, nil, []string{"/apis/networking.k8s.io/v1beta1/ingresses"}, []string{""})
	f("ingress", []string{"x", "y"}, []Selector{
		{
			Role:  "node",
			Field: "xyay",
		},
		{
			Role:  "ingress",
			Field: "abc",
		},
		{
			Role:  "ingress",
			Label: "cde",
		},
		{
			Role:  "ingress",
			Label: "baaa",
		},
	}, []string{
		"/apis/networking.k8s.io/v1beta1/namespaces/x/ingresses?labelSelector=cde%2Cbaaa&fieldSelector=abc",
		"/apis/networking.k8s.io/v1beta1/namespaces/y/ingresses?labelSelector=cde%2Cbaaa&fieldSelector=abc",
	}, []string{"x", "y"})
}

func TestParseBookmark(t *testing.T) {
	data := `{"kind": "Pod", "apiVersion": "v1", "metadata": {"resourceVersion": "12746"} }`
	bm, err := parseBookmark([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedResourceVersion := "12746"
	if bm.Metadata.ResourceVersion != expectedResourceVersion {
		t.Fatalf("unexpected resourceVersion; got %q; want %q", bm.Metadata.ResourceVersion, expectedResourceVersion)
	}
}
