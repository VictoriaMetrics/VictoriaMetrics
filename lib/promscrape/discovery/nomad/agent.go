package nomad

import (
	"encoding/json"
	"fmt"
)

// Agent is Nomad agent.
//
// See https://developer.hashicorp.com/nomad/api-docs/agent
type Agent struct {
	Config AgentConfig
}

// AgentConfig is Nomad agent config.
//
// See https://developer.hashicorp.com/nomad/api-docs/agent
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
