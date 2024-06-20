package logstorage

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestStorageSearchStreamIDs(t *testing.T) {
	t.Parallel()

	path := t.Name()
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

	f := func(filterStream string, expectedStreamIDs []streamID) {
		t.Helper()
		sf := mustNewTestStreamFilter(filterStream)
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
		sf := mustNewTestStreamFilter(`{job="job-0",instance="instance-0"}`)
		for i := 0; i < 3; i++ {
			streamIDs := idb.searchStreamIDs([]TenantID{tenantID}, sf)
			if len(streamIDs) > 0 {
				t.Fatalf("unexpected non-empty streamIDs on iteration %d: %d", i, len(streamIDs))
			}
		}
	})

	// missing-job
	f(`{job="non-existing-job",instance="instance-0"}`, nil)

	// missing-job-re
	f(`{job=~"non-existing-job|",instance="instance-0"}`, nil)

	// missing-job-negative-re
	f(`{job!~"job.+",instance="instance-0"}`, nil)

	// empty-job
	f(`{job="",instance="instance-0"}`, nil)

	// missing-instance
	f(`{job="job-0",instance="non-existing-instance"}`, nil)

	// missing-instance-re
	f(`{job="job-0",instance=~"non-existing-instance|"}`, nil)

	// missing-instance-negative-re
	f(`{job="job-0",instance!~"instance.+"}`, nil)

	// empty-instance
	f(`{job="job-0",instance=""}`, nil)

	// non-existing-tag
	f(`{job="job-0",instance="instance-0",non_existing_tag="foobar"}`, nil)

	// non-existing-non-empty-tag
	f(`{job="job-0",instance="instance-0",non_existing_tag!=""}`, nil)

	// non-existing-tag-re
	f(`{job="job-0",instance="instance-0",non_existing_tag=~"foo.+"}`, nil)

	//non-existing-non-empty-tag-re
	f(`{job="job-0",instance="instance-0",non_existing_tag!~""}`, nil)

	// match-job-instance
	sid, _ := getStreamIDForTags(map[string]string{
		"instance": "instance-0",
		"job":      "job-0",
	})
	f(`{job="job-0",instance="instance-0"}`, []streamID{sid})

	// match-non-existing-tag
	sid, _ = getStreamIDForTags(map[string]string{
		"instance": "instance-0",
		"job":      "job-0",
	})
	f(`{job="job-0",instance="instance-0",non_existing_tag=~"foo|"}`, []streamID{sid})

	// match-job
	var streamIDs []streamID
	for i := 0; i < instancesCount; i++ {
		sid, _ := getStreamIDForTags(map[string]string{
			"instance": fmt.Sprintf("instance-%d", i),
			"job":      "job-0",
		})
		streamIDs = append(streamIDs, sid)
	}
	f(`{job="job-0"}`, streamIDs)

	// match-instance
	streamIDs = nil
	for i := 0; i < jobsCount; i++ {
		sid, _ := getStreamIDForTags(map[string]string{
			"instance": "instance-1",
			"job":      fmt.Sprintf("job-%d", i),
		})
		streamIDs = append(streamIDs, sid)
	}
	f(`{instance="instance-1"}`, streamIDs)

	// match-re
	streamIDs = nil
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

	// match-re-empty-match
	streamIDs = nil
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

	// match-negative-re
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
	streamIDs = nil
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

	// match-negative-re-empty-match
	instanceIDs = nil
	for i := 0; i < instancesCount; i++ {
		if i != 0 && i != 1 {
			instanceIDs = append(instanceIDs, i)
		}
	}
	jobIDs = nil
	for i := 0; i < jobsCount; i++ {
		if i > 2 {
			jobIDs = append(jobIDs, i)
		}
	}
	streamIDs = nil
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

	// match-negative-job
	instanceIDs = []int{2}
	jobIDs = nil
	for i := 0; i < jobsCount; i++ {
		if i != 1 {
			jobIDs = append(jobIDs, i)
		}
	}
	streamIDs = nil
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

	mustCloseIndexdb(idb)
	fs.MustRemoveAll(path)

	closeTestStorage(s)
}
