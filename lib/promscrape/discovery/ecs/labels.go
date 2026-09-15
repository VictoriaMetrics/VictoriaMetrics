package ecs

import (
	"context"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"golang.org/x/sync/errgroup"
)

func getInstancesLabels(ctx context.Context, cfg *apiConfig) ([]*promutil.Labels, error) {
	clusterARNs, err := listClusters(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if len(clusterARNs) == 0 {
		return nil, nil
	}

	var (
		clusters    []Cluster
		serviceARNs map[string][]string
		taskARNs    map[string][]string
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		cls, err := describeClusters(gctx, cfg, clusterARNs)
		if err != nil {
			return err
		}
		clusters = cls
		return nil
	})
	g.Go(func() error {
		arns, err := listAllServiceARNs(gctx, cfg, clusterARNs)
		if err != nil {
			return err
		}
		serviceARNs = arns
		return nil
	})
	g.Go(func() error {
		arns, err := listAllTaskARNs(gctx, cfg, clusterARNs)
		if err != nil {
			return err
		}
		taskARNs = arns
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	region := cfg.awsConfig.GetRegion()
	perCluster := make([][]*promutil.Labels, len(clusters))
	g, gctx = errgroup.WithContext(ctx)
	for i := range clusters {
		g.Go(func() error {
			ms, err := clusters[i].getTargetLabels(gctx, cfg, region, serviceARNs[clusters[i].ClusterArn], taskARNs[clusters[i].ClusterArn])
			if err != nil {
				return fmt.Errorf("cannot get targets for cluster %q: %w", clusters[i].ClusterArn, err)
			}
			perCluster[i] = ms
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	var ms []*promutil.Labels
	for _, clusterMS := range perCluster {
		ms = append(ms, clusterMS...)
	}
	return ms, nil
}

func (cl *Cluster) getTargetLabels(ctx context.Context, cfg *apiConfig, region string, serviceARNsList, taskARNsList []string) ([]*promutil.Labels, error) {
	if len(taskARNsList) == 0 {
		return nil, nil
	}

	var (
		tasks    []Task
		services []Service
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		ts, err := describeTasks(gctx, cfg, cl.ClusterArn, taskARNsList)
		if err != nil {
			return err
		}
		tasks = ts
		return nil
	})
	g.Go(func() error {
		svcs, err := describeServices(gctx, cfg, cl.ClusterArn, serviceARNsList)
		if err != nil {
			return err
		}
		services = svcs
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	var (
		containerToEC2 map[string]string
		eniPubIPs      map[string]string
	)
	g, gctx = errgroup.WithContext(ctx)
	g.Go(func() error {
		m, err := describeContainerInstances(gctx, cfg, cl.ClusterArn, tasks)
		if err != nil {
			return err
		}
		containerToEC2 = m
		return nil
	})
	g.Go(func() error {
		ips, err := describeNetworkInterfaces(gctx, cfg, tasks)
		if err != nil {
			return err
		}
		eniPubIPs = ips
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	instanceInfos, err := describeEC2Instances(ctx, cfg, containerToEC2)
	if err != nil {
		return nil, err
	}

	svcMap := make(map[string]Service, len(services))
	for _, svc := range services {
		svcMap[svc.ServiceName] = svc
	}
	var ms []*promutil.Labels
	for i := range tasks {
		ms = tasks[i].appendTargetLabels(ms, cl, svcMap, containerToEC2, instanceInfos, eniPubIPs, region, cfg.port)
	}
	return ms, nil
}

func (t *Task) appendTargetLabels(ms []*promutil.Labels, cl *Cluster, svcMap map[string]Service,
	containerToEC2 map[string]string, instanceInfos map[string]ec2InstanceInfo,
	eniPubIPs map[string]string, region string, port int) []*promutil.Labels {

	var ipAddress, subnetID, publicIP string
	var networkMode string
	var ec2ID, ec2Type, ec2PrivateIP, ec2PublicIP string

	var eniAtt *Attachment
	for i := range t.Attachments {
		if t.Attachments[i].Type == "ElasticNetworkInterface" {
			eniAtt = &t.Attachments[i]
			break
		}
	}

	if eniAtt != nil {
		networkMode = "awsvpc"
		var eniID string
		for _, d := range eniAtt.Details {
			switch d.Name {
			case "privateIPv4Address":
				ipAddress = d.Value
			case "subnetId":
				subnetID = d.Value
			case "networkInterfaceId":
				eniID = d.Value
			}
		}
		publicIP = eniPubIPs[eniID]
		// For awsvpc tasks on EC2, also collect host instance metadata.
		if t.ContainerInstanceArn != "" {
			if id, ok := containerToEC2[t.ContainerInstanceArn]; ok {
				ec2ID = id
				if info, ok := instanceInfos[id]; ok {
					ec2Type = info.instanceType
					ec2PrivateIP = info.privateIP
					ec2PublicIP = info.publicIP
				}
			}
		}
	} else if t.ContainerInstanceArn != "" {
		networkMode = "bridge"
		if id, ok := containerToEC2[t.ContainerInstanceArn]; ok {
			ec2ID = id
			if info, ok := instanceInfos[id]; ok {
				ipAddress = info.privateIP
				publicIP = info.publicIP
				subnetID = info.subnetID
				ec2Type = info.instanceType
				ec2PrivateIP = info.privateIP
				ec2PublicIP = info.publicIP
			}
		}
	}

	if ipAddress == "" {
		logger.Warnf("skipping task %q in cluster %q: cannot resolve IP address (launch type: %q, network mode: %q)",
			t.TaskArn, cl.ClusterArn, t.LaunchType, networkMode)
		return ms
	}

	m := promutil.NewLabels(32)
	m.Add("__address__", discoveryutil.JoinHostPort(ipAddress, port))
	m.Add("__meta_ecs_cluster_arn", cl.ClusterArn)
	m.Add("__meta_ecs_cluster", cl.ClusterName)
	m.Add("__meta_ecs_task_group", t.Group)
	m.Add("__meta_ecs_task_arn", t.TaskArn)
	m.Add("__meta_ecs_task_definition", t.TaskDefinitionArn)
	m.Add("__meta_ecs_ip_address", ipAddress)
	m.Add("__meta_ecs_region", region)
	m.Add("__meta_ecs_launch_type", t.LaunchType)
	m.Add("__meta_ecs_availability_zone", t.AvailabilityZone)
	m.Add("__meta_ecs_desired_status", t.DesiredStatus)
	m.Add("__meta_ecs_last_status", t.LastStatus)
	m.Add("__meta_ecs_health_status", t.HealthStatus)
	m.Add("__meta_ecs_network_mode", networkMode)
	if subnetID != "" {
		m.Add("__meta_ecs_subnet_id", subnetID)
	}
	if publicIP != "" {
		m.Add("__meta_ecs_public_ip", publicIP)
	}
	if t.ContainerInstanceArn != "" {
		m.Add("__meta_ecs_container_instance_arn", t.ContainerInstanceArn)
	}
	if ec2ID != "" {
		m.Add("__meta_ecs_ec2_instance_id", ec2ID)
	}
	if ec2Type != "" {
		m.Add("__meta_ecs_ec2_instance_type", ec2Type)
	}
	if ec2PrivateIP != "" {
		m.Add("__meta_ecs_ec2_instance_private_ip", ec2PrivateIP)
	}
	if ec2PublicIP != "" {
		m.Add("__meta_ecs_ec2_instance_public_ip", ec2PublicIP)
	}
	if t.PlatformFamily != "" {
		m.Add("__meta_ecs_platform_family", t.PlatformFamily)
	}
	if t.PlatformVersion != "" {
		m.Add("__meta_ecs_platform_version", t.PlatformVersion)
	}
	for _, tag := range cl.Tags {
		if tag.Key != "" && tag.Value != "" {
			m.Add(discoveryutil.SanitizeLabelName("__meta_ecs_tag_cluster_"+tag.Key), tag.Value)
		}
	}
	if !strings.HasPrefix(t.Group, "family:") {
		_, svcName, _ := strings.Cut(t.Group, ":")
		if svc, ok := svcMap[svcName]; ok {
			svc.appendLabels(m)
		}
	}
	for _, tag := range t.Tags {
		if tag.Key != "" && tag.Value != "" {
			m.Add(discoveryutil.SanitizeLabelName("__meta_ecs_tag_task_"+tag.Key), tag.Value)
		}
	}
	if ec2ID != "" {
		if info, ok := instanceInfos[ec2ID]; ok {
			for k, v := range info.tags {
				m.Add(discoveryutil.SanitizeLabelName("__meta_ecs_tag_ec2_"+k), v)
			}
		}
	}
	return append(ms, m)
}
