package azure

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
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
	NetworkProfile  networkProfile  `json:"networkProfile,omitempty"`
	OsProfile       osProfile       `json:"osProfile,omitempty"`
	StorageProfile  storageProfile  `json:"storageProfile,omitempty"`
	HardwareProfile hardwareProfile `json:"hardwareProfile,omitempty"`
}

type hardwareProfile struct {
	VMSize string `json:"vmSize,omitempty"`
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

// visitAllAPIObjects iterates over list API with pagination and applies cb for each response object
func visitAllAPIObjects(ac *apiConfig, apiURL string, cb func(data json.RawMessage) error) error {
	nextLinkURI := apiURL
	for {
		resp, err := ac.c.GetAPIResponseWithReqParams(nextLinkURI, func(request *http.Request) {
			request.Header.Set("Authorization", "Bearer "+ac.mustGetAuthToken())
		})
		if err != nil {
			return fmt.Errorf("cannot execute azure api request at %s: %w", nextLinkURI, err)
		}
		var lar listAPIResponse
		if err := json.Unmarshal(resp, &lar); err != nil {
			return fmt.Errorf("cannot parse azure api response %q obtained from %s: %w", resp, nextLinkURI, err)
		}
		for i := range lar.Value {
			if err := cb(lar.Value[i]); err != nil {
				return err
			}
		}

		// Azure API returns NextLink with apiServer in it, so we need to remove it.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3247
		if lar.NextLink == "" {
			break
		}
		nextURL, err := url.Parse(lar.NextLink)
		if err != nil {
			return fmt.Errorf("cannot parse nextLink from response %q: %w", lar.NextLink, err)
		}

		// Sometimes Azure will respond a host with a port. Since all possible apiServer defined in cloudEnvironments do not include a port,
		// it is best to check the host without the port. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6912
		if nextURL.Host != "" && nextURL.Hostname() != ac.apiServerHost {
			return fmt.Errorf("unexpected nextLink host %q, expecting %q", nextURL.Hostname(), ac.apiServerHost)
		}

		nextLinkURI = nextURL.RequestURI()
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
		return nil, fmt.Errorf("cannot list virtual machines for scaleSets: %w", err)
	}
	vms = append(vms, ssvms...)
	if err := enrichVirtualMachinesNetworkInterfaces(ac, vms); err != nil {
		return nil, fmt.Errorf("cannot discover network interfaces for virtual machines: %w", err)
	}
	return vms, nil
}

func enrichVirtualMachinesNetworkInterfaces(ac *apiConfig, vms []virtualMachine) error {
	concurrency := cgroup.AvailableCPUs() * 10
	workCh := make(chan *virtualMachine, concurrency)
	resultCh := make(chan error, concurrency)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for vm := range workCh {
				err := enrichVMNetworkInterfaces(ac, vm)
				resultCh <- err
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range vms {
			workCh <- &vms[i]
		}
		close(workCh)
	}()
	var firstErr error
	for range vms {
		err := <-resultCh
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	wg.Wait()
	return firstErr
}

// See https://docs.microsoft.com/en-us/rest/api/compute/virtual-machines/list-all
func listVMs(ac *apiConfig) ([]virtualMachine, error) {
	// https://management.azure.com/subscriptions/{subscriptionId}/providers/Microsoft.Compute/virtualMachines?api-version=2022-03-01
	apiURL := "/subscriptions/" + ac.subscriptionID
	if ac.resourceGroup != "" {
		// special case filter by resourceGroup
		// https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/virtualMachines?api-version=2022-03-01
		apiURL += "/resourceGroups/" + ac.resourceGroup
	}
	apiURL += "/providers/Microsoft.Compute/virtualMachines?api-version=2022-03-01"
	var vms []virtualMachine
	err := visitAllAPIObjects(ac, apiURL, func(data json.RawMessage) error {
		var vm virtualMachine
		if err := json.Unmarshal(data, &vm); err != nil {
			return fmt.Errorf("cannot parse virtualMachine list API response %q: %w", data, err)
		}
		vms = append(vms, vm)
		return nil
	})
	return vms, err
}

type scaleSet struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// See https://docs.microsoft.com/en-us/rest/api/compute/virtual-machine-scale-sets/list-all
// and https://learn.microsoft.com/en-us/rest/api/compute/virtual-machine-scale-sets/list (need resourceGroup)
func listScaleSetRefs(ac *apiConfig) ([]scaleSet, error) {
	// https://management.azure.com/subscriptions/{subscriptionId}/providers/Microsoft.Compute/virtualMachineScaleSets?api-version=2022-03-01
	apiURL := "/subscriptions/" + ac.subscriptionID
	if ac.resourceGroup != "" {
		// special case filter by resourceGroup
		// https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/virtualMachineScaleSets?api-version=2022-03-01
		apiURL += "/resourceGroups/" + ac.resourceGroup
	}
	apiURL += "/providers/Microsoft.Compute/virtualMachineScaleSets?api-version=2022-03-01"
	var sss []scaleSet
	err := visitAllAPIObjects(ac, apiURL, func(data json.RawMessage) error {
		var ss scaleSet
		if err := json.Unmarshal(data, &ss); err != nil {
			return fmt.Errorf("cannot parse scaleSet list API response %q: %w", data, err)
		}
		sss = append(sss, ss)
		return nil
	})
	return sss, err
}

// See https://docs.microsoft.com/en-us/rest/api/compute/virtual-machine-scale-set-vms/list
func listScaleSetVMs(ac *apiConfig, sss []scaleSet) ([]virtualMachine, error) {
	// https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/virtualMachineScaleSets/{virtualMachineScaleSetName}/virtualMachines?api-version=2022-03-01
	var vms []virtualMachine
	for _, ss := range sss {
		apiURI := ss.ID + "/virtualMachines?api-version=2022-03-01"
		err := visitAllAPIObjects(ac, apiURI, func(data json.RawMessage) error {
			var vm virtualMachine
			if err := json.Unmarshal(data, &vm); err != nil {
				return fmt.Errorf("cannot parse virtualMachine list API response %q: %w", data, err)
			}
			vm.scaleSet = ss.Name
			vms = append(vms, vm)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return vms, nil
}
