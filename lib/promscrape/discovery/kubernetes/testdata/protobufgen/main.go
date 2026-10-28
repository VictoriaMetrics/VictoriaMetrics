// protobufgen generates protobuf and JSON test data for Kubernetes pods with the official Kubernetes API types and serializers.
//
// Usage:
//
//	cd lib/promscrape/discovery/kubernetes/testdata/protobufgen && go run . -outputDir=..
//
// The generated files are used by tests in lib/promscrape/discovery/kubernetes for verifying
// that protobuf responses are parsed into the same objects as the equivalent JSON responses.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/apimachinery/pkg/types"
)

var outputDir = flag.String("outputDir", "..", "Directory for the generated files")

func main() {
	flag.Parse()

	pods := []*corev1.Pod{newFullPod(), newPendingPod(), newIPv6Pod()}
	list := &corev1.PodList{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "PodList"},
		ListMeta: metav1.ListMeta{ResourceVersion: "1001"},
	}
	for _, pod := range pods {
		list.Items = append(list.Items, *pod)
	}
	writeFile("pods_list.pb", encodeProtobuf(list))
	writeFile("pods_list.json", encodeJSON(list))

	emptyList := &corev1.PodList{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "PodList"},
		ListMeta: metav1.ListMeta{ResourceVersion: "2001"},
	}
	writeFile("pods_list_empty.pb", encodeProtobuf(emptyList))
	writeFile("pods_list_empty.json", encodeJSON(emptyList))

	modifiedPod := newFullPod()
	modifiedPod.ResourceVersion = "1003"
	modifiedPod.Labels["version"] = "v2"
	modifiedPod.Status.Phase = corev1.PodSucceeded
	modifiedPod.Status.ContainerStatuses[0].State = corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed"},
	}
	addedPod := newPendingPod()
	addedPod.Name = "pending-1"
	addedPod.ResourceVersion = "1002"
	deletedPod := newIPv6Pod()
	deletedPod.ResourceVersion = "1004"
	bookmark := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "1005",
			Annotations:     map[string]string{"k8s.io/initial-events-end": "true"},
		},
	}
	status := &metav1.Status{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"},
		Status:   metav1.StatusFailure,
		Message:  "too old resource version: 1 (1001)",
		Reason:   metav1.StatusReasonExpired,
		Code:     410,
	}
	events := []watchEvent{
		{"ADDED", addedPod},
		{"MODIFIED", modifiedPod},
		{"DELETED", deletedPod},
		{"BOOKMARK", bookmark},
		{"ERROR", status},
	}
	writeFile("pods_watch.pb", encodeProtobufWatchEvents(events))
	writeFile("pods_watch.json", encodeJSONWatchEvents(events))
}

type watchEvent struct {
	eventType string
	object    runtime.Object
}

func setPodTypeMeta(o runtime.Object) {
	if pod, ok := o.(*corev1.Pod); ok {
		pod.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}
	}
}

// encodeProtobuf encodes o in the same way as Kubernetes API server does for application/vnd.kubernetes.protobuf responses.
func encodeProtobuf(o runtime.Object) []byte {
	setPodTypeMeta(o)
	var bb bytes.Buffer
	if err := protobuf.NewSerializer(nil, nil).Encode(o, &bb); err != nil {
		log.Fatalf("cannot encode %T to protobuf: %s", o, err)
	}
	return bb.Bytes()
}

func encodeJSON(o runtime.Object) []byte {
	setPodTypeMeta(o)
	data, err := json.Marshal(o)
	if err != nil {
		log.Fatalf("cannot encode %T to JSON: %s", o, err)
	}
	return data
}

// encodeProtobufWatchEvents encodes events in the same way as Kubernetes API server does for protobuf watch responses.
//
// Every event is encoded as 4-byte big-endian length followed by raw WatchEvent message,
// while the object inside the event is encoded with protobuf magic prefix.
func encodeProtobufWatchEvents(events []watchEvent) []byte {
	var bb bytes.Buffer
	for _, e := range events {
		we := &metav1.WatchEvent{
			Type:   e.eventType,
			Object: runtime.RawExtension{Raw: encodeProtobuf(e.object)},
		}
		data, err := we.Marshal()
		if err != nil {
			log.Fatalf("cannot marshal WatchEvent: %s", err)
		}
		var hdr [4]byte
		binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
		bb.Write(hdr[:])
		bb.Write(data)
	}
	return bb.Bytes()
}

func encodeJSONWatchEvents(events []watchEvent) []byte {
	var bb bytes.Buffer
	for _, e := range events {
		we := &metav1.WatchEvent{
			Type:   e.eventType,
			Object: runtime.RawExtension{Raw: encodeJSON(e.object)},
		}
		data, err := json.Marshal(we)
		if err != nil {
			log.Fatalf("cannot marshal WatchEvent: %s", err)
		}
		bb.Write(data)
		bb.WriteString("\n")
	}
	return bb.Bytes()
}

func writeFile(name string, data []byte) {
	path := filepath.Join(*outputDir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatalf("cannot write %q: %s", path, err)
	}
	log.Printf("written %d bytes to %q", len(data), path)
}

