package azure

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func TestAppendMachineLabels(t *testing.T) {
	f := func(name string, vms []virtualMachine, expectedLabels [][]prompbmarshal.Label) {
		t.Run(name, func(t *testing.T) {
			labelss := appendMachineLabels(vms, 80, &SDConfig{SubscriptionID: "some-id"})
			var sortedLabelss [][]prompbmarshal.Label
			for _, labels := range labelss {
				sortedLabelss = append(sortedLabelss, discoveryutils.GetSortedLabels(labels))
			}
			if !reflect.DeepEqual(sortedLabelss, expectedLabels) {
				t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, expectedLabels)
			}
		})
	}
	f("single vm", []virtualMachine{
		{
			Name:       "vm-1",
			ID:         "id-2",
			Type:       "Azure",
			Location:   "eu-west-1",
			Properties: virtualMachineProperties{OsProfile: osProfile{ComputerName: "test-1"}, StorageProfile: storageProfile{OsDisk: osDisk{OsType: "Linux"}}},
			Tags:       map[string]string{"key-1": "value-1"},
			ipAddresses: []vmIPAddress{
				{privateIP: "10.10.10.1"},
			},
		},
	}, [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__":                        "10.10.10.1:80",
			"__meta_azure_machine_id":            "id-2",
			"__meta_azure_subscription_id":       "some-id",
			"__meta_azure_machine_os_type":       "Linux",
			"__meta_azure_machine_name":          "vm-1",
			"__meta_azure_machine_computer_name": "test-1",
			"__meta_azure_machine_location":      "eu-west-1",
			"__meta_azure_machine_private_ip":    "10.10.10.1",
			"__meta_azure_machine_tag_key_1":     "value-1",
		}),
	})
}
