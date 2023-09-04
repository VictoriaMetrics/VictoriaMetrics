package azure

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// networkInterface a network interface in a resource group.
type networkInterface struct {
	Properties networkProperties `json:"properties,omitempty"`
}

type networkProperties struct {
	// Primary - Gets whether this is a primary network interface on a virtual machine.
	Primary          bool              `json:"primary,omitempty"`
	IPConfigurations []ipConfiguration `json:"ipConfigurations,omitempty"`
}

type ipConfiguration struct {
	Properties ipProperties `json:"properties,omitempty"`
}

type ipProperties struct {
	PublicIPAddress  publicIPAddress `json:"publicIPAddress,omitempty"`
	PrivateIPAddress string          `json:"privateIPAddress,omitempty"`
}

type publicIPAddress struct {
	Properties publicIPProperties `json:"properties,omitempty"`
}

type publicIPProperties struct {
	IPAddress string `json:"ipAddress,omitempty"`
}

func enrichVMNetworkInterfaces(ac *apiConfig, vm *virtualMachine) error {
	for _, nicRef := range vm.Properties.NetworkProfile.NetworkInterfaces {
		isScaleSetVM := vm.scaleSet != ""
		nic, err := getNIC(ac, nicRef.ID, isScaleSetVM)
		if err != nil {
			return err
		}
		// only primary interface is relevant for us
		// mimic Prometheus logic
		if nic.Properties.Primary {
			for _, ipCfg := range nic.Properties.IPConfigurations {
				vm.ipAddresses = append(vm.ipAddresses, vmIPAddress{
					publicIP:  ipCfg.Properties.PublicIPAddress.Properties.IPAddress,
					privateIP: ipCfg.Properties.PrivateIPAddress,
				})
			}
		}
	}
	return nil
}

// See https://docs.microsoft.com/en-us/rest/api/virtualnetwork/network-interfaces/get
func getNIC(ac *apiConfig, id string, isScaleSetVM bool) (*networkInterface, error) {
	// https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Network/networkInterfaces/{networkInterfaceName}?api-version=2021-08-01
	apiQueryParams := "api-version=2021-08-01&$expand=ipConfigurations/publicIPAddress"
	// special case for VMs managed by ScaleSet
	// it's not documented at API docs.
	if isScaleSetVM {
		apiQueryParams = "api-version=2021-03-01&$expand=ipConfigurations/publicIPAddress"
	}
	apiURL := id + "?" + apiQueryParams
	resp, err := ac.c.GetAPIResponseWithReqParams(apiURL, func(request *http.Request) {
		request.Header.Set("Authorization", "Bearer "+ac.mustGetAuthToken())
	})
	if err != nil {
		return nil, fmt.Errorf("cannot execute api request at %s :%w", apiURL, err)
	}
	var nic networkInterface
	if err := json.Unmarshal(resp, &nic); err != nil {
		return nil, fmt.Errorf("cannot parse network-interface api response %q: %w", resp, err)
	}
	return &nic, nil
}
