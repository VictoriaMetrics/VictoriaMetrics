package kubernetes

import (
	"bytes"
	"testing"
)

func TestParseNamespaceListFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		r := bytes.NewBufferString(s)
		objectsByKey, _, err := parseNamespaceList(r)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if len(objectsByKey) != 0 {
			t.Fatalf("unexpected non-empty objectsByKey: %v", objectsByKey)
		}
	}
	f(``)
	f(`[1,23]`)
	f(`{"items":[{"metadata":1}]}`)
	f(`{"items":[{"metadata":{"labels":[1]}}]}`)
}

func TestParseNamespaceListSuccess(t *testing.T) {
	data := `{
  "kind": "NamespaceList",
  "apiVersion": "v1",
  "metadata": {
    "resourceVersion": "2295"
  },
  "items": [
    {
      "metadata": {
        "name": "default",
        "uid": "b5d26698-3840-4f04-9c6e-e0b029a49109",
        "resourceVersion": "23",
        "creationTimestamp": "2025-10-17T16:27:07Z",
        "labels": {
          "kubernetes.io/metadata.name": "default"
        },
        "managedFields": [
          {
            "manager": "kube-apiserver",
            "operation": "Update",
            "apiVersion": "v1",
            "time": "2025-10-17T16:27:07Z",
            "fieldsType": "FieldsV1",
            "fieldsV1": {
              "f:metadata": {
                "f:labels": {
                  ".": {},
                  "f:kubernetes.io/metadata.name": {}
                }
              }
            }
          }
        ]
      },
      "spec": {
        "finalizers": [
          "kubernetes"
        ]
      },
      "status": {
        "phase": "Active"
      }
    },
    {
      "metadata": {
        "name": "kube-system",
        "uid": "fa5a7bd9-43e5-43cd-acad-41ad238e3b91",
        "resourceVersion": "5",
        "creationTimestamp": "2025-10-17T16:27:07Z",
        "labels": {
          "kubernetes.io/metadata.name": "kube-system"
        },
        "managedFields": [
          {
            "manager": "kube-apiserver",
            "operation": "Update",
            "apiVersion": "v1",
            "time": "2025-10-17T16:27:07Z",
            "fieldsType": "FieldsV1",
            "fieldsV1": {
              "f:metadata": {
                "f:labels": {
                  ".": {},
                  "f:kubernetes.io/metadata.name": {}
                }
              }
            }
          }
        ]
      },
      "spec": {
        "finalizers": [
          "kubernetes"
        ]
      },
      "status": {
        "phase": "Active"
      }
    }
  ]
}
`
	r := bytes.NewBufferString(data)
	objectsByKey, meta, err := parseNamespaceList(r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedResourceVersion := "2295"
	if meta.ResourceVersion != expectedResourceVersion {
		t.Fatalf("unexpected resource version; got %s; want %s", meta.ResourceVersion, expectedResourceVersion)
	}

	// Verify we have the expected namespaces
	expectedKeys := []string{"/default", "/kube-system"}
	if len(objectsByKey) != len(expectedKeys) {
		t.Fatalf("unexpected number of namespaces; got %d; want %d", len(objectsByKey), len(expectedKeys))
	}
	for _, key := range expectedKeys {
		if _, ok := objectsByKey[key]; !ok {
			t.Fatalf("expected namespace key %s not found", key)
		}
	}

	// Verify namespace objects were parsed correctly
	defaultNS, ok := objectsByKey["/default"].(*Namespace)
	if !ok {
		t.Fatalf("expected namespace object for default, got %T", objectsByKey["/default"])
	}
	if defaultNS.Metadata.Name != "default" {
		t.Fatalf("unexpected namespace name; got %s; want default", defaultNS.Metadata.Name)
	}
	if defaultNS.Status.Phase != "Active" {
		t.Fatalf("unexpected namespace phase; got %s; want Active", defaultNS.Status.Phase)
	}

	// Namespaces don't generate scrape targets, so getTargetLabels should return nil
	sortedLabelss := getSortedLabelss(objectsByKey)
	if len(sortedLabelss) != 0 {
		t.Fatalf("expected no target labels for namespaces; got %d", len(sortedLabelss))
	}
}
