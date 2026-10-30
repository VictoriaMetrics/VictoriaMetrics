package ecs

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestParseListClustersResponseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		resp, err := parseListClustersResponse([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if resp != nil {
			t.Fatalf("unexpected non-nil response: %v", resp)
		}
	}
	f(``)
	f(`[1,2,3]`)
	f(`{"clusterArns": "not-an-array"}`)
}

func TestParseListClustersResponseSuccess(t *testing.T) {
	data := `{"clusterArns":["arn:aws:ecs:us-east-1:123456789012:cluster/prod","arn:aws:ecs:us-east-1:123456789012:cluster/staging"],"nextToken":"tok1"}`
	resp, err := parseListClustersResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(resp.ClusterArns) != 2 {
		t.Fatalf("unexpected number of cluster ARNs; got %d; want 2", len(resp.ClusterArns))
	}
	if resp.NextToken != "tok1" {
		t.Fatalf("unexpected NextToken; got %q; want %q", resp.NextToken, "tok1")
	}
}

func TestParseDescribeClustersResponseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		resp, err := parseDescribeClustersResponse([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if resp != nil {
			t.Fatalf("unexpected non-nil response: %v", resp)
		}
	}
	f(``)
	f(`[1,2,3]`)
	f(`{"clusters": "not-an-array"}`)
}

func TestParseDescribeClustersResponseSuccess(t *testing.T) {
	data := `{
		"clusters": [
			{
				"clusterArn": "arn:aws:ecs:us-east-1:123456789012:cluster/prod",
				"clusterName": "prod",
				"tags": [{"key": "env", "value": "production"}]
			}
		]
	}`
	resp, err := parseDescribeClustersResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(resp.Clusters) != 1 {
		t.Fatalf("unexpected number of clusters; got %d; want 1", len(resp.Clusters))
	}
	cl := resp.Clusters[0]
	if cl.ClusterArn != "arn:aws:ecs:us-east-1:123456789012:cluster/prod" {
		t.Fatalf("unexpected ClusterArn; got %q", cl.ClusterArn)
	}
	if cl.ClusterName != "prod" {
		t.Fatalf("unexpected ClusterName; got %q", cl.ClusterName)
	}
	if len(cl.Tags) != 1 || cl.Tags[0].Key != "env" || cl.Tags[0].Value != "production" {
		t.Fatalf("unexpected Tags: %v", cl.Tags)
	}
}

func TestParseListTasksResponseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		resp, err := parseListTasksResponse([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if resp != nil {
			t.Fatalf("unexpected non-nil response: %v", resp)
		}
	}
	f(``)
	f(`[1,2,3]`)
	f(`{"taskArns": "not-an-array"}`)
}

func TestParseListTasksResponseSuccess(t *testing.T) {
	data := `{"taskArns":["arn:aws:ecs:us-east-1:123456789012:task/abc","arn:aws:ecs:us-east-1:123456789012:task/def"],"nextToken":"tok2"}`
	resp, err := parseListTasksResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(resp.TaskArns) != 2 {
		t.Fatalf("unexpected number of task ARNs; got %d; want 2", len(resp.TaskArns))
	}
	if resp.NextToken != "tok2" {
		t.Fatalf("unexpected NextToken; got %q; want %q", resp.NextToken, "tok2")
	}
}

func TestParseDescribeTasksResponseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		resp, err := parseDescribeTasksResponse([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if resp != nil {
			t.Fatalf("unexpected non-nil response: %v", resp)
		}
	}
	f(``)
	f(`[1,2,3]`)
	f(`{"tasks": "not-an-array"}`)
}

