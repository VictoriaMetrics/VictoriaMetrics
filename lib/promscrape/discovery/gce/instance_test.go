package gce

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func TestParseInstanceListFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		il, err := parseInstanceList([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if il != nil {
			t.Fatalf("unexpected non-nil InstanceList: %v", il)
		}
	}
	f(``)
	f(`[1,23]`)
	f(`{"items":[{"metadata":1}]}`)
}

func TestParseInstanceListSuccess(t *testing.T) {
	data := `
{
  "id": "projects/victoriametrics-test/zones/us-east1-b/instances",
  "items": [
    {
      "id": "7897352091592122",
      "creationTimestamp": "2020-02-16T07:10:14.357-08:00",
      "name": "play-1m-1-vmagent",
      "tags": {
        "items": [
          "play",
          "play-1m-1",
          "vmagent"
        ],
        "fingerprint": "O44NvJ36CCo="
      },
      "machineType": "https://www.googleapis.com/compute/v1/projects/victoriametrics-test/zones/us-east1-b/machineTypes/f1-micro",
      "status": "RUNNING",
      "zone": "https://www.googleapis.com/compute/v1/projects/victoriametrics-test/zones/us-east1-b",
      "networkInterfaces": [
        {
          "network": "https://www.googleapis.com/compute/v1/projects/victoriametrics-test/global/networks/default",
          "subnetwork": "https://www.googleapis.com/compute/v1/projects/victoriametrics-test/regions/us-east1/subnetworks/play-1m-1-snw",
          "networkIP": "10.11.2.7",
          "name": "nic0",
          "fingerprint": "O4eNOfaplJ4=",
          "kind": "compute#networkInterface"
        }
      ],
      "disks": [
        {
          "type": "PERSISTENT",
          "mode": "READ_WRITE",
          "source": "https://www.googleapis.com/compute/v1/projects/victoriametrics-test/zones/us-east1-b/disks/play-1m-1-vmagent",
          "deviceName": "boot",
          "index": 0,
          "boot": true,
          "autoDelete": true,
          "licenses": [
            "https://www.googleapis.com/compute/v1/projects/cos-cloud-shielded/global/licenses/shielded-cos",
            "https://www.googleapis.com/compute/v1/projects/cos-cloud/global/licenses/cos",
            "https://www.googleapis.com/compute/v1/projects/cos-cloud/global/licenses/cos-pcid"
          ],
          "interface": "SCSI",
          "guestOsFeatures": [
            {
              "type": "VIRTIO_SCSI_MULTIQUEUE"
            },
            {
              "type": "UEFI_COMPATIBLE"
            }
          ],
          "diskSizeGb": "10",
          "kind": "compute#attachedDisk"
        }
      ],
      "metadata": {
        "fingerprint": "BAFZwTyaAxQ=",
        "items": [
          {
            "key": "gce-container-declaration",
            "value": "foobar"
	  }
        ],
        "kind": "compute#metadata"
      },
      "serviceAccounts": [
        {
          "email": "12-compute@developer.gserviceaccount.com",
          "scopes": [
            "https://www.googleapis.com/auth/devstorage.read_write",
            "https://www.googleapis.com/auth/logging.write",
            "https://www.googleapis.com/auth/monitoring.write",
            "https://www.googleapis.com/auth/servicecontrol",
            "https://www.googleapis.com/auth/service.management.readonly"
          ]
        }
      ],
      "selfLink": "https://www.googleapis.com/compute/v1/projects/victoriametrics-test/zones/us-east1-b/instances/play-1m-1-vmagent",
      "scheduling": {
        "onHostMaintenance": "MIGRATE",
        "automaticRestart": true,
        "preemptible": false
      },
      "cpuPlatform": "Intel Haswell",
      "labels": {
        "goog-dm": "play-deployment",
        "cluster_num": "1",
        "cluster_retention": "1m",
        "env": "play",
        "type": "vmagent"
      },
      "labelFingerprint": "-CXeRXMQiVc=",
      "startRestricted": false,
      "deletionProtection": false,
      "shieldedInstanceConfig": {
        "enableSecureBoot": false,
        "enableVtpm": true,
        "enableIntegrityMonitoring": true
      },
      "shieldedInstanceIntegrityPolicy": {
        "updateAutoLearnPolicy": true
      },
      "fingerprint": "hd3NB2-9QIg=",
      "kind": "compute#instance"
    }
  ]
}
`
	il, err := parseInstanceList([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(il.Items) != 1 {
		t.Fatalf("unexpected length of InstanceList.Items; got %d; want %d", len(il.Items), 1)
	}
	inst := il.Items[0]

	// Check inst.appendTargetLabels()
	project := "proj-1"
	tagSeparator := ","
	port := 80
	labelss := inst.appendTargetLabels(nil, project, tagSeparator, port)
	var sortedLabelss [][]prompbmarshal.Label
	for _, labels := range labelss {
		sortedLabelss = append(sortedLabelss, discoveryutils.GetSortedLabels(labels))
	}
	expectedLabelss := [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__":                                   "10.11.2.7:80",
			"__meta_gce_instance_id":                        "7897352091592122",
			"__meta_gce_instance_name":                      "play-1m-1-vmagent",
			"__meta_gce_instance_status":                    "RUNNING",
			"__meta_gce_label_cluster_num":                  "1",
			"__meta_gce_label_cluster_retention":            "1m",
			"__meta_gce_label_env":                          "play",
			"__meta_gce_label_goog_dm":                      "play-deployment",
			"__meta_gce_label_type":                         "vmagent",
			"__meta_gce_machine_type":                       "https://www.googleapis.com/compute/v1/projects/victoriametrics-test/zones/us-east1-b/machineTypes/f1-micro",
			"__meta_gce_metadata_gce_container_declaration": "foobar",
			"__meta_gce_network":                            "https://www.googleapis.com/compute/v1/projects/victoriametrics-test/global/networks/default",
			"__meta_gce_private_ip":                         "10.11.2.7",
			"__meta_gce_project":                            "proj-1",
			"__meta_gce_subnetwork":                         "https://www.googleapis.com/compute/v1/projects/victoriametrics-test/regions/us-east1/subnetworks/play-1m-1-snw",
			"__meta_gce_tags":                               ",play,play-1m-1,vmagent,",
			"__meta_gce_zone":                               "https://www.googleapis.com/compute/v1/projects/victoriametrics-test/zones/us-east1-b",
		}),
	}
	if !reflect.DeepEqual(sortedLabelss, expectedLabelss) {
		t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, expectedLabelss)
	}
}
