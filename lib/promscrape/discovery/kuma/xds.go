package kuma

import (
	"encoding/json"
	"fmt"
)

// discoveryRequest represent xDS-requests for Kuma Service Mesh
// https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/discovery/v3/discovery.proto#envoy-v3-api-msg-service-discovery-v3-discoveryrequest
type discoveryRequest struct {
	VersionInfo   string               `json:"version_info,omitempty"`
	Node          discoveryRequestNode `json:"node,omitempty"`
	ResourceNames []string             `json:"resource_names,omitempty"`
	TypeUrl       string               `json:"type_url,omitempty"`
	ResponseNonce string               `json:"response_nonce,omitempty"`
}

type discoveryRequestNode struct {
	Id string `json:"id,omitempty"`
}

// discoveryResponse represent xDS-requests for Kuma Service Mesh
// https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/discovery/v3/discovery.proto#envoy-v3-api-msg-service-discovery-v3-discoveryresponse
type discoveryResponse struct {
	VersionInfo string `json:"version_info,omitempty"`
	Resources   []struct {
		Mesh    string `json:"mesh,omitempty"`
		Service string `json:"service,omitempty"`
		Targets []struct {
			Name        string            `json:"name,omitempty"`
			Scheme      string            `json:"scheme,omitempty"`
			Address     string            `json:"address,omitempty"`
			MetricsPath string            `json:"metrics_path,omitempty"`
			Labels      map[string]string `json:"labels,omitempty"`
		} `json:"targets,omitempty"`
		Labels map[string]string `json:"labels,omitempty"`
	} `json:"resources,omitempty"`
	TypeUrl      string `json:"type_url,omitempty"`
	Nonce        string `json:"nonce,omitempty"`
	ControlPlane struct {
		Identifier string `json:"identifier,omitempty"`
	} `json:"control_plane,omitempty"`
}

func parseDiscoveryResponse(data []byte) (discoveryResponse, error) {
	response := discoveryResponse{}
	err := json.Unmarshal(data, &response)
	if err != nil {
		return discoveryResponse{}, fmt.Errorf("cannot parse kuma_sd api response, err:  %w", err)
	}
	if response.TypeUrl != xdsResourceTypeUrl {
		return discoveryResponse{}, fmt.Errorf("unexpected type_url in kuma_sd api response, expected: %s, got: %s", xdsResourceTypeUrl, response.TypeUrl)
	}

	return response, nil
}
