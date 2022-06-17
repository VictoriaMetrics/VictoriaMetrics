package azure

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/fasthttp"
)

// virtualMachine represents an Azure virtual machine (which can also be created by a VMSS)
type virtualMachine struct {
	ID         string                   `json:"id,omitempty"`
	Name       string                   `json:"name,omitempty"`
	Type       string                   `json:"type,omitempty"`
	Location   string                   `json:"location,omitempty"`
	Properties virtualMachineProperties `json:"properties,omitempty"`
	Tags       map[string]string        `json:"tags,omitempty"`
	// enriched during service discovery
	scaleSet    string
	ipAddresses []vmIPAddress
}

type vmIPAddress struct {
	publicIP  string
	privateIP string
}

type virtualMachineProperties struct {
	NetworkProfile networkProfile `json:"networkProfile,omitempty"`
	OsProfile      osProfile      `json:"osProfile,omitempty"`
	StorageProfile storageProfile `json:"storageProfile,omitempty"`
}

type storageProfile struct {
	OsDisk osDisk `json:"osDisk,omitempty"`
}

type osDisk struct {
	OsType string `json:"osType,omitempty"`
}

type osProfile struct {
	ComputerName string `json:"computerName,omitempty"`
}
type networkProfile struct {
	// NetworkInterfaces - Specifies the list of resource Ids for the network interfaces associated with the virtual machine.
	NetworkInterfaces []networkInterfaceReference `json:"networkInterfaces,omitempty"`
}

type networkInterfaceReference struct {
	ID string `json:"id,omitempty"`
}

// listAPIResponse generic response from list api
type listAPIResponse struct {
	NextLink string            `json:"nextLink"`
	Value    []json.RawMessage `json:"value"`
}

// visitAllAPIObjects iterates over list API with pagination and applies callback for each response object
func visitAllAPIObjects(ac *apiConfig, apiURL string, cb func(data json.RawMessage) error) error {
	nextLink := apiURL
	for nextLink != "" {
		resp, err := ac.c.GetAPIResponseWithReqParams(nextLink, func(request *fasthttp.Request) {
			request.Header.Set("Authorization", "Bearer "+ac.mustGetAuthToken())
		})
		if err != nil {
			return fmt.Errorf("cannot execute azure api request for url: %s : %w", nextLink, err)
		}
		var lar listAPIResponse
		if err := json.Unmarshal(resp, &lar); err != nil {
			return fmt.Errorf("cannot parse azure api response: %q at url: %s, err: %w", string(resp), nextLink, err)
		}
		for i := range lar.Value {
			if err := cb(lar.Value[i]); err != nil {
				return err
			}
		}
		nextLink = lar.NextLink
	}
	return nil
}

// getVirtualMachines
func getVirtualMachines(ac *apiConfig) ([]virtualMachine, error) {
	vms, err := listVMs(ac)
	if err != nil {
		return nil, fmt.Errorf("cannot list virtual machines: %w", err)
	}
	scaleSetRefs, err := listScaleSetRefs(ac)
	if err != nil {
		return nil, fmt.Errorf("cannot list scaleSets: %w", err)
	}
	ssvms, err := listScaleSetVMs(ac, scaleSetRefs)
	if err != nil {
		return nil, fmt.Errorf("cannot list scaleSets virtual machines: %w", err)
	}
	vms = append(vms, ssvms...)
	// operations IO bound, it's ok to spawn move goroutines then CPU
	concurrency := cgroup.AvailableCPUs() * 10
	limiter := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var firstErr error
	var errLock sync.Mutex
	for i := range vms {
		limiter <- struct{}{}
		vm := &vms[i]
		wg.Add(1)
		go func(vm *virtualMachine) {
			defer wg.Done()
			if err := enrichVMNetworkInterface(ac, vm); err != nil {
				errLock.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("cannot enrich network interface for vm: %w", err)
				}
				errLock.Unlock()
			}
			<-limiter
		}(vm)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return vms, nil
}

// https://docs.microsoft.com/en-us/rest/api/compute/virtual-machines/list-all
// https://docs.microsoft.com/en-us/rest/api/compute/virtual-machines/list
func listVMs(ac *apiConfig) ([]virtualMachine, error) {
	var vms []virtualMachine
	// https://management.azure.com/subscriptions/{subscriptionId}/providers/Microsoft.Compute/virtualMachines?api-version=2022-03-01
	apiURL := "/subscriptions/" + ac.subscriptionID
	if len(ac.resourceGroup) > 0 {
		// special case filter by resourceGroup
		// https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/virtualMachines?api-version=2022-03-01
		apiURL += "/resourceGroups/" + ac.resourceGroup
	}
	apiURL += "/providers/Microsoft.Compute/virtualMachines?api-version=2022-03-01"
	err := visitAllAPIObjects(ac, apiURL, func(data json.RawMessage) error {
		var vm virtualMachine
		if err := json.Unmarshal(data, &vm); err != nil {
			return fmt.Errorf("cannot parse list VirtualMachines API response: %q : %w", string(data), err)
		}
		vms = append(vms, vm)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return vms, nil
}

type scaleSet struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// https://docs.microsoft.com/en-us/rest/api/compute/virtual-machine-scale-sets/list-all
// GET https://management.azure.com/subscriptions/{subscriptionId}/providers/Microsoft.Compute/virtualMachineScaleSets?api-version=2022-03-01
func listScaleSetRefs(ac *apiConfig) ([]scaleSet, error) {
	var ssrs []scaleSet
	apiURL := "/subscriptions/" + ac.subscriptionID + "/providers/Microsoft.Compute/virtualMachineScaleSets?api-version=2022-03-01"
	err := visitAllAPIObjects(ac, apiURL, func(data json.RawMessage) error {
		var ss scaleSet
		if err := json.Unmarshal(data, &ss); err != nil {
			return err
		}
		ssrs = append(ssrs, ss)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ssrs, nil
}

// https://docs.microsoft.com/en-us/rest/api/compute/virtual-machine-scale-set-vms/list
// GET https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/virtualMachineScaleSets/{virtualMachineScaleSetName}/virtualMachines?api-version=2022-03-01
func listScaleSetVMs(ac *apiConfig, ssrs []scaleSet) ([]virtualMachine, error) {
	var vms []virtualMachine
	for _, ssr := range ssrs {
		apiURI := ssr.ID + "/virtualMachines?api-version=2022-03-01"
		err := visitAllAPIObjects(ac, apiURI, func(data json.RawMessage) error {
			var vm virtualMachine
			if err := json.Unmarshal(data, &vm); err != nil {
				return fmt.Errorf("cannot parse ScaleSet list API response: %q: %w", string(data), err)
			}
			vm.scaleSet = ssr.Name
			vms = append(vms, vm)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return vms, nil
}
