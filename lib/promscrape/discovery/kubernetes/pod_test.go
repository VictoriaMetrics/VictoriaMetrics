package kubernetes

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func TestParsePodListFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		nls, err := parsePodList([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if nls != nil {
			t.Fatalf("unexpected non-nil PodList: %v", nls)
		}
	}
	f(``)
	f(`[1,23]`)
	f(`{"items":[{"metadata":1}]}`)
	f(`{"items":[{"metadata":{"labels":[1]}}]}`)
}

func TestParsePodListSuccess(t *testing.T) {
	data := `
{
  "kind": "PodList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/pods",
    "resourceVersion": "72425"
  },
  "items": [
    {
      "metadata": {
        "name": "etcd-m01",
        "namespace": "kube-system",
        "selfLink": "/api/v1/namespaces/kube-system/pods/etcd-m01",
        "uid": "9d328156-75d1-411a-bdd0-aeacb53a38de",
        "resourceVersion": "22318",
        "creationTimestamp": "2020-03-16T20:44:30Z",
        "labels": {
          "component": "etcd",
          "tier": "control-plane"
        },
        "annotations": {
          "kubernetes.io/config.hash": "3ec997b76fb6ed3b78da8e0b5676dac4",
          "kubernetes.io/config.mirror": "3ec997b76fb6ed3b78da8e0b5676dac4",
          "kubernetes.io/config.seen": "2020-03-16T20:44:26.538136233Z",
          "kubernetes.io/config.source": "file"
        },
        "ownerReferences": [
          {
            "apiVersion": "v1",
            "kind": "Node",
            "name": "m01",
            "uid": "b48dd901-ead0-476a-b209-d2d908d65109",
            "controller": true
          }
        ]
      },
      "spec": {
        "volumes": [
          {
            "name": "etcd-certs",
            "hostPath": {
              "path": "/var/lib/minikube/certs/etcd",
              "type": "DirectoryOrCreate"
            }
          },
          {
            "name": "etcd-data",
            "hostPath": {
              "path": "/var/lib/minikube/etcd",
              "type": "DirectoryOrCreate"
            }
          }
        ],
        "containers": [
          {
            "name": "etcd",
            "image": "k8s.gcr.io/etcd:3.4.3-0",
            "command": [
              "etcd",
              "--advertise-client-urls=https://172.17.0.2:2379",
              "--cert-file=/var/lib/minikube/certs/etcd/server.crt",
              "--client-cert-auth=true",
              "--data-dir=/var/lib/minikube/etcd",
              "--initial-advertise-peer-urls=https://172.17.0.2:2380",
              "--initial-cluster=m01=https://172.17.0.2:2380",
              "--key-file=/var/lib/minikube/certs/etcd/server.key",
              "--listen-client-urls=https://127.0.0.1:2379,https://172.17.0.2:2379",
              "--listen-metrics-urls=http://127.0.0.1:2381",
              "--listen-peer-urls=https://172.17.0.2:2380",
              "--name=m01",
              "--peer-cert-file=/var/lib/minikube/certs/etcd/peer.crt",
              "--peer-client-cert-auth=true",
              "--peer-key-file=/var/lib/minikube/certs/etcd/peer.key",
              "--peer-trusted-ca-file=/var/lib/minikube/certs/etcd/ca.crt",
              "--snapshot-count=10000",
              "--trusted-ca-file=/var/lib/minikube/certs/etcd/ca.crt"
            ],
            "resources": {
              
            },
            "ports": [
              {
                "name": "foobar",
                "containerPort": 1234,
                "protocol": "TCP"
              }
            ],
	    "volumeMounts": [
              {
                "name": "etcd-data",
                "mountPath": "/var/lib/minikube/etcd"
              },
              {
                "name": "etcd-certs",
                "mountPath": "/var/lib/minikube/certs/etcd"
              }
            ],
            "livenessProbe": {
              "httpGet": {
                "path": "/health",
                "port": 2381,
                "host": "127.0.0.1",
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 15,
              "timeoutSeconds": 15,
              "periodSeconds": 10,
              "successThreshold": 1,
              "failureThreshold": 8
            },
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "imagePullPolicy": "IfNotPresent"
          }
        ],
        "restartPolicy": "Always",
        "terminationGracePeriodSeconds": 30,
        "dnsPolicy": "ClusterFirst",
        "nodeName": "m01",
        "hostNetwork": true,
        "securityContext": {
          
        },
        "schedulerName": "default-scheduler",
        "tolerations": [
          {
            "operator": "Exists",
            "effect": "NoExecute"
          }
        ],
        "priorityClassName": "system-cluster-critical",
        "priority": 2000000000,
        "enableServiceLinks": true
      },
      "status": {
        "phase": "Running",
        "conditions": [
          {
            "type": "Initialized",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2020-03-20T13:30:29Z"
          },
          {
            "type": "Ready",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2020-03-20T13:30:32Z"
          },
          {
            "type": "ContainersReady",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2020-03-20T13:30:32Z"
          },
          {
            "type": "PodScheduled",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2020-03-20T13:30:29Z"
          }
        ],
        "hostIP": "172.17.0.2",
        "podIP": "172.17.0.2",
        "podIPs": [
          {
            "ip": "172.17.0.2"
          }
        ],
        "startTime": "2020-03-20T13:30:29Z",
        "containerStatuses": [
          {
            "name": "etcd",
            "state": {
              "running": {
                "startedAt": "2020-03-20T13:30:30Z"
              }
            },
            "lastState": {
              "terminated": {
                "exitCode": 0,
                "reason": "Completed",
                "startedAt": "2020-03-17T18:56:24Z",
                "finishedAt": "2020-03-20T13:29:54Z",
                "containerID": "docker://24eea6f192d4598fcc129b5f163a02d1457137f4ec34e8c80c6049a65604cb07"
              }
            },
            "ready": true,
            "restartCount": 2,
            "image": "k8s.gcr.io/etcd:3.4.3-0",
            "imageID": "docker-pullable://k8s.gcr.io/etcd@sha256:4afb99b4690b418ffc2ceb67e1a17376457e441c1f09ab55447f0aaf992fa646",
            "containerID": "docker://a28f0800855008485376c1eece1cf61de97cb7026b9188d138b0d55d92fc2f5c",
            "started": true
          }
        ],
        "qosClass": "BestEffort"
      }
    }
  ]
}
`
	pls, err := parsePodList([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(pls.Items) != 1 {
		t.Fatalf("unexpected length of PodList.Items; got %d; want %d", len(pls.Items), 1)
	}
	pod := pls.Items[0]

	// Check pod.appendTargetLabels()
	labelss := pod.appendTargetLabels(nil)
	var sortedLabelss [][]prompbmarshal.Label
	for _, labels := range labelss {
		sortedLabelss = append(sortedLabelss, discoveryutils.GetSortedLabels(labels))
	}
	expectedLabels := [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "172.17.0.2:1234",

			"__meta_kubernetes_namespace":                   "kube-system",
			"__meta_kubernetes_pod_name":                    "etcd-m01",
			"__meta_kubernetes_pod_ip":                      "172.17.0.2",
			"__meta_kubernetes_pod_container_name":          "etcd",
			"__meta_kubernetes_pod_container_port_name":     "foobar",
			"__meta_kubernetes_pod_container_port_number":   "1234",
			"__meta_kubernetes_pod_container_port_protocol": "TCP",
			"__meta_kubernetes_pod_ready":                   "true",
			"__meta_kubernetes_pod_phase":                   "Running",
			"__meta_kubernetes_pod_node_name":               "m01",
			"__meta_kubernetes_pod_host_ip":                 "172.17.0.2",
			"__meta_kubernetes_pod_uid":                     "9d328156-75d1-411a-bdd0-aeacb53a38de",
			"__meta_kubernetes_pod_controller_kind":         "Node",
			"__meta_kubernetes_pod_controller_name":         "m01",
			"__meta_kubernetes_pod_container_init":          "false",

			"__meta_kubernetes_pod_label_component": "etcd",
			"__meta_kubernetes_pod_label_tier":      "control-plane",

			"__meta_kubernetes_pod_labelpresent_component": "true",
			"__meta_kubernetes_pod_labelpresent_tier":      "true",

			"__meta_kubernetes_pod_annotation_kubernetes_io_config_hash":   "3ec997b76fb6ed3b78da8e0b5676dac4",
			"__meta_kubernetes_pod_annotation_kubernetes_io_config_mirror": "3ec997b76fb6ed3b78da8e0b5676dac4",
			"__meta_kubernetes_pod_annotation_kubernetes_io_config_seen":   "2020-03-16T20:44:26.538136233Z",
			"__meta_kubernetes_pod_annotation_kubernetes_io_config_source": "file",

			"__meta_kubernetes_pod_annotationpresent_kubernetes_io_config_hash":   "true",
			"__meta_kubernetes_pod_annotationpresent_kubernetes_io_config_mirror": "true",
			"__meta_kubernetes_pod_annotationpresent_kubernetes_io_config_seen":   "true",
			"__meta_kubernetes_pod_annotationpresent_kubernetes_io_config_source": "true",
		}),
	}
	if !reflect.DeepEqual(sortedLabelss, expectedLabels) {
		t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, expectedLabels)
	}
}
