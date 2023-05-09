package consul

import (
	"encoding/json"
	"fmt"
)

// Agent is Consul agent.
//
// See https://www.consul.io/api/agent.html#read-configuration
type Agent struct {
	Config AgentConfig
	Member AgentMember
	Meta   map[string]string
}

// AgentConfig is Consul agent config.
//
// See https://www.consul.io/api/agent.html#read-configuration
type AgentConfig struct {
	Datacenter string
	NodeName   string
}

// AgentMember is Consul agent member info.
//
// See https://www.consul.io/api/agent.html#read-configuration
type AgentMember struct {
	Addr string
}

// ParseAgent parses Consul agent information from data.
func ParseAgent(data []byte) (*Agent, error) {
	var a Agent
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("cannot unmarshal agent info from %q: %w", data, err)
	}
	return &a, nil
}
