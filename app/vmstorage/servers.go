package main

import (
	"flag"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vminsertapi"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

var (
	maxTagKeys = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned per search. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
	maxTagValues = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned per search. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
	maxTagValueSuffixesPerSearch = flag.Int("search.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned from /metrics/find")
	maxConcurrentRequests        = flag.Int("search.maxConcurrentRequests", 2*cgroup.AvailableCPUs(), "The maximum number of concurrent vmselect requests "+
		"the vmstorage can process at -vmselectAddr. It shouldn't be high, since a single request usually saturates a CPU core, and many concurrently executed requests "+
		"may require high amounts of memory. See also -search.maxQueueDuration")
	maxQueueDuration = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the incoming vmselect request waits for execution "+
		"when -search.maxConcurrentRequests limit is reached")
	disableRPCCompression = flag.Bool("rpc.disableCompression", false, "Whether to disable compression of the data sent from vmstorage to vmselect. "+
		"This reduces CPU usage at the cost of higher network bandwidth usage")
	vminsertConnsShutdownDuration = flag.Duration("storage.vminsertConnsShutdownDuration", 10*time.Second, "The time needed for gradual closing of vminsert connections during "+
		"graceful shutdown. Bigger duration reduces spikes in CPU, RAM and disk IO load on the remaining vmstorage nodes during rolling restart. "+
		"Smaller duration reduces the time needed to close all the vminsert connections, thus reducing the time for graceful shutdown. "+
		"Configured value must always be lower than the graceful shutdown period configured by the orchestration platform (terminationGracePeriodSeconds for Kubernetes). "+
		"See https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#improving-re-routing-performance-during-restart")
)

// newVMSelectServer starts new server at the given addr, which serves vmselect requests from the given s.
func newVMSelectServer(addr string, api vmselectapi.API) (*vmselectapi.Server, error) {
	limits := vmselectapi.Limits{
		MaxLabelNames:                 *maxTagKeys,
		MaxLabelValues:                *maxTagValues,
		MaxTagValueSuffixes:           *maxTagValueSuffixesPerSearch,
		MaxConcurrentRequests:         *maxConcurrentRequests,
		MaxConcurrentRequestsFlagName: "search.maxConcurrentRequests",
		MaxQueueDuration:              *maxQueueDuration,
		MaxQueueDurationFlagName:      "search.maxQueueDuration",
	}
	return vmselectapi.NewServer(addr, api, limits, *disableRPCCompression)
}

// newVMInsertServer starts vminsertapi.VMInsertServer at the given addr serving the given storage.
func newVMInsertServer(addr string, api vminsertapi.API) (*vminsertapi.VMInsertServer, error) {
	return vminsertapi.NewVMInsertServer(addr, *vminsertConnsShutdownDuration, "vminsert", api, nil)
}
