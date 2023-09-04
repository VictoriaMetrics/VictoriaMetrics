package ec2

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestDescribeAvailabilityZonesResponse(t *testing.T) {
	data := `<DescribeAvailabilityZonesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
    <requestId>e23c5a54-a29c-43ee-8b55-0c13c26e9e01</requestId>
    <availabilityZoneInfo>
        <item>
            <optInStatus>opt-in-not-required</optInStatus>
            <zoneName>us-west-2a</zoneName>
            <zoneId>usw2-az1</zoneId>
            <zoneState>available</zoneState>
            <regionName>us-west-2</regionName>
            <messageSet/>
            <NetworkBorderGroup>us-west-2-lax-1</NetworkBorderGroup>
        </item>
        <item>
            <groupName>us-west-2</groupName>
            <optInStatus>opt-in-not-required</optInStatus>
            <zoneName>us-west-2b</zoneName>
            <zoneId>usw2-az2</zoneId>
            <zoneState>available</zoneState>
            <regionName>us-west-2</regionName>
            <messageSet/>
            <NetworkBorderGroup>us-west-2-lax-1</NetworkBorderGroup>
        </item>
    </availabilityZoneInfo>
</DescribeAvailabilityZonesResponse>
`

	azr, err := parseAvailabilityZonesResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error when parsing data: %s", err)
	}
	azrExpected := &AvailabilityZonesResponse{
		AvailabilityZoneInfo: AvailabilityZoneInfo{
			Items: []AvailabilityZone{
				{
					ZoneName: "us-west-2a",
					ZoneID:   "usw2-az1",
				},
				{
					ZoneName: "us-west-2b",
					ZoneID:   "usw2-az2",
				},
			},
		},
	}
	if !reflect.DeepEqual(azr, azrExpected) {
		t.Fatalf("unexpected DescribeAvailabilityZonesResponse parsed;\ngot\n%+v\nwant\n%+v", azr, azrExpected)
	}
}

