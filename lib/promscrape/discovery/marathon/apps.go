package marathon

import (
	"encoding/json"
	"fmt"
	"math/rand"
)

// AppList is a list of Marathon apps.
type AppList struct {
	Apps []app `json:"apps"`
}

// App describes a service running on Marathon.
type app struct {
	ID              string            `json:"id"`
	Tasks           []task            `json:"tasks"`
	TasksRunning    int               `json:"tasksRunning"`
	Labels          map[string]string `json:"labels"`
	Container       container         `json:"container"`
	PortDefinitions []portDefinition  `json:"portDefinitions"`
	Networks        []network         `json:"networks"`
	RequirePorts    bool              `json:"requirePorts"`
}

// task describes one instance of a service running on Marathon.
type task struct {
	ID          string   `json:"id"`
	Host        string   `json:"host"`
	Ports       []uint32 `json:"ports"`
	IPAddresses []ipAddr `json:"ipAddresses"`
}

// ipAddress describes the address and protocol the container's network interface is bound to.
type ipAddr struct {
	IPAddress string `json:"ipAddress"`
	Protocol  string `json:"protocol"`
}

// Container describes the runtime an app in running in.
type container struct {
	Docker       dockerContainer `json:"docker"`
	PortMappings []portMapping   `json:"portMappings"`
}

// DockerContainer describes a container which uses the docker runtime.
type dockerContainer struct {
	Image        string        `json:"image"`
	PortMappings []portMapping `json:"portMappings"`
}

// PortMapping describes in which port the process are binding inside the docker container.
type portMapping struct {
	Labels        map[string]string `json:"labels"`
	ContainerPort uint32            `json:"containerPort"`
	HostPort      uint32            `json:"hostPort"`
	ServicePort   uint32            `json:"servicePort"`
}

// PortDefinition describes which load balancer port you should access to access the service.
type portDefinition struct {
	Labels map[string]string `json:"labels"`
	Port   uint32            `json:"port"`
}

// Network describes the name and type of network the container is attached to.
type network struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

// isContainerNet checks if the app's first network is set to mode 'container'.
func (app app) isContainerNet() bool {
	return len(app.Networks) > 0 && app.Networks[0].Mode == "container"
}

// GetAppsList randomly choose a server from the `servers` and get the list of running applications.
func GetAppsList(ac *apiConfig) (*AppList, error) {
	c := ac.cs[rand.Intn(len(ac.cs))]

	// /v2/apps API: get the list of running applications.
	// https://mesosphere.github.io/marathon/api-console/index.html
	path := "/v2/apps/?embed=apps.tasks"
	resp, err := c.GetAPIResponse(path)
	if err != nil {
		return nil, fmt.Errorf("cannot get Marathon response from %s: %w", path, err)
	}

	var apps AppList
	err = json.Unmarshal(resp, &apps)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal AppList obtained from %q: %w; response=%q", path, err, resp)
	}

	return &apps, nil
}
