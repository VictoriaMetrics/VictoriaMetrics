package logstorage

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestStorageSearchStreamIDs(t *testing.T) {
	const path = "TestStorageSearchStreamIDs"
	const partitionName = "foobar"
	s := newTestStorage()
	mustCreateIndexdb(path)
	idb := mustOpenIndexdb(path, partitionName, s)

	tenantID := TenantID{
		AccountID: 123,
		ProjectID: 567,
	}
	getStreamIDForTags := func(tags map[string]string) (streamID, []byte) {
		st := GetStreamTags()
		for k, v := range tags {
			st.Add(k, v)
		}
		streamTagsCanonical := st.MarshalCanonical(nil)
		PutStreamTags(st)
		id := hash128(streamTagsCanonical)
		sid := streamID{
			tenantID: tenantID,
			id:       id,
		}
		return sid, streamTagsCanonical
	}

	// Create indexdb entries
	const jobsCount = 7
	const instancesCount = 5
	for i := 0; i < jobsCount; i++ {
		for j := 0; j < instancesCount; j++ {
			sid, streamTagsCanonical := getStreamIDForTags(map[string]string{
				"job":      fmt.Sprintf("job-%d", i),
				"instance": fmt.Sprintf("instance-%d", j),
			})
			idb.mustRegisterStream(&sid, streamTagsCanonical)
		}
	}
	idb.debugFlush()

	f := func(streamFilter string, expectedStreamIDs []streamID) {
		t.Helper()
		sf := mustNewStreamFilter(streamFilter)
		if expectedStreamIDs == nil {
			expectedStreamIDs = []streamID{}
		}
		sortStreamIDs(expectedStreamIDs)
		for i := 0; i < 3; i++ {
			streamIDs := idb.searchStreamIDs([]TenantID{tenantID}, sf)
			if !reflect.DeepEqual(streamIDs, expectedStreamIDs) {
				t.Fatalf("unexpected streamIDs on iteration %d; got %v; want %v", i, streamIDs, expectedStreamIDs)
			}
		}
	}

	t.Run("missing-tenant-id", func(t *testing.T) {
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 2,
		}
		sf := mustNewStreamFilter(`{job="job-0",instance="instance-0"}`)
		for i := 0; i < 3; i++ {
			streamIDs := idb.searchStreamIDs([]TenantID{tenantID}, sf)
			if len(streamIDs) > 0 {
				t.Fatalf("unexpected non-empty streamIDs on iteration %d: %d", i, len(streamIDs))
			}
		}
	})
	t.Run("missing-job", func(t *testing.T) {
		f(`{job="non-existing-job",instance="instance-0"}`, nil)
	})
	t.Run("missing-job-re", func(t *testing.T) {
		f(`{job=~"non-existing-job|",instance="instance-0"}`, nil)
	})
	t.Run("missing-job-negative-re", func(t *testing.T) {
		f(`{job!~"job.+",instance="instance-0"}`, nil)
	})
	t.Run("empty-job", func(t *testing.T) {
		f(`{job="",instance="instance-0"}`, nil)
	})
	t.Run("missing-instance", func(t *testing.T) {
		f(`{job="job-0",instance="non-existing-instance"}`, nil)
	})
	t.Run("missing-instance-re", func(t *testing.T) {
		f(`{job="job-0",instance=~"non-existing-instance|"}`, nil)
	})
	t.Run("missing-instance-negative-re", func(t *testing.T) {
		f(`{job="job-0",instance!~"instance.+"}`, nil)
	})
	t.Run("empty-instance", func(t *testing.T) {
		f(`{job="job-0",instance=""}`, nil)
	})
	t.Run("non-existing-tag", func(t *testing.T) {
		f(`{job="job-0",instance="instance-0",non_existing_tag="foobar"}`, nil)
	})
	t.Run("non-existing-non-empty-tag", func(t *testing.T) {
		f(`{job="job-0",instance="instance-0",non_existing_tag!=""}`, nil)
	})
	t.Run("non-existing-tag-re", func(t *testing.T) {
		f(`{job="job-0",instance="instance-0",non_existing_tag=~"foo.+"}`, nil)
	})
	t.Run("non-existing-non-empty-tag-re", func(t *testing.T) {
		f(`{job="job-0",instance="instance-0",non_existing_tag!~""}`, nil)
	})

	t.Run("match-job-instance", func(t *testing.T) {
		sid, _ := getStreamIDForTags(map[string]string{
			"instance": "instance-0",
			"job":      "job-0",
		})
		f(`{job="job-0",instance="instance-0"}`, []streamID{sid})
	})
	t.Run("match-non-existing-tag", func(t *testing.T) {
		sid, _ := getStreamIDForTags(map[string]string{
			"instance": "instance-0",
			"job":      "job-0",
		})
		f(`{job="job-0",instance="instance-0",non_existing_tag=~"foo|"}`, []streamID{sid})
	})
	t.Run("match-job", func(t *testing.T) {
		var streamIDs []streamID
		for i := 0; i < instancesCount; i++ {
			sid, _ := getStreamIDForTags(map[string]string{
				"instance": fmt.Sprintf("instance-%d", i),
				"job":      "job-0",
			})
			streamIDs = append(streamIDs, sid)
		}
		f(`{job="job-0"}`, streamIDs)
	})
	t.Run("match-instance", func(t *testing.T) {
		var streamIDs []streamID
		for i := 0; i < jobsCount; i++ {
			sid, _ := getStreamIDForTags(map[string]string{
				"instance": "instance-1",
				"job":      fmt.Sprintf("job-%d", i),
			})
			streamIDs = append(streamIDs, sid)
		}
		f(`{instance="instance-1"}`, streamIDs)
	})
	t.Run("match-re", func(t *testing.T) {
		var streamIDs []streamID
		for _, instanceID := range []int{3, 1} {
			for _, jobID := range []int{0, 2} {
				sid, _ := getStreamIDForTags(map[string]string{
					"instance": fmt.Sprintf("instance-%d", instanceID),
					"job":      fmt.Sprintf("job-%d", jobID),
				})
				streamIDs = append(streamIDs, sid)
			}
		}
		f(`{job=~"job-(0|2)",instance=~"instance-[13]"}`, streamIDs)
	})
	t.Run("match-re-empty-match", func(t *testing.T) {
		var streamIDs []streamID
		for _, instanceID := range []int{3, 1} {
			for _, jobID := range []int{0, 2} {
				sid, _ := getStreamIDForTags(map[string]string{
					"instance": fmt.Sprintf("instance-%d", instanceID),
					"job":      fmt.Sprintf("job-%d", jobID),
				})
				streamIDs = append(streamIDs, sid)
			}
		}
		f(`{job=~"job-(0|2)|",instance=~"instance-[13]"}`, streamIDs)
	})
	t.Run("match-negative-re", func(t *testing.T) {
		var instanceIDs []int
		for i := 0; i < instancesCount; i++ {
			if i != 0 && i != 1 {
				instanceIDs = append(instanceIDs, i)
			}
		}
		var jobIDs []int
		for i := 0; i < jobsCount; i++ {
			if i > 2 {
				jobIDs = append(jobIDs, i)
			}
		}
		var streamIDs []streamID
		for _, instanceID := range instanceIDs {
			for _, jobID := range jobIDs {
				sid, _ := getStreamIDForTags(map[string]string{
					"instance": fmt.Sprintf("instance-%d", instanceID),
					"job":      fmt.Sprintf("job-%d", jobID),
				})
				streamIDs = append(streamIDs, sid)
			}
		}
		f(`{job!~"job-[0-2]",instance!~"instance-(0|1)"}`, streamIDs)
	})
	t.Run("match-negative-re-empty-match", func(t *testing.T) {
		var instanceIDs []int
		for i := 0; i < instancesCount; i++ {
			if i != 0 && i != 1 {
				instanceIDs = append(instanceIDs, i)
			}
		}
		var jobIDs []int
		for i := 0; i < jobsCount; i++ {
			if i > 2 {
				jobIDs = append(jobIDs, i)
			}
		}
		var streamIDs []streamID
		for _, instanceID := range instanceIDs {
			for _, jobID := range jobIDs {
				sid, _ := getStreamIDForTags(map[string]string{
					"instance": fmt.Sprintf("instance-%d", instanceID),
					"job":      fmt.Sprintf("job-%d", jobID),
				})
				streamIDs = append(streamIDs, sid)
			}
		}
		f(`{job!~"job-[0-2]",instance!~"instance-(0|1)|"}`, streamIDs)
	})
	t.Run("match-negative-job", func(t *testing.T) {
		instanceIDs := []int{2}
		var jobIDs []int
		for i := 0; i < jobsCount; i++ {
			if i != 1 {
				jobIDs = append(jobIDs, i)
			}
		}
		var streamIDs []streamID
		for _, instanceID := range instanceIDs {
			for _, jobID := range jobIDs {
				sid, _ := getStreamIDForTags(map[string]string{
					"instance": fmt.Sprintf("instance-%d", instanceID),
					"job":      fmt.Sprintf("job-%d", jobID),
				})
				streamIDs = append(streamIDs, sid)
			}
		}
		f(`{instance="instance-2",job!="job-1"}`, streamIDs)
	})

	mustCloseIndexdb(idb)
	fs.MustRemoveAll(path)

	closeTestStorage(s)
}
