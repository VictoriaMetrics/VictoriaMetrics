package vmstorage

import (
	"context"
	"flag"
	"fmt"
	"math"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricnamestats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

var (
	accountID = flag.Uint64("accountID", 0, "The accountID of the stored data")
	projectID = flag.Uint64("projectID", 0, "The projectID of the stored data")
)

func newVMStorageWithTenantID(vms *VMStorage) *VMStorageWithTenantID {
	if *accountID > math.MaxUint32 {
		logger.Fatalf("-accountID must be in the range [0, %d], got %d", uint32(math.MaxUint32), *accountID)
	}
	if *projectID > math.MaxUint32 {
		logger.Fatalf("-projectID must be in the range [0, %d], got %d", uint32(math.MaxUint32), *projectID)
	}
	return &VMStorageWithTenantID{
		vms:       vms,
		accountID: uint32(*accountID),
		projectID: uint32(*projectID),
	}
}

// VMStorageWithTenantID is a thin wrapper around VMStorage type that overrides
// its methods to properly serve requests coming from a vmselect (require
// tenantID).
//
// A new instance of this type should be created using
// newVMStorageWithTenantID(). The created instance does not require closing.
// The instance also does not take ownership of vms and it is the responsibility
// of the caller to close vms.
type VMStorageWithTenantID struct {
	vms *VMStorage

	accountID uint32
	projectID uint32
}

// InitSearch initializes a storage search for a request initiated by a
// vmselect.
//
// The search is initialized only if the search query is either multitenant or
// its accountID and projectID match -accountID and -projectID flag values.
// Otherwise, the method returns an interator that will return no data.
//
// The method also overrides the data format of the data returned by the
// iterator by prepending accountID and projectID bytes to the metric name and
// the data block (a format used in vmcluster).
func (vmst *VMStorageWithTenantID) InitSearch(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery) (vmselectapi.BlockIterator, error) {
	if !vmst.hasValidTenantID(sq) {
		return emptyBI, nil
	}
	return vmst.vms.initSearch(ctx, qt, sq, vmst.marshalMetricBlock)
}

var emptyBI = &emptyBlockIterator{}

// emptyBlockIterator is an implementation of vmselectapi.BlockIterator that
// always returns no data.
type emptyBlockIterator struct{}

func (*emptyBlockIterator) MustClose() {}

func (*emptyBlockIterator) NextBlock(_ context.Context, dst []byte) ([]byte, bool) {
	return dst, false
}

func (*emptyBlockIterator) Error() error {
	return nil
}

// marshalMetricBlock serializes a metric block in the format expected by
// vmselect.
//
// vmselect expects metric names and data blocks to have the tenantID but
// vmsingle does not have it. Therefore the tenantID needs to be included to
// every metric name and block.
func (vmst *VMStorageWithTenantID) marshalMetricBlock(dst []byte, src *storage.MetricBlock) []byte {
	// Marshal metric name:
	// 1. Marshal metric name length + accountID length + projectID length (in
	//    bytes).
	// 2. append accountID and projectID bytes
	// 3. Finally append metric name bytes
	dst = encoding.MarshalVarUint64(dst, uint64(len(src.MetricName))+8)
	dst = encoding.MarshalUint32(dst, vmst.accountID)
	dst = encoding.MarshalUint32(dst, vmst.projectID)
	dst = append(dst, src.MetricName...)

	// Marshal data block.
	dst = encoding.MarshalUint32(dst, vmst.accountID)
	dst = encoding.MarshalUint32(dst, vmst.projectID)
	dst = storage.MarshalBlock(dst, &src.Block)

	return dst
}

// SearchMetricNames searches the storage for metric names that match the query.
//
// If the query is not multitenant or the query accountID and projectID do not
// match the -accoutID and -projectID flag values, the method will return an
// empty result.
//
// Found metric names are prepended with accountID and projectID bytes (a format
// used in vmcluster).
func (vmst *VMStorageWithTenantID) SearchMetricNames(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery) ([]string, error) {
	if !vmst.hasValidTenantID(sq) {
		return nil, nil
	}

	metricNames, err := vmst.vms.SearchMetricNames(ctx, qt, sq)
	if err != nil {
		return nil, err
	}

	// vmselect expects metric names to have the tenantID but vmsingle does not
	// have it. Therefore the tenantID needs to be prepended to every metric
	// name.
	dst := make([]byte, 0, 8)
	dst = encoding.MarshalUint32(dst, vmst.accountID)
	dst = encoding.MarshalUint32(dst, vmst.projectID)
	tenantID := string(dst)

	for i, metricName := range metricNames {
		metricNames[i] = tenantID + metricName
	}
	return metricNames, nil
}

// LabelValues searches the storage for label values that match the query and
// correspond to a label whose name is `labelName`. The returned result
// will contain not more than `maxLabelValues`.
//
// If the query is not multitenant or the query accountID and projectID do not
// match the -accoutID and -projectID flag values, the method will return an
// empty result.
func (vmst *VMStorageWithTenantID) LabelValues(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int) ([]string, error) {
	if !vmst.hasValidTenantID(sq) {
		return nil, nil
	}
	return vmst.vms.LabelValues(ctx, qt, sq, labelName, maxLabelValues)
}

// TagValueSuffixes searches the storage for Graphite tag value suffixes. The
// returned result will contain not more than `maxSuffixes`.
//
// If the query is not multitenant or the query accountID and projectID do not
// match the -accoutID and -projectID flag values, the method will return an
// empty result.
func (vmst *VMStorageWithTenantID) TagValueSuffixes(ctx context.Context, qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxSuffixes int) ([]string, error) {
	if !vmst.isValidTenantID(accountID, projectID) {
		return nil, nil
	}
	return vmst.vms.TagValueSuffixes(ctx, qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes)
}

// LabelNames searches the storage for label names that match the query.
// The returned result will contain not more than `maxLabelNames`.
//
// If the query is not multitenant or the query accountID and projectID do not
// match the -accoutID and -projectID flag values, the method will return an
// empty result.
func (vmst *VMStorageWithTenantID) LabelNames(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int) ([]string, error) {
	if !vmst.hasValidTenantID(sq) {
		return nil, nil
	}
	return vmst.vms.LabelNames(ctx, qt, sq, maxLabelNames)
}

// SeriesCount returns the total number of metrics stored in the database.
//
// The method may return inflated numbers. How inflated the count depends
// on the churn rate and the retention period. For example, if a metric lasts
// for 2 months, it will be counted twice.
//
// The method also counts the deleted metrics.
//
// If the query is not multitenant or the query accountID and projectID do not
// match the -accoutID and -projectID flag values, the method will return 0.
func (vmst *VMStorageWithTenantID) SeriesCount(ctx context.Context, qt *querytracer.Tracer, accountID, projectID uint32) (uint64, error) {
	if !vmst.isValidTenantID(accountID, projectID) {
		return 0, nil
	}
	return vmst.vms.SeriesCount(ctx, qt, accountID, projectID)
}

// Tenants returns just one tenant consisting of the -accountID and -projectID
// flag values.
func (vmst *VMStorageWithTenantID) Tenants(_ context.Context, qt *querytracer.Tracer, tr storage.TimeRange) ([]string, error) {
	tenantID := fmt.Sprintf("%d:%d", vmst.accountID, vmst.projectID)
	return []string{tenantID}, nil
}

// TSDBStatus retrieves the status for metrics that match to the search query.
//
// If the query is not multitenant or the query accountID and projectID do not
// match the -accoutID and -projectID flag values, the method will return empty
// status.
func (vmst *VMStorageWithTenantID) TSDBStatus(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int) (*storage.TSDBStatus, error) {
	if !vmst.hasValidTenantID(sq) {
		return &storage.TSDBStatus{}, nil
	}
	return vmst.vms.TSDBStatus(ctx, qt, sq, focusLabel, topN)
}

// DeleteSeries marks as deleted metrics that match the search query.
// The method returns the number of deleted metrics.
//
// If the query is not multitenant or the query accountID and projectID do not
// match the -accoutID and -projectID flag values, no metrics will be deleted
// and the method will return 0.
func (vmst *VMStorageWithTenantID) DeleteSeries(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery) (int, error) {
	if !vmst.hasValidTenantID(sq) {
		return 0, nil
	}
	return vmst.vms.DeleteSeries(ctx, qt, sq)
}

// RegisterMetricNames registers metric names in the index, the sample values
// and timestamps are ignored.
func (vmst *VMStorageWithTenantID) RegisterMetricNames(ctx context.Context, qt *querytracer.Tracer, mrs []storage.MetricRow) error {
	return vmst.vms.RegisterMetricNames(ctx, qt, mrs)
}

// GetMetricNamesUsageStats retrieves the usage stats for metrics whose name
// matches the pattern.
//
// If the request is not multitenant or the request accountID and projectID do
// not match the -accoutID and -projectID flag values, no metrics will be
// deleted and the method will return 0.
func (vmst *VMStorageWithTenantID) GetMetricNamesUsageStats(ctx context.Context, qt *querytracer.Tracer, tt *storage.TenantToken, limit, le int, matchPattern string) (metricnamestats.StatsResult, error) {
	if !vmst.isValidTenantToken(tt) {
		return metricnamestats.StatsResult{}, nil
	}
	return vmst.vms.GetMetricNamesUsageStats(ctx, qt, tt, limit, le, matchPattern)
}

// ResetMetricNamesUsageStats resets the metric name usage stats.
func (vmst *VMStorageWithTenantID) ResetMetricNamesUsageStats(ctx context.Context, qt *querytracer.Tracer) error {
	return vmst.vms.ResetMetricNamesUsageStats(ctx, qt)
}

// GetMetadataRecords retrieves the metadata for the metricName.
//
// If the request is not multitenant or the request accountID and projectID do
// not match the -accoutID and -projectID flag values, no metrics will be
// deleted and the method will return 0.
func (vmst *VMStorageWithTenantID) GetMetadataRecords(ctx context.Context, qt *querytracer.Tracer, tt *storage.TenantToken, limit int, metricName string) ([]*metricsmetadata.Row, error) {
	if !vmst.isValidTenantToken(tt) {
		return nil, nil
	}
	return vmst.vms.GetMetadataRecords(ctx, qt, tt, limit, metricName)
}

// hasValidTenantID returns true if the search query is either multitenant or
// its accountID and projectID match -accountID and -projectID flag values.
func (vmst *VMStorageWithTenantID) hasValidTenantID(sq *storage.SearchQuery) bool {
	return sq.IsMultiTenant || vmst.isValidTenantID(sq.AccountID, sq.ProjectID)
}

// isValidTenantToken returns true if the TenantToken is either multitenant or
// its accountID and projectID match -accountID and -projectID flag values.
func (vmst *VMStorageWithTenantID) isValidTenantToken(tt *storage.TenantToken) bool {
	return tt == nil || vmst.isValidTenantID(tt.AccountID, tt.ProjectID)
}

// isValidTenantID returns true if the accountID and projectID match -accountID
// and -projectID flag values.
func (vmst *VMStorageWithTenantID) isValidTenantID(accountID, projectID uint32) bool {
	return accountID == vmst.accountID && projectID == vmst.projectID
}
