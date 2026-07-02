package ecs

import (
	"context"
	"encoding/xml"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/set"
)

func describeEC2Instances(ctx context.Context, cfg *apiConfig, containerToEC2 map[string]string) (map[string]ec2InstanceInfo, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
	if len(containerToEC2) == 0 {
		return nil, nil
	}
	ids := make(set.Ordered[string])
	for _, id := range containerToEC2 {
		ids.Add(id)
	}
	instanceIDs := ids.Items()
	filtersQS := buildIDFilterQueryString("InstanceId", instanceIDs)
	result := make(map[string]ec2InstanceInfo)
	pageToken := ""
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := cfg.awsConfig.GetEC2APIResponse("DescribeInstances", filtersQS, pageToken)
		if err != nil {
			return nil, fmt.Errorf("cannot describe EC2 instances: %w", err)
		}
		resp, err := parseEC2InstancesResponse(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse EC2 instances: %w", err)
		}
		for _, r := range resp.ReservationSet.Items {
			for _, inst := range r.InstancesSet.Items {
				if inst.InstanceId == "" || inst.PrivateIpAddress == "" {
					continue
				}
				info := ec2InstanceInfo{
					privateIP:    inst.PrivateIpAddress,
					publicIP:     inst.PublicIpAddress,
					subnetID:     inst.SubnetId,
					instanceType: inst.InstanceType,
					tags:         make(map[string]string, len(inst.TagSet.Items)),
				}
				for _, tag := range inst.TagSet.Items {
					if tag.Key != "" && tag.Value != "" {
						info.tags[tag.Key] = tag.Value
					}
				}
				result[inst.InstanceId] = info
			}
		}
		if resp.NextToken == "" {
			return result, nil
		}
		pageToken = resp.NextToken
	}
}

func describeNetworkInterfaces(ctx context.Context, cfg *apiConfig, tasks []Task) (map[string]string, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeNetworkInterfaces.html
	var eniIDs []string
	for i := range tasks {
		for j := range tasks[i].Attachments {
			att := &tasks[i].Attachments[j]
			if att.Type != "ElasticNetworkInterface" {
				continue
			}
			for _, d := range att.Details {
				if d.Name == "networkInterfaceId" && d.Value != "" {
					eniIDs = append(eniIDs, d.Value)
					break
				}
			}
			break
		}
	}
	if len(eniIDs) == 0 {
		return nil, nil
	}
	filtersQS := buildIDFilterQueryString("NetworkInterfaceId", eniIDs)
	result := make(map[string]string)
	pageToken := ""
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := cfg.awsConfig.GetEC2APIResponse("DescribeNetworkInterfaces", filtersQS, pageToken)
		if err != nil {
			return nil, fmt.Errorf("cannot describe network interfaces: %w", err)
		}
		resp, err := parseEC2NetworkInterfacesResponse(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse network interfaces: %w", err)
		}
		for _, eni := range resp.NetworkInterfaceSet.Items {
			if eni.NetworkInterfaceId != "" && eni.Association.PublicIp != "" {
				result[eni.NetworkInterfaceId] = eni.Association.PublicIp
			}
		}
		if resp.NextToken == "" {
			return result, nil
		}
		pageToken = resp.NextToken
	}
}

type ec2InstanceInfo struct {
	privateIP    string
	publicIP     string
	subnetID     string
	instanceType string
	tags         map[string]string
}

// EC2InstancesResponse represents response to https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
type EC2InstancesResponse struct {
	ReservationSet struct {
		Items []struct {
			InstancesSet struct {
				Items []struct {
					InstanceId       string `xml:"instanceId"`
					PrivateIpAddress string `xml:"privateIpAddress"`
					PublicIpAddress  string `xml:"ipAddress"`
					SubnetId         string `xml:"subnetId"`
					InstanceType     string `xml:"instanceType"`
					TagSet           struct {
						Items []struct {
							Key   string `xml:"key"`
							Value string `xml:"value"`
						} `xml:"item"`
					} `xml:"tagSet"`
				} `xml:"item"`
			} `xml:"instancesSet"`
		} `xml:"item"`
	} `xml:"reservationSet"`
	NextToken string `xml:"nextToken"`
}

func parseEC2InstancesResponse(data []byte) (*EC2InstancesResponse, error) {
	var v EC2InstancesResponse
	if err := xml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal DescribeInstances response from %q: %w", data, err)
	}
	return &v, nil
}

// EC2NetworkInterfacesResponse represents response to https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeNetworkInterfaces.html
type EC2NetworkInterfacesResponse struct {
	NetworkInterfaceSet struct {
		Items []struct {
			NetworkInterfaceId string `xml:"networkInterfaceId"`
			Association        struct {
				PublicIp string `xml:"publicIp"`
			} `xml:"association"`
		} `xml:"item"`
	} `xml:"networkInterfaceSet"`
	NextToken string `xml:"nextToken"`
}

func parseEC2NetworkInterfacesResponse(data []byte) (*EC2NetworkInterfacesResponse, error) {
	var v EC2NetworkInterfacesResponse
	if err := xml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal DescribeNetworkInterfaces response from %q: %w", data, err)
	}
	return &v, nil
}
