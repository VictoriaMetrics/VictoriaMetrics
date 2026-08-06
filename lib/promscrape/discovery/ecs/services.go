package ecs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"golang.org/x/sync/errgroup"
)

func listAllServiceARNs(ctx context.Context, cfg *apiConfig, clusterARNs []string) (map[string][]string, error) {
	var mu sync.Mutex
	result := make(map[string][]string, len(clusterARNs))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.requestConcurrency)
	for _, clusterARN := range clusterARNs {
		g.Go(func() error {
			arns, err := listServices(gctx, cfg, clusterARN)
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

func listServices(ctx context.Context, cfg *apiConfig, clusterARN string) ([]string, error) {
	// See https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListServices.html
	var arns []string
	nextToken := ""
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := cfg.awsConfig.GetECSAPIResponse(cfg.ecsEndpoint, "ListServices", buildListBody(clusterARN, nextToken))
		if err != nil {
			return nil, fmt.Errorf("cannot list services for cluster %q: %w", clusterARN, err)
		}
		resp, err := parseListServicesResponse(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse service list for cluster %q: %w", clusterARN, err)
		}
		arns = append(arns, resp.ServiceArns...)
		if resp.NextToken == "" {
			return arns, nil
		}
		nextToken = resp.NextToken
	}
}

// ListServicesResponse represents response to https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListServices.html
type ListServicesResponse struct {
	ServiceArns []string `json:"serviceArns"`
	NextToken   string   `json:"nextToken"`
}

func parseListServicesResponse(data []byte) (*ListServicesResponse, error) {
	var v ListServicesResponse
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal ListServices response from %q: %w", data, err)
	}
	return &v, nil
}

func describeServices(ctx context.Context, cfg *apiConfig, clusterARN string, arns []string) ([]Service, error) {
	// See https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeServices.html
	// DescribeServices allows up to 10 services per call.
	var mu sync.Mutex
	var services []Service
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.requestConcurrency)
	for i := 0; i < len(arns); i += 10 {
		batch := arns[i:min(i+10, len(arns))]
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			data, err := cfg.awsConfig.GetECSAPIResponse(cfg.ecsEndpoint, "DescribeServices", buildDescribeBody(clusterARN, true, "services", batch))
			if err != nil {
				return fmt.Errorf("cannot describe services for cluster %q: %w", clusterARN, err)
			}
			resp, err := parseDescribeServicesResponse(data)
			if err != nil {
				return fmt.Errorf("cannot parse services for cluster %q: %w", clusterARN, err)
			}
			mu.Lock()
			services = append(services, resp.Services...)
			mu.Unlock()
			return nil
		})
	}
	return services, g.Wait()
}

// DescribeServicesResponse represents response to https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeServices.html
type DescribeServicesResponse struct {
	Services []Service `json:"services"`
}

func parseDescribeServicesResponse(data []byte) (*DescribeServicesResponse, error) {
	var v DescribeServicesResponse
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal DescribeServices response from %q: %w", data, err)
	}
	return &v, nil
}

// Service represents a service from https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html
type Service struct {
	ServiceArn  string `json:"serviceArn"`
	ServiceName string `json:"serviceName"`
	Status      string `json:"status"`
	Tags        []Tag  `json:"tags"`
}

func (svc *Service) appendLabels(m *promutil.Labels) {
	m.Add("__meta_ecs_service", svc.ServiceName)
	m.Add("__meta_ecs_service_arn", svc.ServiceArn)
	m.Add("__meta_ecs_service_status", svc.Status)
	for _, tag := range svc.Tags {
		if tag.Key != "" && tag.Value != "" {
			m.Add(discoveryutil.SanitizeLabelName("__meta_ecs_tag_service_"+tag.Key), tag.Value)
		}
	}
}