func TestParseDescribeTasksResponseSuccess(t *testing.T) {
	data := `{
		"tasks": [
			{
				"taskArn": "arn:aws:ecs:us-east-1:123456789012:task/abc",
				"taskDefinitionArn": "arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:1",
				"group": "service:my-service",
				"launchType": "FARGATE",
				"availabilityZone": "us-east-1a",
				"desiredStatus": "RUNNING",
				"lastStatus": "RUNNING",
				"healthStatus": "HEALTHY",
				"platformFamily": "Linux",
				"platformVersion": "1.4.0",
				"attachments": [
					{
						"type": "ElasticNetworkInterface",
						"details": [
							{"name": "privateIPv4Address", "value": "10.0.0.1"},
							{"name": "subnetId", "value": "subnet-abc"},
							{"name": "networkInterfaceId", "value": "eni-abc"}
						]
					}
				],
				"tags": [{"key": "version", "value": "v1"}]
			}
		]
	}`
	resp, err := parseDescribeTasksResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(resp.Tasks) != 1 {
		t.Fatalf("unexpected number of tasks; got %d; want 1", len(resp.Tasks))
	}
	task := resp.Tasks[0]

	cl := &Cluster{
		ClusterArn:  "arn:aws:ecs:us-east-1:123456789012:cluster/prod",
		ClusterName: "prod",
		Tags:        []Tag{{Key: "env", Value: "production"}},
	}
	svcMap := map[string]Service{
		"my-service": {
			ServiceArn:  "arn:aws:ecs:us-east-1:123456789012:service/prod/my-service",
			ServiceName: "my-service",
			Status:      "ACTIVE",
			Tags:        []Tag{{Key: "team", Value: "backend"}},
		},
	}
	eniPubIPs := map[string]string{"eni-abc": "54.0.0.1"}
	labelss := task.appendTargetLabels(nil, cl, svcMap, nil, nil, eniPubIPs, "us-east-1", 80)
	expectedLabelss := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                  "10.0.0.1:80",
			"__meta_ecs_cluster_arn":       "arn:aws:ecs:us-east-1:123456789012:cluster/prod",
			"__meta_ecs_cluster":           "prod",
			"__meta_ecs_task_group":        "service:my-service",
			"__meta_ecs_task_arn":          "arn:aws:ecs:us-east-1:123456789012:task/abc",
			"__meta_ecs_task_definition":   "arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:1",
			"__meta_ecs_ip_address":        "10.0.0.1",
			"__meta_ecs_region":            "us-east-1",
			"__meta_ecs_launch_type":       "FARGATE",
			"__meta_ecs_availability_zone": "us-east-1a",
			"__meta_ecs_desired_status":    "RUNNING",
			"__meta_ecs_last_status":       "RUNNING",
			"__meta_ecs_health_status":     "HEALTHY",
			"__meta_ecs_network_mode":      "awsvpc",
			"__meta_ecs_subnet_id":         "subnet-abc",
			"__meta_ecs_public_ip":         "54.0.0.1",
			"__meta_ecs_platform_family":   "Linux",
			"__meta_ecs_platform_version":  "1.4.0",
			"__meta_ecs_tag_cluster_env":   "production",
			"__meta_ecs_service":           "my-service",
			"__meta_ecs_service_arn":       "arn:aws:ecs:us-east-1:123456789012:service/prod/my-service",
			"__meta_ecs_service_status":    "ACTIVE",
			"__meta_ecs_tag_service_team":  "backend",
			"__meta_ecs_tag_task_version":  "v1",
		}),
	}
	discoveryutil.TestEqualLabelss(t, labelss, expectedLabelss)
}

func TestParseListServicesResponseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		resp, err := parseListServicesResponse([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if resp != nil {
			t.Fatalf("unexpected non-nil response: %v", resp)
		}
	}
	f(``)
	f(`[1,2,3]`)
	f(`{"serviceArns": "not-an-array"}`)
}

func TestParseListServicesResponseSuccess(t *testing.T) {
	data := `{"serviceArns":["arn:aws:ecs:us-east-1:123456789012:service/prod/my-service"]}`
	resp, err := parseListServicesResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(resp.ServiceArns) != 1 {
		t.Fatalf("unexpected number of service ARNs; got %d; want 1", len(resp.ServiceArns))
	}
}

func TestParseDescribeServicesResponseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		resp, err := parseDescribeServicesResponse([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if resp != nil {
			t.Fatalf("unexpected non-nil response: %v", resp)
		}
	}
	f(``)
	f(`[1,2,3]`)
	f(`{"services": "not-an-array"}`)
}

func TestParseDescribeServicesResponseSuccess(t *testing.T) {
	data := `{
		"services": [
			{
				"serviceArn": "arn:aws:ecs:us-east-1:123456789012:service/prod/my-service",
				"serviceName": "my-service",
				"status": "ACTIVE",
				"tags": [{"key": "team", "value": "backend"}]
			}
		]
	}`
	resp, err := parseDescribeServicesResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(resp.Services) != 1 {
		t.Fatalf("unexpected number of services; got %d; want 1", len(resp.Services))
	}
	svc := resp.Services[0]
	if svc.ServiceName != "my-service" {
		t.Fatalf("unexpected ServiceName; got %q", svc.ServiceName)
	}
	if svc.Status != "ACTIVE" {
		t.Fatalf("unexpected Status; got %q", svc.Status)
	}
}

func TestParseDescribeContainerInstancesResponseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		resp, err := parseDescribeContainerInstancesResponse([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if resp != nil {
			t.Fatalf("unexpected non-nil response: %v", resp)
		}
	}
	f(``)
	f(`[1,2,3]`)
	f(`{"containerInstances": "not-an-array"}`)
}

func TestParseDescribeContainerInstancesResponseSuccess(t *testing.T) {
	data := `{
		"containerInstances": [
			{
				"containerInstanceArn": "arn:aws:ecs:us-east-1:123456789012:container-instance/ci-abc",
				"ec2InstanceId": "i-0abcdef1234567890"
			}
		]
	}`
	resp, err := parseDescribeContainerInstancesResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(resp.ContainerInstances) != 1 {
		t.Fatalf("unexpected number of container instances; got %d; want 1", len(resp.ContainerInstances))
	}
	ci := resp.ContainerInstances[0]
	if ci.ContainerInstanceArn != "arn:aws:ecs:us-east-1:123456789012:container-instance/ci-abc" {
		t.Fatalf("unexpected ContainerInstanceArn; got %q", ci.ContainerInstanceArn)
	}
	if ci.Ec2InstanceId != "i-0abcdef1234567890" {
		t.Fatalf("unexpected Ec2InstanceId; got %q", ci.Ec2InstanceId)
	}
}

func TestParseEC2InstancesResponseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		resp, err := parseEC2InstancesResponse([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if resp != nil {
			t.Fatalf("unexpected non-nil response: %v", resp)
		}
	}
	f(``)
	f(`{"not": "xml"}`)
}

func TestParseEC2InstancesResponseSuccess(t *testing.T) {
	data := `<?xml version="1.0" encoding="UTF-8"?>
<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
    <reservationSet>
        <item>
            <instancesSet>
                <item>
                    <instanceId>i-0abcdef1234567890</instanceId>
                    <privateIpAddress>172.31.10.1</privateIpAddress>
                    <ipAddress>54.0.0.1</ipAddress>
                    <subnetId>subnet-abc123</subnetId>
                    <instanceType>m5.large</instanceType>
                    <tagSet>
                        <item>
                            <key>Name</key>
                            <value>my-host</value>
                        </item>
                    </tagSet>
                </item>
            </instancesSet>
        </item>
    </reservationSet>
</DescribeInstancesResponse>`
	resp, err := parseEC2InstancesResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(resp.ReservationSet.Items) != 1 {
		t.Fatalf("unexpected number of reservations; got %d; want 1", len(resp.ReservationSet.Items))
	}
	inst := resp.ReservationSet.Items[0].InstancesSet.Items[0]
	if inst.InstanceId != "i-0abcdef1234567890" {
		t.Fatalf("unexpected InstanceId; got %q", inst.InstanceId)
	}
	if inst.PrivateIpAddress != "172.31.10.1" {
		t.Fatalf("unexpected PrivateIpAddress; got %q", inst.PrivateIpAddress)
	}
	if inst.PublicIpAddress != "54.0.0.1" {
		t.Fatalf("unexpected PublicIpAddress; got %q", inst.PublicIpAddress)
	}
	if inst.SubnetId != "subnet-abc123" {
		t.Fatalf("unexpected SubnetId; got %q", inst.SubnetId)
	}
	if inst.InstanceType != "m5.large" {
		t.Fatalf("unexpected InstanceType; got %q", inst.InstanceType)
	}
	if len(inst.TagSet.Items) != 1 || inst.TagSet.Items[0].Key != "Name" || inst.TagSet.Items[0].Value != "my-host" {
		t.Fatalf("unexpected TagSet: %+v", inst.TagSet)
	}
}

func TestParseEC2NetworkInterfacesResponseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		resp, err := parseEC2NetworkInterfacesResponse([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if resp != nil {
			t.Fatalf("unexpected non-nil response: %v", resp)
		}
	}
	f(``)
	f(`{"not": "xml"}`)
}

func TestParseEC2NetworkInterfacesResponseSuccess(t *testing.T) {
	data := `<?xml version="1.0" encoding="UTF-8"?>
<DescribeNetworkInterfacesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
    <networkInterfaceSet>
        <item>
            <networkInterfaceId>eni-abc</networkInterfaceId>
            <association>
                <publicIp>54.1.2.3</publicIp>
            </association>
        </item>
    </networkInterfaceSet>
</DescribeNetworkInterfacesResponse>`
	resp, err := parseEC2NetworkInterfacesResponse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(resp.NetworkInterfaceSet.Items) != 1 {
		t.Fatalf("unexpected number of ENIs; got %d; want 1", len(resp.NetworkInterfaceSet.Items))
	}
	eni := resp.NetworkInterfaceSet.Items[0]
	if eni.NetworkInterfaceId != "eni-abc" {
		t.Fatalf("unexpected NetworkInterfaceId; got %q", eni.NetworkInterfaceId)
	}
	if eni.Association.PublicIp != "54.1.2.3" {
		t.Fatalf("unexpected PublicIp; got %q", eni.Association.PublicIp)
	}
}

