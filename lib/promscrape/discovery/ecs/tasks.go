package ecs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/set"
)

func listAllTaskARNs(ctx context.Context, cfg *apiConfig, clusterARNs []string) (map[string][]string, error) {
	var mu sync.Mutex
	result := make(map[string][]string, len(clusterARNs))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.requestConcurrency)
	for _, clusterARN := range clusterARNs {
		g.Go(func() error {
			arns, err := listTasks(gctx, cfg, clusterARN)
			if err != nil {
				return err
			}
			mu.Lock()
			result[clusterARN] = arns
			mu.Unlock()
			return nil
		})
	}
	return result, g.Wait()
}

func listTasks(ctx context.Context, cfg *apiConfig, clusterARN string) ([]string, error) {
	// See https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListTasks.html
	var arns []string
	nextToken := ""
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := cfg.awsConfig.GetECSAPIResponse(cfg.ecsEndpoint, "ListTasks", buildListBody(clusterARN, nextToken))
		if err != nil {
			return nil, fmt.Errorf("cannot list tasks for cluster %q: %w", clusterARN, err)
		}
		resp, err := parseListTasksResponse(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse task list for cluster %q: %w", clusterARN, err)
		}
		arns = append(arns, resp.TaskArns...)
		if resp.NextToken == "" {
			return arns, nil
		}
		nextToken = resp.NextToken
	}
}

// ListTasksResponse represents response to https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListTasks.html
type ListTasksResponse struct {
	TaskArns  []string `json:"taskArns"`
	NextToken string   `json:"nextToken"`
}

func parseListTasksResponse(data []byte) (*ListTasksResponse, error) {
	var v ListTasksResponse
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal ListTasks response from %q: %w", data, err)
	}
	return &v, nil
}

func describeTasks(ctx context.Context, cfg *apiConfig, clusterARN string, arns []string) ([]Task, error) {
	// See https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeTasks.html
	// DescribeTasks allows up to 100 tasks per call.
	var mu sync.Mutex
	var tasks []Task
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.requestConcurrency)
	for i := 0; i < len(arns); i += 100 {
		batch := arns[i:min(i+100, len(arns))]
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			data, err := cfg.awsConfig.GetECSAPIResponse(cfg.ecsEndpoint, "DescribeTasks", buildDescribeBody(clusterARN, true, "tasks", batch))
			if err != nil {
				return fmt.Errorf("cannot describe tasks for cluster %q: %w", clusterARN, err)
			}
			resp, err := parseDescribeTasksResponse(data)
			if err != nil {
				return fmt.Errorf("cannot parse tasks for cluster %q: %w", clusterARN, err)
			}
			mu.Lock()
			tasks = append(tasks, resp.Tasks...)
			mu.Unlock()
			return nil
		})
	}
	return tasks, g.Wait()
}

// DescribeTasksResponse represents response to https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeTasks.html
type DescribeTasksResponse struct {
	Tasks []Task `json:"tasks"`
}

func parseDescribeTasksResponse(data []byte) (*DescribeTasksResponse, error) {
	var v DescribeTasksResponse
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal DescribeTasks response from %q: %w", data, err)
	}
	return &v, nil
}

func describeContainerInstances(ctx context.Context, cfg *apiConfig, clusterARN string, tasks []Task) (map[string]string, error) {
	// See https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeContainerInstances.html
	arnSet := make(set.Ordered[string])
	for _, t := range tasks {
		if t.ContainerInstanceArn != "" {
			arnSet.Add(t.ContainerInstanceArn)
		}
	}
	if len(arnSet) == 0 {
		return nil, nil
	}
	arns := arnSet.Items()
	var mu sync.Mutex
	containerToEC2 := make(map[string]string)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.requestConcurrency)
	for i := 0; i < len(arns); i += 100 {
		batch := arns[i:min(i+100, len(arns))]
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			data, err := cfg.awsConfig.GetECSAPIResponse(cfg.ecsEndpoint, "DescribeContainerInstances", buildDescribeBody(clusterARN, false, "containerInstances", batch))
			if err != nil {
				return fmt.Errorf("cannot describe container instances for cluster %q: %w", clusterARN, err)
			}
			resp, err := parseDescribeContainerInstancesResponse(data)
			if err != nil {
				return fmt.Errorf("cannot parse container instances for cluster %q: %w", clusterARN, err)
			}
			mu.Lock()
			for _, ci := range resp.ContainerInstances {
				if ci.ContainerInstanceArn != "" && ci.Ec2InstanceId != "" {
					containerToEC2[ci.ContainerInstanceArn] = ci.Ec2InstanceId
				}
			}
			mu.Unlock()
			return nil
		})
	}
	return containerToEC2, g.Wait()
}

// DescribeContainerInstancesResponse represents response to https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeContainerInstances.html
type DescribeContainerInstancesResponse struct {
	ContainerInstances []ContainerInstance `json:"containerInstances"`
}

func parseDescribeContainerInstancesResponse(data []byte) (*DescribeContainerInstancesResponse, error) {
	var v DescribeContainerInstancesResponse
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal DescribeContainerInstances response from %q: %w", data, err)
	}
	return &v, nil
}

// Task represents a task from https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Task.html
type Task struct {
	TaskArn              string       `json:"taskArn"`
	TaskDefinitionArn    string       `json:"taskDefinitionArn"`
	ContainerInstanceArn string       `json:"containerInstanceArn"`
	Group                string       `json:"group"`
	LaunchType           string       `json:"launchType"`
	AvailabilityZone     string       `json:"availabilityZone"`
	DesiredStatus        string       `json:"desiredStatus"`
	LastStatus           string       `json:"lastStatus"`
	HealthStatus         string       `json:"healthStatus"`
	PlatformFamily       string       `json:"platformFamily"`
	PlatformVersion      string       `json:"platformVersion"`
	Attachments          []Attachment `json:"attachments"`
	Tags                 []Tag        `json:"tags"`
}

// Attachment represents a task attachment from https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Attachment.html
type Attachment struct {
	Type    string             `json:"type"`
	Details []AttachmentDetail `json:"details"`
}

// AttachmentDetail represents a key/value detail on an attachment.
type AttachmentDetail struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ContainerInstance represents a container instance from https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerInstance.html
type ContainerInstance struct {
	ContainerInstanceArn string `json:"containerInstanceArn"`
	Ec2InstanceId        string `json:"ec2InstanceId"`
}
