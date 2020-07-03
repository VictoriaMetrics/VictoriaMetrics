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
}

// AgentConfig is Consul agent config.
//
// See https://www.consul.io/api/agent.html#read-configuration
type AgentConfig struct {
	Datacenter string
}

func parseAgent(data []byte) (*Agent, error) {
	var a Agent
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("cannot unmarshal agent info from %q: %w", data, err)
	}
	return &a, nil
}
