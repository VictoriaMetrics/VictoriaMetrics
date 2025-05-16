package kubernetes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/easyproto"
)

func (ig *Ingress) key() string {
	return ig.Metadata.key()
}

func parseIngressList(data []byte, contentType string) (map[string]object, ListMeta, error) {
	igl := &IngressList{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, igl); err != nil {
			return nil, igl.Metadata, fmt.Errorf("cannot unmarshal IngressList: %w", err)
		}
	case contentTypeProtobuf:
		if err := igl.unmarshalProtobuf(data); err != nil {
			return nil, igl.Metadata, fmt.Errorf("cannot unmarshal IngressList: %w", err)
		}
	}
	objectsByKey := make(map[string]object)
	for _, ig := range igl.Items {
		objectsByKey[ig.key()] = &ig
	}
	return objectsByKey, igl.Metadata, nil
}

func parseIngress(data []byte, contentType string) (object, error) {
	ig := &Ingress{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, ig); err != nil {
			return nil, err
		}
	case contentTypeProtobuf:
		if err := ig.unmarshalProtobuf(data); err != nil {
			return nil, err
		}
	}
	return ig, nil
}

// IngressList represents ingress list in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#ingresslist-v1-networking-k8s-io
type IngressList struct {
	Metadata ListMeta
	Items    []Ingress
}

// unmarshalProtobuf unmarshals IngressList according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *IngressList) unmarshalProtobuf(src []byte) (err error) {
	// message IngressList {
	//   optional ListMeta metadata = 1;
	//   repeated Ingress items = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in IngressList: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ListMeta")
			}
			m := &r.Metadata
			if err := m.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ListMeta: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Items")
			}
			r.Items = slicesutil.SetLength(r.Items, len(r.Items)+1)
			i := &r.Items[len(r.Items)-1]
			if err := i.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Items: %w", err)
			}
		}
	}
	return nil
}

func (r *IngressList) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	r.Metadata.marshalProtobuf(mm.AppendMessage(1))
	for _, item := range r.Items {
		item.marshalProtobuf(mm.AppendMessage(2))
	}
}

// Ingress represents ingress in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#ingress-v1-networking-k8s-io
type Ingress struct {
	Metadata ObjectMeta
	Spec     IngressSpec
}

func (ig *Ingress) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	ig.Metadata.marshalProtobuf(mm.AppendMessage(1))
	ig.Spec.marshalProtobuf(mm.AppendMessage(2))
}

// unmarshalProtobuf unmarshals Ingress according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (ig *Ingress) unmarshalProtobuf(src []byte) (err error) {
	// message Ingress {
	//   optional ObjectMeta metadata = 1;
	//   repeated IngressSpec spec = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Ingress: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ObjectMeta")
			}
			m := &ig.Metadata
			if err := m.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ObjectMeta: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Spec")
			}
			s := &ig.Spec
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Spec: %w", err)
			}
		}
	}
	return nil
}

// IngressSpec represents ingress spec in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#ingressspec-v1-networking-k8s-io
type IngressSpec struct {
	TLS              []IngressTLS `json:"tls"`
	Rules            []IngressRule
	IngressClassName string
}

// unmarshalProtobuf unmarshals IngressSpec according to spec
//
// See https://github.com/kubernetes/api/blob/master/extensions/v1beta1/generated.proto
func (r *IngressSpec) unmarshalProtobuf(src []byte) (err error) {
	// message IngressSpec {
	//   repeated IngressTLS tls = 2;
	//   repeated IngressRule rules = 3;
	//   optional string ingressClassName = 4;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in IngressSpec: %w", err)
		}
		switch fc.FieldNum {
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read IngressTLS")
			}
			r.TLS = slicesutil.SetLength(r.TLS, len(r.TLS)+1)
			t := &r.TLS[len(r.TLS)-1]
			if err := t.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal IngressTLS: %w", err)
			}
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read IngressRule")
			}
			r.Rules = slicesutil.SetLength(r.Rules, len(r.Rules)+1)
			rule := &r.Rules[len(r.Rules)-1]
			if err := rule.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal IngressRule: %w", err)
			}
		case 4:
			ingressClassName, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read IngressClassName")
			}
			r.IngressClassName = strings.Clone(ingressClassName)
		}
	}
	return nil
}

func (r *IngressSpec) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, tls := range r.TLS {
		tls.marshalProtobuf(mm.AppendMessage(2))
	}
	for _, rule := range r.Rules {
		rule.marshalProtobuf(mm.AppendMessage(3))
	}
	mm.AppendString(4, r.IngressClassName)
}

// IngressTLS represents ingress TLS spec in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#ingresstls-v1-networking-k8s-io
type IngressTLS struct {
	Hosts []string
}

// unmarshalProtobuf unmarshals IngressTLS according to spec
//
// See https://github.com/kubernetes/api/blob/master/extensions/v1beta1/generated.proto
func (r *IngressTLS) unmarshalProtobuf(src []byte) (err error) {
	// message IngressTLS {
	//   repeated string hosts = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in IngressTLS: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			host, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Host")
			}
			r.Hosts = append(r.Hosts, host)

		}
	}
	return nil
}

