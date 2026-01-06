package kubernetes

import (
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// Gateway represents Gateway k8s object.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#gateway
type Gateway struct {
	ObjectMeta `json:"metadata"`
	Spec       GatewaySpec
}

// GatewaySpec represents Gateway object spec.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#gatewayspec
type GatewaySpec struct {
	Listeners []GatewayListener
}

// GatewayListener represents Gateway lisneters.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#listener
type GatewayListener struct {
	Name     string
	Hostname string
	Port     int
	Protocol string
}

// GRPCRoute represents GRPCRoute k8s object.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#grpcroute
type GRPCRoute struct {
	ObjectMeta `json:"metadata"`
	Spec       GRPCRouteSpec
}

// GRPCRouteSpec represents GRPCRoute object spec.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#grpcroutespec
type GRPCRouteSpec struct {
	ParentRefs []ParentReference
	Hostnames  []string
	Rules      []GRPCRouteRule
}

// GRPCRouteRule represents rule for GRPCRoute.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#grpcrouterule
type GRPCRouteRule struct {
	Name    string
	Matches []GRPCRouteMatch
}

// GRPCRouteMatch repesets rule matches for GRPCRoute.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#grpcroutematch
type GRPCRouteMatch struct {
	Method string
}

// HTTPRoute represents HTTPRoute k8s object.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#httproute
type HTTPRoute struct {
	ObjectMeta `json:"metadata"`
	Spec       HTTPRouteSpec
}

// HTTPRouteSpec represents HTTPRoute object spec.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#httproutespec
type HTTPRouteSpec struct {
	ParentRefs []ParentReference
	Hostnames  []string
	Rules      []HTTPRouteRule
}

// ParentReference represents parent references for route object.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#parentreference
type ParentReference struct {
	Group       string
	Kind        string
	Namespace   string
	Name        string
	SectionName string
}

// HTTPRouteRule represents rule for HTTPRoute.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#httprouterule
type HTTPRouteRule struct {
	Name    string
	Matches []HTTPRouteMatch
}

// HTTPRouteMatch repesets rule matches for HTTPRoute.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#httproutematch
type HTTPRouteMatch struct {
	Path   HTTPPathMatch
	Method string
}

// HTTPPathMatch repesents path match in a rule for HTTPRoute.
//
// See https://gateway-api.sigs.k8s.io/reference/spec/#httppathmatch
type HTTPPathMatch struct {
	Value string
}

func getListeners(refs []ParentReference, gw *groupWatcher) []GatewayListener {
	var listeners []GatewayListener
	for i := range refs {
		ref := &refs[i]
		switch ref.Kind {
		case "Gateway":
			o := gw.getObjectByRoleLocked("gateway", ref.Namespace, ref.Name)
			if o == nil {
				continue
			}
			g := o.(*Gateway)
			for _, l := range g.Spec.Listeners {
				if len(ref.SectionName) == 0 || ref.SectionName == l.Name {
					listeners = append(listeners, l)
				}
			}
		case "Service":
			o := gw.getObjectByRoleLocked("service", ref.Namespace, ref.Name)
			if o == nil {
				continue
			}
			s := o.(*Service)
			for _, p := range s.Spec.Ports {
				if len(ref.SectionName) == 0 || ref.SectionName == p.Name {
					listeners = append(listeners, GatewayListener{
						Name:     p.Name,
						Port:     p.Port,
						Protocol: p.Protocol,
					})
				}
			}
		default:
			continue
		}
	}
	return listeners
}

func getHostnames(pattern string, hostnames []string) []string {
	if len(pattern) == 0 {
		return hostnames
	}
	result := hostnames[:0]
	for _, h := range hostnames {
		if matchesHostPattern(pattern, h) {
			result = append(result, h)
		}
	}
	return result
}

// getTargetLabels returns labels for GRPCRoute.
func (gr *GRPCRoute) getTargetLabels(gw *groupWatcher) []*promutil.Labels {
	listeners := getListeners(gr.Spec.ParentRefs, gw)
	if len(listeners) == 0 {
		return nil
	}
	var ls []*promutil.Labels
	for _, listener := range listeners {
		hostnames := getHostnames(listener.Hostname, gr.Spec.Hostnames)
		if len(hostnames) == 0 {
			continue
		}
		for i := range gr.Spec.Rules {
			r := &gr.Spec.Rules[i]
			if len(r.Matches) == 0 {
				r.Matches = append(r.Matches, GRPCRouteMatch{
					Method: "GET",
				})
			}
			for j := range r.Matches {
				m := &r.Matches[j]
				if len(m.Method) == 0 {
					m.Method = "GET"
				}
				for _, h := range hostnames {
					l := promutil.GetLabels()
					l.Add("__address__", h)
					l.Add("__meta_kubernetes_namespace", gr.Namespace)
					l.Add("__meta_kubernetes_grpcroute_name", gr.Name)
					l.Add("__meta_kubernetes_grpcroute_scheme", strings.ToLower(listener.Protocol))
					l.Add("__meta_kubernetes_grpcroute_host", h)
					if gw.attachNamespaceMetadata {
						o := gw.getObjectByRoleLocked("namespace", "", gr.Namespace)
						if o != nil {
							ns := o.(*Namespace)
							ns.registerLabelsAndAnnotations("__meta_kubernetes_namespace", l)
						}
					}
					gr.registerLabelsAndAnnotations("__meta_kubernetes_grpcroute", l)
					ls = append(ls, l)
				}
			}
		}
	}
	return ls
}

// getTargetLabels returns labels for HTTPRoute.
func (hr *HTTPRoute) getTargetLabels(gw *groupWatcher) []*promutil.Labels {
	listeners := getListeners(hr.Spec.ParentRefs, gw)
	if len(listeners) == 0 {
		return nil
	}
	var ls []*promutil.Labels
	for _, listener := range listeners {
		hostnames := getHostnames(listener.Hostname, hr.Spec.Hostnames)
		if len(hostnames) == 0 {
			continue
		}
		for i := range hr.Spec.Rules {
			r := &hr.Spec.Rules[i]
			if len(r.Matches) == 0 {
				r.Matches = append(r.Matches, HTTPRouteMatch{
					Path: HTTPPathMatch{
						Value: "/",
					},
					Method: "GET",
				})
			}
			for j := range r.Matches {
				m := &r.Matches[j]
				if len(m.Path.Value) == 0 {
					m.Path.Value = "/"
				}
				if len(m.Method) == 0 {
					m.Method = "GET"
				}
				for _, h := range hostnames {
					l := promutil.GetLabels()
					l.Add("__address__", h)
					l.Add("__meta_kubernetes_namespace", hr.Namespace)
					l.Add("__meta_kubernetes_httproute_name", hr.Name)
					l.Add("__meta_kubernetes_httproute_scheme", strings.ToLower(listener.Protocol))
					l.Add("__meta_kubernetes_httproute_host", h)
					l.Add("__meta_kubernetes_httproute_path", m.Path.Value)
					if gw.attachNamespaceMetadata {
						o := gw.getObjectByRoleLocked("namespace", "", hr.Namespace)
						if o != nil {
							ns := o.(*Namespace)
							ns.registerLabelsAndAnnotations("__meta_kubernetes_namespace", l)
						}
					}
					hr.registerLabelsAndAnnotations("__meta_kubernetes_httproute", l)
					ls = append(ls, l)
				}
			}
		}
	}
	return ls
}

// getTargetLabels returns labels for Gateway.
// Gateways themselves are not scraped, so this returns nil.
// The Gateway metadata is used to enrich labels for other Gateway API resources.
func (*Gateway) getTargetLabels(_ *groupWatcher) []*promutil.Labels {
	return nil
}
