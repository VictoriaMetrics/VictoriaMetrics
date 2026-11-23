package ec2

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/awsapi"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// getInstancesLabels returns labels for ec2 instances obtained from the given cfg
func getInstancesLabels(cfg *apiConfig) ([]*promutil.Labels, error) {
	rs, err := getReservations(cfg)
	if err != nil {
		return nil, err
	}
	azMap := getAZMap(cfg)
	region := cfg.awsConfig.GetRegion()
	var ms []*promutil.Labels
	for _, r := range rs {
		for _, inst := range r.InstanceSet.Items {
			ms = inst.appendTargetLabels(ms, r.OwnerID, region, cfg.port, azMap)
		}
	}
	return ms, nil
}

func getReservations(cfg *apiConfig) ([]Reservation, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
	var rs []Reservation
	pageToken := ""
	instanceFilters := awsapi.GetFiltersQueryString(cfg.instanceFilters)
	for {
		data, err := cfg.awsConfig.GetEC2APIResponse("DescribeInstances", instanceFilters, pageToken)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain instances: %w", err)
		}
		ir, err := parseInstancesResponse(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse instance list: %w", err)
		}
		rs = append(rs, ir.ReservationSet.Items...)
		if len(ir.NextPageToken) == 0 {
			return rs, nil
		}
		pageToken = ir.NextPageToken
	}
}

// InstancesResponse represents response to https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
type InstancesResponse struct {
	ReservationSet ReservationSet `xml:"reservationSet"`
	NextPageToken  string         `xml:"nextToken"`
}

// ReservationSet represents ReservationSet from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
type ReservationSet struct {
	Items []Reservation `xml:"item"`
}

// Reservation represents Reservation from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Reservation.html
type Reservation struct {
	OwnerID     string      `xml:"ownerId"`
	InstanceSet InstanceSet `xml:"instancesSet"`
}

// InstanceSet represents InstanceSet from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Reservation.html
type InstanceSet struct {
	Items []Instance `xml:"item"`
}

// Instance represents Instance from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html
type Instance struct {
	PrivateIPAddress    string              `xml:"privateIpAddress"`
	Architecture        string              `xml:"architecture"`
	Placement           Placement           `xml:"placement"`
	ImageID             string              `xml:"imageId"`
	ID                  string              `xml:"instanceId"`
	Lifecycle           string              `xml:"instanceLifecycle"`
	State               InstanceState       `xml:"instanceState"`
	Type                string              `xml:"instanceType"`
	Platform            string              `xml:"platform"`
	SubnetID            string              `xml:"subnetId"`
	PrivateDNSName      string              `xml:"privateDnsName"`
	PublicDNSName       string              `xml:"dnsName"`
	PublicIPAddress     string              `xml:"ipAddress"`
	VPCID               string              `xml:"vpcId"`
	NetworkInterfaceSet NetworkInterfaceSet `xml:"networkInterfaceSet"`
	TagSet              TagSet              `xml:"tagSet"`
}

// Placement represents Placement from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Placement.html
type Placement struct {
	AvailabilityZone string `xml:"availabilityZone"`
}

// InstanceState represents InstanceState from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceState.html
type InstanceState struct {
	Name string `xml:"name"`
}

// NetworkInterfaceSet represents NetworkInterfaceSet from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html
type NetworkInterfaceSet struct {
	Items []NetworkInterface `xml:"item"`
}

// NetworkInterface represents NetworkInterface from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceNetworkInterface.html
type NetworkInterface struct {
	SubnetID         string                      `xml:"subnetId"`
	Attachment       *NetworkInterfaceAttachment `xml:"attachment"`
	IPv6AddressesSet Ipv6AddressesSet            `xml:"ipv6AddressesSet"`
}

// NetworkInterfaceAttachment represents InstanceNetworkInterfaceAttachment from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceNetworkInterfaceAttachment.html
type NetworkInterfaceAttachment struct {
	DeviceIndex *int `xml:"deviceIndex"`
}

// Ipv6AddressesSet represents ipv6AddressesSet from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceNetworkInterface.html
type Ipv6AddressesSet struct {
	Items []InstanceIPv6Address `xml:"item"`
}

// InstanceIPv6Address represents InstanceIpv6Address from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceIpv6Address.html
type InstanceIPv6Address struct {
	IPv6Address   string `xml:"ipv6Address"`
	IsPrimaryIPv6 bool   `xml:"isPrimaryIpv6"`
}

// TagSet represents TagSet from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html
type TagSet struct {
	Items []Tag `xml:"item"`
}

// Tag represents Tag from https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Tag.html
type Tag struct {
	Key   string `xml:"key"`
	Value string `xml:"value"`
}

func parseInstancesResponse(data []byte) (*InstancesResponse, error) {
	var v InstancesResponse
	if err := xml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal InstancesResponse from %q: %w", data, err)
	}
	return &v, nil
}