func newFullPod() *corev1.Pod {
	creationTime := metav1.NewTime(time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC))
	restartAlways := corev1.ContainerRestartPolicyAlways
	gracePeriod := int64(30)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "vmagent-0",
			GenerateName:      "vmagent-",
			Namespace:         "monitoring",
			UID:               types.UID("7d6c3a4e-1b2f-4e7a-9a51-2c0d5e8f1a01"),
			ResourceVersion:   "1000",
			Generation:        2,
			CreationTimestamp: creationTime,
			Labels: map[string]string{
				"app.kubernetes.io/name":   "vmagent",
				"app":                      "vmagent",
				"controller-revision-hash": "vmagent-6d9f8b7c5",
				"empty":                    "",
			},
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   "8429",
				"unicode":              "значение-値",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       "vmagent",
					UID:        types.UID("0a1b2c3d-0000-4000-8000-000000000001"),
					Controller: ptr(true),
				},
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       "not-a-controller",
					UID:        types.UID("0a1b2c3d-0000-4000-8000-000000000002"),
				},
			},
			Finalizers: []string{"example.com/finalizer"},
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    "kube-controller-manager",
					Operation:  metav1.ManagedFieldsOperationUpdate,
					APIVersion: "v1",
					Time:       &creationTime,
					FieldsType: "FieldsV1",
					FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:labels":{".":{}}}}`)},
				},
			},
		},
		Spec: corev1.PodSpec{
			NodeName:                      "node-1",
			ServiceAccountName:            "vmagent",
			TerminationGracePeriodSeconds: &gracePeriod,
			InitContainers: []corev1.Container{
				{
					Name:          "config-init",
					Image:         "busybox:1.36",
					Command:       []string{"sh", "-c", "true"},
					RestartPolicy: &restartAlways,
					Ports: []corev1.ContainerPort{
						{ContainerPort: 9000, Protocol: corev1.ProtocolTCP},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "vmagent",
					Image: "victoriametrics/vmagent:v1.154.1",
					Args:  []string{"-promscrape.config=/etc/vmagent/config.yaml"},
					Env: []corev1.EnvVar{
						{Name: "GOMAXPROCS", Value: "4"},
					},
					Ports: []corev1.ContainerPort{
						{Name: "http", ContainerPort: 8429, Protocol: corev1.ProtocolTCP},
						{Name: "statsd", ContainerPort: 8125, HostPort: 8125, Protocol: corev1.ProtocolUDP},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("8Gi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "config", MountPath: "/etc/vmagent"},
					},
				},
				{
					Name:  "config-reloader",
					Image: "victoriametrics/operator:config-reloader-v0.50.0",
				},
			},
			Volumes: []corev1.Volume{
				{Name: "config", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
			Tolerations: []corev1.Toleration{
				{Key: "node.kubernetes.io/not-ready", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodInitialized, Status: corev1.ConditionTrue, LastTransitionTime: creationTime},
				{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: creationTime},
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue, LastTransitionTime: creationTime},
				{Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: creationTime},
			},
			HostIP:    "192.0.2.1",
			HostIPs:   []corev1.HostIP{{IP: "192.0.2.1"}},
			PodIP:     "198.51.100.5",
			PodIPs:    []corev1.PodIP{{IP: "198.51.100.5"}},
			StartTime: &creationTime,
			QOSClass:  corev1.PodQOSBurstable,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:        "config-init",
					ContainerID: "containerd://1111111111111111",
					Image:       "busybox:1.36",
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{StartedAt: creationTime},
					},
					Ready:   true,
					Started: ptr(true),
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "vmagent",
					ContainerID:  "containerd://2222222222222222",
					Image:        "victoriametrics/vmagent:v1.154.1",
					ImageID:      "docker.io/victoriametrics/vmagent@sha256:0000",
					RestartCount: 3,
					Ready:        true,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{StartedAt: creationTime},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 137, Reason: "OOMKilled"},
					},
				},
				{
					Name:        "config-reloader",
					ContainerID: "containerd://3333333333333333",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 2, Reason: "Error"},
					},
				},
			},
		},
	}
}

func newPendingPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "pending",
			Namespace:       "default",
			UID:             types.UID("7d6c3a4e-1b2f-4e7a-9a51-2c0d5e8f1a02"),
			ResourceVersion: "999",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable"},
			},
		},
	}
}

func newIPv6Pod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "ipv6",
			Namespace:       "default",
			UID:             types.UID("7d6c3a4e-1b2f-4e7a-9a51-2c0d5e8f1a03"),
			ResourceVersion: "998",
			Labels:          map[string]string{"app": "ipv6"},
		},
		Spec: corev1.PodSpec{
			NodeName: "node-v6",
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx",
					Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 80, Protocol: corev1.ProtocolTCP}},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase:  corev1.PodRunning,
			HostIP: "2001:db8::100",
			PodIP:  "2001:db8::1",
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:        "app",
					ContainerID: "containerd://4444444444444444",
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}
}

func ptr[T any](v T) *T {
	return &v
}
