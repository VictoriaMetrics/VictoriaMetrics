package kubernetes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/easyproto"
)

func (s *Service) key() string {
	return s.Metadata.key()
}

func parseServiceList(data []byte, contentType string) (map[string]object, ListMeta, error) {
	sl := &ServiceList{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, sl); err != nil {
			return nil, sl.Metadata, fmt.Errorf("cannot unmarshal ServiceList: %w", err)
		}
	case contentTypeProtobuf:
		if err := sl.unmarshalProtobuf(data); err != nil {
			return nil, sl.Metadata, fmt.Errorf("cannot unmarshal ServiceList: %w", err)
		}
	}
	objectsByKey := make(map[string]object)
	for _, s := range sl.Items {
		objectsByKey[s.key()] = &s
	}
	return objectsByKey, sl.Metadata, nil
}

func parseService(data []byte, contentType string) (object, error) {
	s := &Service{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, s); err != nil {
			return nil, err
		}
	case contentTypeProtobuf:
		if err := s.unmarshalProtobuf(data); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// ServiceList is k8s service list.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#servicelist-v1-core
type ServiceList struct {
	Metadata ListMeta
	Items    []Service
}

// unmarshalProtobuf unmarshals ServiceList according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *ServiceList) unmarshalProtobuf(src []byte) (err error) {
	// message ServiceList {
	//   optional ListMeta metadata = 1;
	//   repeated Service items = 2;
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
			s := &r.Items[len(r.Items)-1]
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Items: %w", err)
			}
		}
	}
	return nil
}

func (r *ServiceList) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	r.Metadata.marshalProtobuf(mm.AppendMessage(1))
	for _, item := range r.Items {
		item.marshalProtobuf(mm.AppendMessage(2))
	}
}

// Service is k8s service.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#service-v1-core
type Service struct {
	Metadata ObjectMeta
	Spec     ServiceSpec
}

// unmarshalProtobuf unmarshals Service according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (s *Service) unmarshalProtobuf(src []byte) (err error) {
	// message Service {
	//   optional ObjectMeta metadata = 1;
	//   repeated ServiceSpec spec = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Service: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ObjectMeta")
			}
			m := &s.Metadata
			if err := m.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ObjectMeta: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Spec")
			}
			spec := &s.Spec
			if err := spec.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Spec: %w", err)
			}
		}
	}
	return nil
}

func (s *Service) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	s.Metadata.marshalProtobuf(mm.AppendMessage(1))
	s.Spec.marshalProtobuf(mm.AppendMessage(2))
}

// ServiceSpec is k8s service spec.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#servicespec-v1-core
type ServiceSpec struct {
	ClusterIP    string
	ExternalName string
	Type         string
	Ports        []ServicePort
}

// unmarshalProtobuf unmarshals ServiceSpec according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *ServiceSpec) unmarshalProtobuf(src []byte) (err error) {
	// message ServiceSpec {
	//   repeated ServicePort ports = 1;
	//   optional string clusterIP = 3;
	//   optional string type = 4;
	//   optional string externalName = 10;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Service: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Port")
			}
			r.Ports = slicesutil.SetLength(r.Ports, len(r.Ports)+1)
			p := &r.Ports[len(r.Ports)-1]
			if err := p.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Port: %w", err)
			}
		case 3:
			clusterIP, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read ClusterIP")
			}
			r.ClusterIP = strings.Clone(clusterIP)
		case 4:
			serviceType, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Type")
			}
			r.Type = strings.Clone(serviceType)
		case 10:
			externalName, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read ExternalName")
			}
			r.ExternalName = strings.Clone(externalName)
		}
	}
	return nil
}

func (r *ServiceSpec) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, p := range r.Ports {
		p.marshalProtobuf(mm.AppendMessage(1))
	}
	mm.AppendString(3, r.ClusterIP)
	mm.AppendString(4, r.Type)
	mm.AppendString(10, r.ExternalName)
}

// ServicePort is k8s service port.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#serviceport-v1-core
type ServicePort struct {
	Name     string
	Protocol string
	Port     int
}

// unmarshalProtobuf unmarshals ServicePort according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *ServicePort) unmarshalProtobuf(src []byte) (err error) {
	// message ServicePort {
	//   optional string name = 1;
	//   optional string protocol = 2;
	//   optional int32 port = 3;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Service: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Name")
			}
			r.Name = strings.Clone(name)
		case 2:
			protocol, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Protocol")
			}
			r.Protocol = strings.Clone(protocol)
		case 3:
			port, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read Port")
			}
			r.Port = int(port)
		}
	}
	return nil
}

func (r *ServicePort) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Name)
	mm.AppendString(2, r.Protocol)
	mm.AppendInt32(3, int32(r.Port))
}

// getTargetLabels returns labels for each port of the given s.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#service
func (s *Service) getTargetLabels(_ *groupWatcher) []*promutil.Labels {
	host := fmt.Sprintf("%s.%s.svc", s.Metadata.Name, s.Metadata.Namespace)
	var ms []*promutil.Labels
	for _, sp := range s.Spec.Ports {
		addr := discoveryutil.JoinHostPort(host, sp.Port)
		m := promutil.GetLabels()
		m.Add("__address__", addr)
		m.Add("__meta_kubernetes_service_port_name", sp.Name)
		m.Add("__meta_kubernetes_service_port_number", strconv.Itoa(sp.Port))
		m.Add("__meta_kubernetes_service_port_protocol", sp.Protocol)
		s.appendCommonLabels(m)
		ms = append(ms, m)
	}
	return ms
}

func (s *Service) appendCommonLabels(m *promutil.Labels) {
	m.Add("__meta_kubernetes_namespace", s.Metadata.Namespace)
	m.Add("__meta_kubernetes_service_name", s.Metadata.Name)
	m.Add("__meta_kubernetes_service_type", s.Spec.Type)
	if s.Spec.Type != "ExternalName" {
		m.Add("__meta_kubernetes_service_cluster_ip", s.Spec.ClusterIP)
	} else {
		m.Add("__meta_kubernetes_service_external_name", s.Spec.ExternalName)
	}
	s.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_service", m)
}
