package ecsmetadata

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
)

var (
	metadataAddr   = flag.String("containerStats.url", "", "ECS task metadata endpoint base URL. Empty disables ECS metadata collection")
	scrapeInterval = flag.Duration("containerStats.scrapeInterval", 15*time.Second, "Scrape interval for ECS task metadata")
	scrapeTimeout  = flag.Duration("containerStats.scrapeTimeout", 5*time.Second, "Timeout for ECS task metadata requests")
)

var (
	httpClient   = &http.Client{}
	rowsInserted = metrics.NewCounter(`vmagent_rows_inserted_total{type="ecs"}`)
)

type TaskResponse struct {
	Metadata TaskMetadata
	Stats    map[string]*ContainerStats
}

type TaskMetadata struct {
	Cluster          string `json:"Cluster"`
	TaskARN          string `json:"TaskARN"`
	Family           string `json:"Family"`
	Revision         string `json:"Revision"`
	DesiredStatus    string `json:"DesiredStatus"`
	KnownStatus      string `json:"KnownStatus"`
	AvailabilityZone string `json:"AvailabilityZone"`
	LaunchType       string `json:"LaunchType"`

	Limits     *TaskLimits         `json:"Limits"`
	Ephemeral  *EphemeralStorage   `json:"EphemeralStorageMetrics"`
	Containers []ContainerMetadata `json:"Containers"`
}

type TaskLimits struct {
	CPU    *float64 `json:"CPU"`
	Memory *int64   `json:"Memory"`
}

type EphemeralStorage struct {
	UtilizedMiBs int64 `json:"Utilized"`
	ReservedMiBs int64 `json:"Reserved"`
}

type ContainerMetadata struct {
	ID           string `json:"DockerId"`
	Name         string `json:"Name"`
	KnownStatus  string `json:"KnownStatus"`
	RestartCount *int   `json:"RestartCount"`
	Limits       struct {
		Memory *int64 `json:"Memory"`
	} `json:"Limits"`
}

type ContainerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
	} `json:"cpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]NetworkStats `json:"networks"`
}

type NetworkStats struct {
	RxBytes  uint64 `json:"rx_bytes"`
	RxErrors uint64 `json:"rx_errors"`
	TxBytes  uint64 `json:"tx_bytes"`
	TxErrors uint64 `json:"tx_errors"`
}

func FetchTask(ctx context.Context, client *http.Client, addr string) (*TaskResponse, error) {
	meta, err := fetch[TaskMetadata](ctx, client, addr+"/task")
	if err != nil {
		return nil, fmt.Errorf("task metadata: %w", err)
	}
	stats, err := fetch[map[string]*ContainerStats](ctx, client, addr+"/task/stats")
	if err != nil {
		logger.Errorf("cannot fetch ECS task stats from %s: %s", addr, err)
	}
	var statsMap map[string]*ContainerStats
	if stats != nil {
		statsMap = *stats
	}
	return &TaskResponse{Metadata: *meta, Stats: statsMap}, nil
}

func fetch[T any](ctx context.Context, client *http.Client, url string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var v T
	if err = json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (resp *TaskResponse) WriteMetrics(w io.Writer) {
	meta := &resp.Metadata

	fmt.Fprintf(w, "ecs_task_metadata_info{availability_zone=%q,cluster=%q,desired_status=%q,family=%q,known_status=%q,launch_type=%q,revision=%q,task_arn=%q} 1\n",
		meta.AvailabilityZone, meta.Cluster, meta.DesiredStatus, meta.Family,
		meta.KnownStatus, meta.LaunchType, meta.Revision, meta.TaskARN,
	)

	if meta.Limits != nil {
		if meta.Limits.CPU != nil {
			fmt.Fprintf(w, "ecs_task_cpu_limit_vcpus %g\n", *meta.Limits.CPU)
		}
		if meta.Limits.Memory != nil {
			fmt.Fprintf(w, "ecs_task_memory_limit_bytes %g\n", float64(*meta.Limits.Memory*1024*1024))
		}
	}

	if meta.Ephemeral != nil {
		fmt.Fprintf(w, "ecs_task_ephemeral_storage_used_bytes %g\n", float64(meta.Ephemeral.UtilizedMiBs*1024*1024))
		fmt.Fprintf(w, "ecs_task_ephemeral_storage_allocated_bytes %g\n", float64(meta.Ephemeral.ReservedMiBs*1024*1024))
	}

	networks := make(map[string]*NetworkStats)

	for _, ctr := range meta.Containers {
		if ctr.KnownStatus != "RUNNING" {
			continue
		}
		s := resp.Stats[ctr.ID]
		if s == nil {
			continue
		}
		n := ctr.Name

		if ctr.RestartCount != nil {
			fmt.Fprintf(w, "ecs_container_restarts_total{container_name=%q} %d\n", n, *ctr.RestartCount)
		}
		fmt.Fprintf(w, "ecs_container_cpu_usage_seconds_total{container_name=%q} %g\n", n, float64(s.CPUStats.CPUUsage.TotalUsage)*1e-9)
		fmt.Fprintf(w, "ecs_container_memory_usage_bytes{container_name=%q} %g\n", n, float64(s.MemoryStats.Usage))
		fmt.Fprintf(w, "ecs_container_memory_limit_bytes{container_name=%q} %g\n", n, float64(containerMemLimitBytes(ctr, meta.Limits)))
		fmt.Fprintf(w, "ecs_container_memory_page_cache_size_bytes{container_name=%q} %g\n", n, float64(s.MemoryStats.Stats["cache"]))

		for iface, ns := range s.Networks {
			networks[iface] = &ns
		}
	}

	for iface, ns := range networks {
		fmt.Fprintf(w, "ecs_network_receive_bytes_total{interface=%q} %g\n", iface, float64(ns.RxBytes))
		fmt.Fprintf(w, "ecs_network_receive_errors_total{interface=%q} %g\n", iface, float64(ns.RxErrors))
		fmt.Fprintf(w, "ecs_network_transmit_bytes_total{interface=%q} %g\n", iface, float64(ns.TxBytes))
		fmt.Fprintf(w, "ecs_network_transmit_errors_total{interface=%q} %g\n", iface, float64(ns.TxErrors))
	}
}

func containerMemLimitBytes(ctr ContainerMetadata, task *TaskLimits) int64 {
	if ctr.Limits.Memory != nil {
		return *ctr.Limits.Memory * 1024 * 1024
	}
	if task != nil && task.Memory != nil {
		return *task.Memory * 1024 * 1024
	}
	return 0
}

var (
	stopCh chan struct{}
	wg     sync.WaitGroup
)

func Start() {
	addr := *metadataAddr
	if addr == "" {
		addr = os.Getenv("ECS_CONTAINER_METADATA_URI_V4")
	}
	if addr == "" {
		return
	}
	stopCh = make(chan struct{})
	logger.Infof("scraping ECS task metadata from %s every %s", addr, *scrapeInterval)
	wg.Go(func() {
		run(addr)
	})
}

func Stop() {
	if stopCh != nil {
		close(stopCh)
		wg.Wait()
	}
}

func run(addr string) {
	t := time.NewTicker(*scrapeInterval)
	defer t.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-t.C:
			if err := collect(addr); err != nil {
				logger.Errorf("cannot collect ECS metrics from %s: %s", addr, err)
			}
		}
	}
}

func collect(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), *scrapeTimeout)
	defer cancel()

	resp, err := FetchTask(ctx, httpClient, addr)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	resp.WriteMetrics(&buf)
	return pushMetrics(buf.String())
}

func pushMetrics(text string) error {
	var rows prometheus.Rows
	rows.UnmarshalWithErrLogger(text, func(s string) {
		logger.Errorf("cannot parse ECS metric line: %s", s)
	})
	if len(rows.Rows) == 0 {
		return nil
	}

	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	now := int64(fasttime.UnixTimestamp()) * 1000
	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]

	for i := range rows.Rows {
		r := &rows.Rows[i]
		labelsLen := len(labels)
		labels = append(labels, prompb.Label{Name: "__name__", Value: r.Metric})
		for j := range r.Tags {
			tag := &r.Tags[j]
			labels = append(labels, prompb.Label{Name: tag.Key, Value: tag.Value})
		}
		ts := r.Timestamp
		if ts == 0 {
			ts = now
		}
		samples = append(samples, prompb.Sample{Value: r.Value, Timestamp: ts})
		tssDst = append(tssDst, prompb.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: samples[len(samples)-1:],
		})
	}

	ctx.WriteRequest.Timeseries = tssDst
	ctx.Labels = labels
	ctx.Samples = samples

	if !remotewrite.TryPush(nil, &ctx.WriteRequest) {
		return fmt.Errorf("remote write queue is full; %d ECS rows dropped", len(tssDst))
	}
	rowsInserted.Add(len(tssDst))
	return nil
}