func TestTaskAppendTargetLabelsEC2Bridge(t *testing.T) {
	ciARN := "arn:aws:ecs:us-east-1:123456789012:container-instance/ci-abc"
	task := &Task{
		TaskArn:              "arn:aws:ecs:us-east-1:123456789012:task/def",
		TaskDefinitionArn:    "arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:2",
		Group:                "family:my-task",
		LaunchType:           "EC2",
		AvailabilityZone:     "us-east-1b",
		DesiredStatus:        "RUNNING",
		LastStatus:           "RUNNING",
		HealthStatus:         "UNKNOWN",
		ContainerInstanceArn: ciARN,
	}
	cl := &Cluster{
		ClusterArn:  "arn:aws:ecs:us-east-1:123456789012:cluster/prod",
		ClusterName: "prod",
		Tags:        []Tag{{Key: "env", Value: "production"}},
	}
	containerToEC2 := map[string]string{ciARN: "i-0abcdef1234567890"}
	instanceInfos := map[string]ec2InstanceInfo{
		"i-0abcdef1234567890": {
			privateIP:    "172.31.10.1",
			publicIP:     "54.0.0.1",
			subnetID:     "subnet-abc123",
			instanceType: "m5.large",
			tags:         map[string]string{"Name": "my-host"},
		},
	}
	labelss := task.appendTargetLabels(nil, cl, nil, containerToEC2, instanceInfos, nil, "us-east-1", 80)
	expectedLabelss := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                        "172.31.10.1:80",
			"__meta_ecs_cluster_arn":             "arn:aws:ecs:us-east-1:123456789012:cluster/prod",
			"__meta_ecs_cluster":                 "prod",
			"__meta_ecs_task_group":              "family:my-task",
			"__meta_ecs_task_arn":                "arn:aws:ecs:us-east-1:123456789012:task/def",
			"__meta_ecs_task_definition":         "arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:2",
			"__meta_ecs_ip_address":              "172.31.10.1",
			"__meta_ecs_region":                  "us-east-1",
			"__meta_ecs_launch_type":             "EC2",
			"__meta_ecs_availability_zone":       "us-east-1b",
			"__meta_ecs_desired_status":          "RUNNING",
			"__meta_ecs_last_status":             "RUNNING",
			"__meta_ecs_health_status":           "UNKNOWN",
			"__meta_ecs_network_mode":            "bridge",
			"__meta_ecs_subnet_id":               "subnet-abc123",
			"__meta_ecs_public_ip":               "54.0.0.1",
			"__meta_ecs_container_instance_arn":  ciARN,
			"__meta_ecs_ec2_instance_id":         "i-0abcdef1234567890",
			"__meta_ecs_ec2_instance_type":       "m5.large",
			"__meta_ecs_ec2_instance_private_ip": "172.31.10.1",
			"__meta_ecs_ec2_instance_public_ip":  "54.0.0.1",
			"__meta_ecs_tag_cluster_env":         "production",
			"__meta_ecs_tag_ec2_Name":            "my-host",
		}),
	}
	discoveryutil.TestEqualLabelss(t, labelss, expectedLabelss)
}

func TestTaskAppendTargetLabelsNoIP(t *testing.T) {
	// Task with no ENI and no ContainerInstanceArn cannot resolve an IP and must be skipped.
	task := &Task{
		TaskArn:           "arn:aws:ecs:us-east-1:123456789012:task/ghi",
		TaskDefinitionArn: "arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:3",
		Group:             "family:my-task",
		LaunchType:        "EXTERNAL",
		DesiredStatus:     "RUNNING",
		LastStatus:        "RUNNING",
		HealthStatus:      "UNKNOWN",
	}
	cl := &Cluster{
		ClusterArn:  "arn:aws:ecs:us-east-1:123456789012:cluster/prod",
		ClusterName: "prod",
	}
	labelss := task.appendTargetLabels(nil, cl, nil, nil, nil, nil, "us-east-1", 80)
	if len(labelss) != 0 {
		t.Fatalf("unexpected non-empty labelss for task without IP; got %d entries", len(labelss))
	}
}
