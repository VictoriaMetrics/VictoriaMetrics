package ecs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

func listClusters(ctx context.Context, cfg *apiConfig) ([]string, error) {
	// See https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListClusters.html
	var arns []string
	nextToken := ""
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := cfg.awsConfig.GetECSAPIResponse(cfg.ecsEndpoint, "ListClusters", buildListBody("", nextToken))
		if err != nil {
			return nil, fmt.Errorf("cannot list clusters: %w", err)
		}
		resp, err := parseListClustersResponse(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse cluster list: %w", err)
		}
		arns = append(arns, resp.ClusterArns...)
		if resp.NextToken == "" {
			return arns, nil
		}
		nextToken = resp.NextToken
	}
}

// ListClustersResponse represents response to https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListClusters.html
type ListClustersResponse struct {
	ClusterArns []string `json:"clusterArns"`
	NextToken   string   `json:"nextToken"`
}

func parseListClustersResponse(data []byte) (*ListClustersResponse, error) {
	var v ListClustersResponse
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal ListClusters response from %q: %w", data, err)
	}
	return &v, nil
}

func describeClusters(ctx context.Context, cfg *apiConfig, arns []string) ([]Cluster, error) {
	// See https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeClusters.html
	// DescribeClusters allows up to 100 clusters per call.
	var mu sync.Mutex
	var clusters []Cluster
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.requestConcurrency)
	for i := 0; i < len(arns); i += 100 {
		batch := arns[i:min(i+100, len(arns))]
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			data, err := cfg.awsConfig.GetECSAPIResponse(cfg.ecsEndpoint, "DescribeClusters", buildDescribeBody("", true, "clusters", batch))
			if err != nil {
				return fmt.Errorf("cannot describe clusters: %w", err)
			}
			resp, err := parseDescribeClustersResponse(data)
			if err != nil {
				return fmt.Errorf("cannot parse clusters: %w", err)
			}
			mu.Lock()
			clusters = append(clusters, resp.Clusters...)
			mu.Unlock()
			return nil
		})
	}
	return clusters, g.Wait()
}

// DescribeClustersResponse represents response to https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeClusters.html
type DescribeClustersResponse struct {
	Clusters []Cluster `json:"clusters"`
}

func parseDescribeClustersResponse(data []byte) (*DescribeClustersResponse, error) {
	var v DescribeClustersResponse
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal DescribeClusters response from %q: %w", data, err)
	}
	return &v, nil
}

// Cluster represents a cluster from https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Cluster.html
type Cluster struct {
	ClusterArn  string `json:"clusterArn"`
	ClusterName string `json:"clusterName"`
	Tags        []Tag  `json:"tags"`
}