func (inst *Instance) appendTargetLabels(ms []*promutil.Labels, ownerID, region string, port int, azMap map[string]string) []*promutil.Labels {
	defaultIPv6Addr, primaryIPv6Addrs, ipv6Addrs := inst.getIPv6Addresses()
	host := inst.PrivateIPAddress
	if len(host) == 0 {
		// IPv6-only instances have no private IPv4 address.
		host = defaultIPv6Addr
	}
	if len(host) == 0 {
		// Cannot scrape instance without private IPv4 or IPv6 address
		return ms
	}
	addr := discoveryutil.JoinHostPort(host, port)
	m := promutil.NewLabels(24)
	m.Add("__address__", addr)
	m.Add("__meta_ec2_architecture", inst.Architecture)
	m.Add("__meta_ec2_ami", inst.ImageID)
	m.Add("__meta_ec2_availability_zone", inst.Placement.AvailabilityZone)
	m.Add("__meta_ec2_availability_zone_id", azMap[inst.Placement.AvailabilityZone])
	if len(defaultIPv6Addr) > 0 {
		m.Add("__meta_ec2_default_ipv6_address", defaultIPv6Addr)
	}
	m.Add("__meta_ec2_instance_id", inst.ID)
	m.Add("__meta_ec2_instance_lifecycle", inst.Lifecycle)
	m.Add("__meta_ec2_instance_state", inst.State.Name)
	m.Add("__meta_ec2_instance_type", inst.Type)
	m.Add("__meta_ec2_owner_id", ownerID)
	m.Add("__meta_ec2_platform", inst.Platform)
	m.Add("__meta_ec2_primary_subnet_id", inst.SubnetID)
	m.Add("__meta_ec2_private_dns_name", inst.PrivateDNSName)
	m.Add("__meta_ec2_private_ip", inst.PrivateIPAddress)
	m.Add("__meta_ec2_public_dns_name", inst.PublicDNSName)
	m.Add("__meta_ec2_public_ip", inst.PublicIPAddress)
	m.Add("__meta_ec2_region", region)
	m.Add("__meta_ec2_vpc_id", inst.VPCID)
	if len(inst.VPCID) > 0 {
		subnets := make([]string, 0, len(inst.NetworkInterfaceSet.Items))
		seenSubnets := make(map[string]bool, len(inst.NetworkInterfaceSet.Items))
		for _, ni := range inst.NetworkInterfaceSet.Items {
			if len(ni.SubnetID) == 0 {
				continue
			}
			// Deduplicate VPC Subnet IDs maintaining the order of the network interfaces returned by EC2.
			if !seenSubnets[ni.SubnetID] {
				seenSubnets[ni.SubnetID] = true
				subnets = append(subnets, ni.SubnetID)
			}
		}
		// We surround the separated list with the separator as well. This way regular expressions
		// in relabeling rules don't have to consider tag positions.
		m.Add("__meta_ec2_subnet_id", ","+strings.Join(subnets, ",")+",")
		if len(primaryIPv6Addrs) > 0 {
			m.Add("__meta_ec2_primary_ipv6_addresses", ","+strings.Join(primaryIPv6Addrs, ",")+",")
		}
		if len(ipv6Addrs) > 0 {
			m.Add("__meta_ec2_ipv6_addresses", ","+strings.Join(ipv6Addrs, ",")+",")
		}
	}
	for _, t := range inst.TagSet.Items {
		if len(t.Key) == 0 || len(t.Value) == 0 {
			continue
		}
		m.Add(discoveryutil.SanitizeLabelName("__meta_ec2_tag_"+t.Key), t.Value)
	}
	ms = append(ms, m)
	return ms
}

// getIPv6Addresses returns the IPv6 addresses of the instance network interfaces in the same way as Prometheus does.
//
// primaryIPv6Addrs contains the primary IPv6 address of every network interface at its device index,
// with empty strings for network interfaces without a primary IPv6 address.
// defaultIPv6Addr is the first primary IPv6 address if any, otherwise the first IPv6 address.
func (inst *Instance) getIPv6Addresses() (defaultIPv6Addr string, primaryIPv6Addrs, ipv6Addrs []string) {
	if len(inst.VPCID) == 0 {
		return "", nil, nil
	}
	for _, ni := range inst.NetworkInterfaceSet.Items {
		if len(ni.SubnetID) == 0 {
			continue
		}
		for _, a := range ni.IPv6AddressesSet.Items {
			if len(a.IPv6Address) == 0 {
				continue
			}
			ipv6Addrs = append(ipv6Addrs, a.IPv6Address)
			if !a.IsPrimaryIPv6 || ni.Attachment == nil || ni.Attachment.DeviceIndex == nil || *ni.Attachment.DeviceIndex < 0 {
				continue
			}
			idx := *ni.Attachment.DeviceIndex
			for len(primaryIPv6Addrs) <= idx {
				primaryIPv6Addrs = append(primaryIPv6Addrs, "")
			}
			primaryIPv6Addrs[idx] = a.IPv6Address
		}
	}
	for _, addr := range primaryIPv6Addrs {
		if len(addr) > 0 {
			return addr, primaryIPv6Addrs, ipv6Addrs
		}
	}
	if len(ipv6Addrs) > 0 {
		return ipv6Addrs[0], primaryIPv6Addrs, ipv6Addrs
	}
	return "", primaryIPv6Addrs, ipv6Addrs
}