func (r *IngressTLS) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, host := range r.Hosts {
		mm.AppendString(1, host)
	}
}

// IngressRule represents ingress rule in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#ingressrule-v1-networking-k8s-io
type IngressRule struct {
	Host string
	HTTP HTTPIngressRuleValue `json:"http"`
}

// unmarshalProtobuf unmarshals IngressRule according to spec
//
// See https://github.com/kubernetes/api/blob/master/extensions/v1beta1/generated.proto
func (r *IngressRule) unmarshalProtobuf(src []byte) (err error) {
	// message IngressRule {
	//   optional string host = 1;
	//   optional IngressRuleValue ingressRuleValue = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in IngressRule: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			host, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Host")
			}
			r.Host = strings.Clone(host)
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read IngressRuleValue")
			}
			v := &r.HTTP
			if err := v.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal IngressRuleValue: %w", err)
			}
		}
	}
	return nil
}

func (r *IngressRule) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Host)
	r.HTTP.marshalProtobuf(mm.AppendMessage(2))
}

// HTTPIngressRuleValue represents HTTP ingress rule value in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#httpingressrulevalue-v1-networking-k8s-io
type HTTPIngressRuleValue struct {
	Paths []HTTPIngressPath
}

// unmarshalProtobuf unmarshals HTTPIngressRuleValue according to spec
//
// See https://github.com/kubernetes/api/blob/master/extensions/v1beta1/generated.proto
func (r *HTTPIngressRuleValue) unmarshalProtobuf(src []byte) (err error) {
	// message HTTPIngressRuleValue {
	//   repeated HTTPIngressPath paths = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ObjectMeta: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read HTTPIngressPath")
			}
			r.Paths = slicesutil.SetLength(r.Paths, len(r.Paths)+1)
			p := r.Paths[len(r.Paths)-1]
			if err := p.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal HTTPIngressPath: %w", err)
			}
		}
	}
	return nil
}

func (r *HTTPIngressRuleValue) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, value := range r.Paths {
		value.marshalProtobuf(mm.AppendMessage(1))
	}
}

// HTTPIngressPath represents HTTP ingress path in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#httpingresspath-v1-networking-k8s-io
type HTTPIngressPath struct {
	Path string
}

// unmarshalProtobuf unmarshals HTTPIngressPath according to spec
//
// See https://github.com/kubernetes/api/blob/master/extensions/v1beta1/generated.proto
func (r *HTTPIngressPath) unmarshalProtobuf(src []byte) (err error) {
	// message HTTPIngressPath {
	//   optional string path = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ObjectMeta: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			path, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Path")
			}
			r.Path = strings.Clone(path)
		}
	}
	return nil
}

func (r *HTTPIngressPath) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Path)
}

// getTargetLabels returns labels for ig.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ingress
func (ig *Ingress) getTargetLabels(_ *groupWatcher) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, r := range ig.Spec.Rules {
		paths := getIngressRulePaths(r.HTTP.Paths)
		scheme := getSchemeForHost(r.Host, ig.Spec.TLS)
		for _, path := range paths {
			m := getLabelsForIngressPath(ig, scheme, r.Host, path)
			ms = append(ms, m)
		}
	}
	return ms
}

func getSchemeForHost(host string, tlss []IngressTLS) string {
	for _, tls := range tlss {
		for _, hostPattern := range tls.Hosts {
			if matchesHostPattern(hostPattern, host) {
				return "https"
			}
		}
	}
	return "http"
}

func matchesHostPattern(pattern, host string) bool {
	if pattern == host {
		return true
	}
	if !strings.HasPrefix(pattern, "*.") {
		return false
	}
	pattern = pattern[len("*."):]
	n := strings.IndexByte(host, '.')
	if n < 0 {
		return false
	}
	host = host[n+1:]
	return pattern == host
}

func getLabelsForIngressPath(ig *Ingress, scheme, host, path string) *promutil.Labels {
	m := promutil.GetLabels()
	m.Add("__address__", host)
	m.Add("__meta_kubernetes_namespace", ig.Metadata.Namespace)
	m.Add("__meta_kubernetes_ingress_name", ig.Metadata.Name)
	m.Add("__meta_kubernetes_ingress_scheme", scheme)
	m.Add("__meta_kubernetes_ingress_host", host)
	m.Add("__meta_kubernetes_ingress_path", path)
	m.Add("__meta_kubernetes_ingress_class_name", ig.Spec.IngressClassName)
	ig.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_ingress", m)
	return m
}

func getIngressRulePaths(paths []HTTPIngressPath) []string {
	if len(paths) == 0 {
		return []string{"/"}
	}
	var result []string
	for _, p := range paths {
		path := p.Path
		if path == "" {
			path = "/"
		}
		result = append(result, path)
	}
	return result
}
