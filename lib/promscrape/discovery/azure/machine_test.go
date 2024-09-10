package azure

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

var (
	testServerURL string
	listAPICall   int
)

func TestGetVirtualMachinesSuccess(t *testing.T) {
	prettifyVMs := func(src []virtualMachine) string {
		var sb strings.Builder
		for idx, vm := range src {
			fmt.Fprintf(&sb, `idx: %d, vm: Name: %q, ID: %q, Location: %q, Type: %q, ComputerName: %q, OsType: %q, scaleSet: %q`,
				idx, vm.Name, vm.ID, vm.Location, vm.Type, vm.Properties.OsProfile.ComputerName, vm.Properties.StorageProfile.OsDisk.OsType, vm.scaleSet)
			if vm.Tags != nil {
				fmt.Fprint(&sb, " vmtags: ")
			}
			for tagK, tagV := range vm.Tags {
				fmt.Fprintf(&sb, `%q: %q, `, tagK, tagV)
			}
			if len(vm.Properties.NetworkProfile.NetworkInterfaces) > 0 {
				fmt.Fprint(&sb, " network ints: ")
			}
			for idx, nic := range vm.Properties.NetworkProfile.NetworkInterfaces {
				fmt.Fprintf(&sb, " idx %d, ID: %q", idx, nic.ID)
			}
			if len(vm.ipAddresses) > 0 {
				fmt.Fprint(&sb, " ip addresses: ")
			}
			for idx, ip := range vm.ipAddresses {
				fmt.Fprintf(&sb, "idx: %d, PrivateIP: %q, PublicIP: %q", idx, ip.privateIP, ip.publicIP)
			}
			fmt.Fprintf(&sb, "\n")
		}
		return sb.String()
	}
	f := func(name string, expectedVMs []virtualMachine, apiResponses [5]string) {
		t.Run(name, func(t *testing.T) {
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				// list vms response
				case strings.Contains(r.URL.Path, "/providers/Microsoft.Compute/virtualMachines"):
					w.WriteHeader(http.StatusOK)
					if listAPICall == 0 {
						// with nextLink
						apiResponse := strings.Replace(apiResponses[0], "{nextLinkPlaceHolder}", testServerURL+"/providers/Microsoft.Compute/virtualMachines", 1)
						fmt.Fprint(w, apiResponse)
						listAPICall++
					} else {
						// without nextLink
						fmt.Fprint(w, apiResponses[1])
					}
					// list scaleSets response
				case strings.Contains(r.URL.RequestURI(), "/providers/Microsoft.Compute/virtualMachineScaleSets?api-version=2022-03-01"):
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, apiResponses[2])
					// list scalesets vms response
				case strings.Contains(r.URL.Path, "/providers/Microsoft.Compute/virtualMachineScaleSets/{virtualMachineScaleSetName}/virtualMach"):
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, apiResponses[3])
					// nic response
				case strings.Contains(r.URL.Path, "/networkInterfaces/"):
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, apiResponses[4])
				default:
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(w, "API path not found: %s", r.URL.Path)
				}
			}))
			defer testServer.Close()
			testServerURL = testServer.URL
			c, err := discoveryutils.NewClient(testServer.URL, nil, nil, nil, &promauth.HTTPClientConfig{})
			if err != nil {
				t.Fatalf("unexpected error at client create: %s", err)
			}
			u, _ := url.Parse(c.APIServer())

			defer c.Stop()
			ac := &apiConfig{
				c:              c,
				apiServerHost:  u.Hostname(),
				subscriptionID: "some-id",
				refreshToken: func() (string, time.Duration, error) {
					return "auth-token", 0, nil
				},
			}
			gotVMs, err := getVirtualMachines(ac)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if !reflect.DeepEqual(gotVMs, expectedVMs) {
				t.Fatalf("unexpected test result\ngot:\n%s\nwant:\n%s", prettifyVMs(gotVMs), prettifyVMs(expectedVMs))
			}
		})
	}
	f("discover single vm", []virtualMachine{
		{
			Name: "{virtualMachineName}", ID: "/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/virtualMachines/{virtualMachineName}",
			Location: "eastus", Type: "Microsoft.Compute/virtualMachines",
			Properties: virtualMachineProperties{
				NetworkProfile: networkProfile{NetworkInterfaces: []networkInterfaceReference{{ID: "/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Network/networkInterfaces/{networkInterfaceName}"}}},
				OsProfile:      osProfile{ComputerName: "Test"},
				StorageProfile: storageProfile{OsDisk: osDisk{OsType: "Windows"}},
			},
			ipAddresses: []vmIPAddress{
				{publicIP: "20.30.40.50", privateIP: "172.20.2.4"},
			},
			Tags: map[string]string{},
		},
	}, [5]string{
		`
{
  "value": [
    { "id": "/some-vm/id",
      "properties": {
        "vmId": "{vmId}",
        "storageProfile": {
          "imageReference": {
            "publisher": "MicrosoftWindowsServer",
            "offer": "WindowsServer",
            "sku": "2012-R2-Datacenter",
            "version": "4.127.20170406",
            "exactVersion": "aaaaaaaaaaaaa",
            "sharedGalleryImageId": "aaaaaaaaaaaaaaa",
            "communityGalleryImageId": "aaaa",
            "id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
          },
          "osDisk": {
            "osType": "Windows",
            "name": "test",
            "createOption": "FromImage",
            "vhd": {
              "uri": "https://{storageAccountName}.blob.core.windows.net/{containerName}/{vhdName}.vhd"
            },
            "caching": "None",
            "diskSizeGB": 127,
            "encryptionSettings": {
              "diskEncryptionKey": {
                "secretUrl": "aaaaaaaaa",
                "sourceVault": {
                  "id": "/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/availabilitySets/{availabilitySetName}"
                }
              },
              "keyEncryptionKey": {
                "keyUrl": "aaaaaaaaaaaaa",
                "sourceVault": {
                  "id": "/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/availabilitySets/{availabilitySetName}"
                }
              },
              "enabled": true
            },
            "image": {
              "uri": "https://{storageAccountName}.blob.core.windows.net/{containerName}/{vhdName}.vhd"
            },
            "writeAcceleratorEnabled": true,
            "diffDiskSettings": {
              "option": "Local",
              "placement": "CacheDisk"
            },
            "managedDisk": {
              "storageAccountType": "Standard_LRS",
              "diskEncryptionSet": {
                "id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaa"
              },
              "securityProfile": {
                "securityEncryptionType": "VMGuestStateOnly",
                "diskEncryptionSet": {
                  "id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaa"
                }
              },
              "id": "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/disks/testingexcludedisk_OsDisk_1_74cdaedcea50483d9833c96adefa100f"
            },
            "deleteOption": "Delete"
          },
          "dataDisks": []
        },
        "osProfile": {
          "computerName": "Test",
          "adminUsername": "Foo12"
        },
        "networkProfile": {
          "networkInterfaces": [
            {
              "id": "/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Network/networkInterfaces/{networkInterfaceName}",
              "properties": {
                "primary": true,
                "deleteOption": "Delete"
              }
            }
          ],
          "networkApiVersion": "2020-11-01"
        }
      },
      "type": "Microsoft.Compute/virtualMachines",
      "location": "eastus",
      "tags": {},
      "id": "/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/virtualMachines/{virtualMachineName}",
      "name": "{virtualMachineName}"
    }
  ],
  "nextLink": "{nextLinkPlaceHolder}"
}`,
		`{
  "value": [],
  "nextLink": ""
}`,
		`{}`,
		`{}`,
		`{
  "name": "test-nic",
  "properties": {
    "ipConfigurations": [
      {
        "name": "ipconfig1",
        "properties": {
          "privateIPAddress": "172.20.2.4",
          "publicIPAddress": {
            "properties": {
              "ipAddress": "20.30.40.50"
            }
          },
          "primary": true
        }
      }
    ],
    "primary": true
  },
  "type": "Microsoft.Network/networkInterfaces"
}`,
	})

	f("discover vm with scaleSet", []virtualMachine{
		{
			Name: "{vmss-vm-name}", ID: "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/virtualMachineScaleSets/{vmss-name}/virtualMachines/0",
			Location: "westus", Type: "Microsoft.Compute/virtualMachines",
			Properties: virtualMachineProperties{
				NetworkProfile: networkProfile{NetworkInterfaces: []networkInterfaceReference{
					{ID: "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/virtualMachineScaleSets/{vmss-name}/virtualMachines/0/networkInterfaces/vmsstestnetconfig5415"},
					{ID: "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/virtualMachineScaleSets/{vmss-name}/virtualMachines/0/networkInterfaces/vmsstestnetconfig5415"},
				}},
				OsProfile:      osProfile{ComputerName: "test000000"},
				StorageProfile: storageProfile{OsDisk: osDisk{OsType: "Windows"}},
			},
			scaleSet: "{virtualMachineScaleSetName}",
			ipAddresses: []vmIPAddress{
				{publicIP: "20.30.40.50", privateIP: "172.20.2.4"},
				{publicIP: "20.30.40.50", privateIP: "172.20.2.4"},
			},
			Tags: map[string]string{},
		},
		{
			Name: "{vmss-vm-name}", ID: "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/virtualMachineScaleSets/{vmss-name}/virtualMachines/15",
			Location: "westp", Type: "Microsoft.Compute/virtualMachines",
			Properties: virtualMachineProperties{
				NetworkProfile: networkProfile{NetworkInterfaces: []networkInterfaceReference{
					{ID: "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/virtualMachineScaleSets/{vmss-name}/virtualMachines/0/networkInterfaces/vmsstestnetconfig5415"},
				}},
				OsProfile:      osProfile{ComputerName: "test-15"},
				StorageProfile: storageProfile{OsDisk: osDisk{OsType: "Linux"}},
			},
			scaleSet: "{virtualMachineScaleSetName}",
			ipAddresses: []vmIPAddress{
				{publicIP: "20.30.40.50", privateIP: "172.20.2.4"},
			},
			Tags: map[string]string{},
		},
	}, [5]string{
		`{}`,
		`{}`,
		`{
  "value": [
    {
      "sku": {
        "tier": "Standard",
        "capacity": 3,
        "name": "Standard_D1_v2"
      },
      "location": "westus",
      "properties": { },
      "id": "/subscriptions/{subscription-id}/resourceGroups/{resourceGroupName}/providers/Microsoft.Compute/virtualMachineScaleSets/{virtualMachineScaleSetName}",
      "name": "{virtualMachineScaleSetName}",
      "type": "Microsoft.Compute/virtualMachineScaleSets",
      "tags": {
        "key8425": "aaa"
      }
    }
  ],
  "nextLink": ""
}`,
		`
{
  "value": [
    {
      "name": "{vmss-vm-name}",
      "id": "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/virtualMachineScaleSets/{vmss-name}/virtualMachines/0",
      "type": "Microsoft.Compute/virtualMachines",
      "location": "westus",
      "tags": {},
      "properties": {
        "storageProfile": {
          "osDisk": {
            "osType": "Windows"
          }
        },
        "osProfile": {
          "computerName": "test000000"
        },
        "networkProfile": {
          "networkInterfaces": [
            {
              "id": "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/virtualMachineScaleSets/{vmss-name}/virtualMachines/0/networkInterfaces/vmsstestnetconfig5415",
              "properties": {
                "primary": true,
                "deleteOption": "Delete"
              }
            },
            {
              "id": "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/virtualMachineScaleSets/{vmss-name}/virtualMachines/0/networkInterfaces/vmsstestnetconfig5415",
              "properties": {
                "primary": true,
                "deleteOption": "Delete"
              }
            }
          ]
        },
        "licenseType": "aaaaaaaaaa",
        "protectionPolicy": {
          "protectFromScaleIn": true,
          "protectFromScaleSetActions": true
        }
      }
    },
    {
      "name": "{vmss-vm-name}",
      "id": "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/virtualMachineScaleSets/{vmss-name}/virtualMachines/15",
      "type": "Microsoft.Compute/virtualMachines",
      "location": "westp",
      "tags": {},
      "properties": {
        "storageProfile": {
          "osDisk": {
            "osType": "Linux"
          }
        },
        "osProfile": {
          "computerName": "test-15"
        },
        "networkProfile": {
          "networkInterfaces": [
            {
              "id": "/subscriptions/{subscription-id}/resourceGroups/myResourceGroup/providers/Microsoft.Compute/virtualMachineScaleSets/{vmss-name}/virtualMachines/0/networkInterfaces/vmsstestnetconfig5415",
              "properties": {
                "primary": true,
                "deleteOption": "Delete"
              }
            }
          ]
        },
        "licenseType": "aaaaaaaaaa"
      }
    }

  ],
  "nextLink": ""
}`,
		`{
  "name": "test-nic",
  "properties": {
    "ipConfigurations": [
      {
        "name": "ipconfig1",
        "properties": {
          "privateIPAddress": "172.20.2.4",
          "publicIPAddress": {
            "properties": {
              "ipAddress": "20.30.40.50"
            }
          },
          "primary": true
        }
      }
    ],
    "primary": true
  },
  "type": "Microsoft.Network/networkInterfaces"
}`,
	})
}
