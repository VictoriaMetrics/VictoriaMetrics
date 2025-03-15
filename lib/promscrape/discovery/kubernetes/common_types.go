package kubernetes

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/easyproto"
)

// ObjectMeta represents ObjectMeta from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta
type ObjectMeta struct {
	Name            string
	Namespace       string
	UID             string
	Labels          *promutil.Labels
	Annotations     *promutil.Labels
	OwnerReferences []OwnerReference
}

// unmarshalProtobuf unmarshals ObjectMeta according to spec
//
// See https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/generated.proto
func (om *ObjectMeta) unmarshalProtobuf(src []byte) (err error) {
	// message ObjectMeta {
	//   optional string name = 1;
	//   optional string namespace = 3;
	//   optional string uid = 5;
	//   map<string, string> labels = 11;
	//   map<string, string> annotations = 12;
	//   repeated OwnerReference ownerReferences = 13;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ObjectMeta: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Name")
			}
			om.Name = strings.Clone(name)
		case 3:
			ns, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Namespace")
			}
			om.Namespace = strings.Clone(ns)
		case 5:
			uid, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read UID")
			}
			om.UID = strings.Clone(uid)
		case 11:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Labels data")
			}
			if om.Labels == nil {
				om.Labels = &promutil.Labels{}
			}
			om.Labels.Labels = slicesutil.SetLength(om.Labels.Labels, len(om.Labels.Labels)+1)
			l := &om.Labels.Labels[len(om.Labels.Labels)-1]
			if err := unmarshalLabel(l, data); err != nil {
				return fmt.Errorf("cannot unmarshal Labels: %w", err)
			}
		case 12:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Annotations data")
			}
			if om.Annotations == nil {
				om.Annotations = &promutil.Labels{}
			}
			om.Annotations.Labels = slicesutil.SetLength(om.Annotations.Labels, len(om.Annotations.Labels)+1)
			l := &om.Annotations.Labels[len(om.Annotations.Labels)-1]
			if err := unmarshalLabel(l, data); err != nil {
				return fmt.Errorf("cannot unmarshal Annotations: %w", err)
			}
		case 13:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read OwnerReference")
			}
			om.OwnerReferences = slicesutil.SetLength(om.OwnerReferences, len(om.OwnerReferences)+1)
			or := &om.OwnerReferences[len(om.OwnerReferences)-1]
			if err := or.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("failed to unmarshal OwnerReference")
			}
		}
	}
	return nil
}

func (om *ObjectMeta) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, om.Name)
	mm.AppendString(3, om.Namespace)
	mm.AppendString(5, om.UID)
	if om.Labels != nil {
		for _, l := range om.Labels.Labels {
			marshalLabel(&l, mm.AppendMessage(11))
		}
	}
	if om.Annotations != nil {
		for _, l := range om.Annotations.Labels {
			marshalLabel(&l, mm.AppendMessage(12))
		}
	}
	for _, or := range om.OwnerReferences {
		or.marshalProtobuf(mm.AppendMessage(13))
	}
}

func (om *ObjectMeta) key() string {
	return om.Namespace + "/" + om.Name
}

func marshalLabel(l *prompbmarshal.Label, mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, l.Name)
	mm.AppendString(2, l.Value)
}

func unmarshalLabel(l *prompbmarshal.Label, src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Label: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Name")
			}
			l.Name = strings.Clone(name)
		case 2:
			value, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Value")
			}
			l.Value = strings.Clone(value)
		}
	}
	return nil
}

// ListMeta is a Kubernetes list metadata
//
// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta
type ListMeta struct {
	ResourceVersion string
}

// unmarshalProtobuf unmarshals ListMeta according to spec
//
// See https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/generated.proto
func (r *ListMeta) unmarshalProtobuf(src []byte) (err error) {
	// message ListMeta {
	//   optional string resourceVersion = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ListMeta: %w", err)
		}
		switch fc.FieldNum {
		case 2:
			resourceVersion, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read ResourceVersion")
			}
			r.ResourceVersion = strings.Clone(resourceVersion)
		}
	}
	return nil
}

func (r *ListMeta) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(2, r.ResourceVersion)
}

func (om *ObjectMeta) registerLabelsAndAnnotations(prefix string, m *promutil.Labels) {
	bb := bbPool.Get()
	b := bb.B
	for _, lb := range om.Labels.GetLabels() {
		b = appendThreeStrings(b[:0], prefix, "_label_", lb.Name)
		labelName := bytesutil.ToUnsafeString(b)
		m.Add(discoveryutil.SanitizeLabelName(labelName), lb.Value)

		b = appendThreeStrings(b[:0], prefix, "_labelpresent_", lb.Name)
		labelName = bytesutil.ToUnsafeString(b)
		m.Add(discoveryutil.SanitizeLabelName(labelName), "true")
	}
	for _, a := range om.Annotations.GetLabels() {
		b = appendThreeStrings(b[:0], prefix, "_annotation_", a.Name)
		labelName := bytesutil.ToUnsafeString(b)
		m.Add(discoveryutil.SanitizeLabelName(labelName), a.Value)

		b = appendThreeStrings(b[:0], prefix, "_annotationpresent_", a.Name)
		labelName = bytesutil.ToUnsafeString(b)
		m.Add(discoveryutil.SanitizeLabelName(labelName), "true")
	}
	bb.B = b
	bbPool.Put(bb)
}

var bbPool bytesutil.ByteBufferPool

func appendThreeStrings(dst []byte, a, b, c string) []byte {
	dst = append(dst, a...)
	dst = append(dst, b...)
	dst = append(dst, c...)
	return dst
}

// OwnerReference represents OwnerReferense from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#ownerreference-v1-meta
type OwnerReference struct {
	Name       string
	Controller bool
	Kind       string
}

// unmarshalProtobuf unmarshals OwnerReference according to spec
//
// See https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/generated.proto
func (r *OwnerReference) unmarshalProtobuf(src []byte) (err error) {
	// message OwnerReference {
	//   optional string kind = 1;
	//   optional string name = 3;
	//   optional bool controller = 6;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ObjectMeta: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			kind, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Kind")
			}
			r.Kind = strings.Clone(kind)
		case 3:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Name")
			}
			r.Name = strings.Clone(name)
		case 5:
			controller, ok := fc.Bool()
			if !ok {
				return fmt.Errorf("cannot read Controller")
			}
			r.Controller = controller
		}
	}
	return nil
}

func (r *OwnerReference) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Kind)
	mm.AppendString(2, r.Name)
	mm.AppendBool(3, r.Controller)
}

// DaemonEndpoint represents DaemonEndpoint from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#daemonendpoint-v1-core
type DaemonEndpoint struct {
	Port int
}

// unmarshalProtobuf unmarshals DaemonEndpoint according to spec
//
// See https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/generated.proto
func (r *DaemonEndpoint) unmarshalProtobuf(src []byte) (err error) {
	// message DaemonEndpoint {
	//   optional int32 port = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in DaemonEndpoint: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			port, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read Port")
			}
			r.Port = int(port)
		}
	}
	return nil
}

func (r *DaemonEndpoint) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendInt32(1, int32(r.Port))
}