func TestParseInstancesResponse(t *testing.T) {
	data := `<?xml version="1.0" encoding="UTF-8"?>
<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2013-10-15/">
    <requestId>98667f8e-7fb6-441b-a612-41c6268c6399</requestId>
    <reservationSet>
        <item>
            <reservationId>r-05534f81f74ea7036</reservationId>
            <ownerId>793614593844</ownerId>
            <groupSet/>
            <instancesSet>
                <item>
                    <instanceId>i-0e730b692d9c15460</instanceId>
                    <imageId>ami-0eb89db7593b5d434</imageId>
                    <instanceState>
                        <code>16</code>
                        <name>running</name>
                    </instanceState>
                    <privateDnsName>ip-172-31-11-152.eu-west-2.compute.internal</privateDnsName>
                    <dnsName>ec2-3-8-232-141.eu-west-2.compute.amazonaws.com</dnsName>
                    <reason/>
                    <keyName>my-laptop</keyName>
                    <amiLaunchIndex>0</amiLaunchIndex>
                    <productCodes/>
                    <instanceType>t2.micro</instanceType>
                    <launchTime>2020-04-27T09:19:26.000Z</launchTime>
                    <placement>
                        <availabilityZone>eu-west-2c</availabilityZone>
                        <groupName/>
                        <tenancy>default</tenancy>
                    </placement>
                    <monitoring>
                        <state>disabled</state>
                    </monitoring>
                    <subnetId>subnet-57044c3e</subnetId>
                    <vpcId>vpc-f1eaad99</vpcId>
                    <privateIpAddress>172.31.11.152</privateIpAddress>
                    <ipAddress>3.8.232.141</ipAddress>
                    <sourceDestCheck>true</sourceDestCheck>
                    <groupSet>
                        <item>
                            <groupId>sg-05d74e4e8551bd020</groupId>
                            <groupName>launch-wizard-1</groupName>
                        </item>
                    </groupSet>
                    <architecture>x86_64</architecture>
                    <rootDeviceType>ebs</rootDeviceType>
                    <rootDeviceName>/dev/sda1</rootDeviceName>
                    <blockDeviceMapping>
                        <item>
                            <deviceName>/dev/sda1</deviceName>
                            <ebs>
                                <volumeId>vol-0153ef24058482522</volumeId>
                                <status>attached</status>
                                <attachTime>2020-04-27T09:19:27.000Z</attachTime>
                                <deleteOnTermination>true</deleteOnTermination>
                            </ebs>
                        </item>
                    </blockDeviceMapping>
                    <virtualizationType>hvm</virtualizationType>
                    <clientToken/>
                    <tagSet>
                        <item>
                            <key>foo</key>
                            <value>bar</value>
                        </item>
                    </tagSet>
                    <hypervisor>xen</hypervisor>
                    <networkInterfaceSet>
                        <item>
                            <networkInterfaceId>eni-01d7b338ea037a60b</networkInterfaceId>
                            <subnetId>subnet-57044c3e</subnetId>
                            <vpcId>vpc-f1eaad99</vpcId>
                            <description/>
                            <ownerId>793614593844</ownerId>
                            <status>in-use</status>
                            <macAddress>02:3b:63:46:13:9a</macAddress>
                            <privateIpAddress>172.31.11.152</privateIpAddress>
                            <privateDnsName>ip-172-31-11-152.eu-west-2.compute.internal</privateDnsName>
                            <sourceDestCheck>true</sourceDestCheck>
                            <groupSet>
                                <item>
                                    <groupId>sg-05d74e4e8551bd020</groupId>
                                    <groupName>launch-wizard-1</groupName>
                                </item>
                            </groupSet>
                            <attachment>
                                <attachmentId>eni-attach-030cc2cdffe745682</attachmentId>
                                <deviceIndex>0</deviceIndex>
                                <status>attached</status>
                                <attachTime>2020-04-27T09:19:26.000Z</attachTime>
                                <deleteOnTermination>true</deleteOnTermination>
                            </attachment>
                            <association>
                                <publicIp>3.8.232.141</publicIp>
                                <publicDnsName>ec2-3-8-232-141.eu-west-2.compute.amazonaws.com</publicDnsName>
                                <ipOwnerId>amazon</ipOwnerId>
                            </association>
                            <privateIpAddressesSet>
                                <item>
                                    <privateIpAddress>172.31.11.152</privateIpAddress>
                                    <privateDnsName>ip-172-31-11-152.eu-west-2.compute.internal</privateDnsName>
                                    <primary>true</primary>
                                    <association>
                                    <publicIp>3.8.232.141</publicIp>
                                    <publicDnsName>ec2-3-8-232-141.eu-west-2.compute.amazonaws.com</publicDnsName>
                                    <ipOwnerId>amazon</ipOwnerId>
                                    </association>
                                </item>
                            </privateIpAddressesSet>
                        </item>
                    </networkInterfaceSet>
                    <ebsOptimized>false</ebsOptimized>
		    <instanceLifecycle>spot</instanceLifecycle>
		    <platform>windows</platform>
                </item>
            </instancesSet>
        </item>
    </reservationSet>
</DescribeInstancesResponse>
`
	ir, err := parseInstancesResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error when parsing data: %s", err)
	}
	irExpected := &InstancesResponse{
		ReservationSet: ReservationSet{
			Items: []Reservation{
				{
					OwnerID: "793614593844",
					InstanceSet: InstanceSet{
						Items: []Instance{
							{
								PrivateIPAddress: "172.31.11.152",
								Architecture:     "x86_64",
								Placement: Placement{
									AvailabilityZone: "eu-west-2c",
								},
								ID:        "i-0e730b692d9c15460",
								ImageID:   "ami-0eb89db7593b5d434",
								Lifecycle: "spot",
								State: InstanceState{
									Name: "running",
								},
								Type:            "t2.micro",
								Platform:        "windows",
								SubnetID:        "subnet-57044c3e",
								PrivateDNSName:  "ip-172-31-11-152.eu-west-2.compute.internal",
								PublicDNSName:   "ec2-3-8-232-141.eu-west-2.compute.amazonaws.com",
								PublicIPAddress: "3.8.232.141",
								VPCID:           "vpc-f1eaad99",
								NetworkInterfaceSet: NetworkInterfaceSet{
									Items: []NetworkInterface{
										{
											SubnetID: "subnet-57044c3e",
										},
									},
								},
								TagSet: TagSet{
									Items: []Tag{
										{
											Key:   "foo",
											Value: "bar",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if !reflect.DeepEqual(ir, irExpected) {
		t.Fatalf("unexpected InstancesResponse parsed;\ngot\n%+v\nwant\n%+v", ir, irExpected)
	}

	rs := ir.ReservationSet.Items[0]
	ownerID := rs.OwnerID
	port := 423
	inst := rs.InstanceSet.Items[0]
	labelss := inst.appendTargetLabels(nil, ownerID, "region-a", port, map[string]string{
		"eu-west-2c": "foobar-zone",
	})
	expectedLabels := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                     "172.31.11.152:423",
			"__meta_ec2_architecture":         "x86_64",
			"__meta_ec2_availability_zone":    "eu-west-2c",
			"__meta_ec2_availability_zone_id": "foobar-zone",
			"__meta_ec2_ami":                  "ami-0eb89db7593b5d434",
			"__meta_ec2_instance_id":          "i-0e730b692d9c15460",
			"__meta_ec2_instance_lifecycle":   "spot",
			"__meta_ec2_instance_state":       "running",
			"__meta_ec2_instance_type":        "t2.micro",
			"__meta_ec2_owner_id":             "793614593844",
			"__meta_ec2_platform":             "windows",
			"__meta_ec2_primary_subnet_id":    "subnet-57044c3e",
			"__meta_ec2_private_dns_name":     "ip-172-31-11-152.eu-west-2.compute.internal",
			"__meta_ec2_private_ip":           "172.31.11.152",
			"__meta_ec2_public_dns_name":      "ec2-3-8-232-141.eu-west-2.compute.amazonaws.com",
			"__meta_ec2_public_ip":            "3.8.232.141",
			"__meta_ec2_region":               "region-a",
			"__meta_ec2_subnet_id":            ",subnet-57044c3e,",
			"__meta_ec2_tag_foo":              "bar",
			"__meta_ec2_vpc_id":               "vpc-f1eaad99",
		}),
	}
	discoveryutils.TestEqualLabelss(t, labelss, expectedLabels)
}
