package tests

import (
	"fmt"
	"regexp"
	"testing"

	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/google/go-cmp/cmp"
)

// snapshotNameRE convers years 1970-2099.
// Corner case examples:
// - 19700101000000-0000000000000000
// - 20991231235959-38EECC8925ED5FFF
var snapshotNameRE = regexp.MustCompile(`^(19[789]\d|20[0-9]{2})(0\d|1[0-2])([0-2]\d|3[01])([01]\d|2[0-3])[0-5]\d[0-5]\d-[0-9,A-F]{16}$`)

func TestSingleSnapshots_CreateListDelete(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	// Insert some data.
	const numSamples = 1000
	samples := make([]string, numSamples)
	for i := range numSamples {
		samples[i] = fmt.Sprintf("metric_%03d %d", i, i)
	}
	sut.PrometheusAPIV1ImportPrometheus(t, samples, at.QueryOpts{})
	sut.ForceFlush(t)

	// Create several snapshots using VictoriaMetrics and Prometheus endpoints.
	const numSnapshots = 4
	snapshots := make([]string, numSnapshots*2)
	i := 0
	for range numSnapshots {
		res := sut.SnapshotCreate(t)
		if got, want := res.Status, "ok"; got != want {
			t.Fatalf("unexpected snapshot creation status: got %q, want %q", got, want)
		}
		if !snapshotNameRE.MatchString(res.Snapshot) {
			t.Fatalf("unexpected snapshot name format: %q", res.Snapshot)
		}
		snapshots[i] = res.Snapshot
		i++
	}
	for range numSnapshots {
		res := sut.APIV1AdminTSDBSnapshot(t)
		if got, want := res.Status, "success"; got != want {
			t.Fatalf("unexpected snapshot creation status: got %q, want %q", got, want)
		}
		if !snapshotNameRE.MatchString(res.Data.Name) {
			t.Fatalf("unexpected snapshot name format: %q", res.Data.Name)
		}
		snapshots[i] = res.Data.Name
		i++
	}

	assertSnapshotList := func(want []string) {
		gotRes := sut.SnapshotList(t)
		wantRes := &at.SnapshotListResponse{
			Status:    "ok",
			Snapshots: want,
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("unexpected response (-want, +got):\n%s", diff)
		}
	}
	assertSnapshotList(snapshots)

	// Delete non-existent snapshot.
	gotDeletedSnapshot := sut.SnapshotDelete(t, "does-not-exist")
	wantDeletedSnapshot := &at.SnapshotDeleteResponse{
		Status: "error",
		Msg:    `cannot find snapshot "does-not-exist"`,
	}
	if diff := cmp.Diff(wantDeletedSnapshot, gotDeletedSnapshot); diff != "" {
		t.Fatalf("unexpected response (-want, +got):\n%s", diff)
	}

	// Delete the first snapshot.
	gotDeletedSnapshot = sut.SnapshotDelete(t, snapshots[0])
	wantDeletedSnapshot = &at.SnapshotDeleteResponse{
		Status: "ok",
	}
	if diff := cmp.Diff(wantDeletedSnapshot, gotDeletedSnapshot); diff != "" {
		t.Fatalf("unexpected response (-want, +got):\n%s", diff)
	}
	assertSnapshotList(snapshots[1:])

	// Delete the rest of the snapshots.
	gotDeleteAllRes := sut.SnapshotDeleteAll(t)
	wantDeleteAllRes := &at.SnapshotDeleteAllResponse{
		Status: "ok",
	}
	if diff := cmp.Diff(wantDeleteAllRes, gotDeleteAllRes); diff != "" {
		t.Fatalf("unexpected response (-want, +got):\n%s", diff)
	}
	assertSnapshotList([]string{})
}

func TestClusterSnapshots_CreateListDelete(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultCluster().(*at.Vmcluster)

	// Insert some data.
	const numSamples = 1000
	samples := make([]string, numSamples)
	for i := range numSamples {
		samples[i] = fmt.Sprintf("metric_%03d %d", i, i)
	}
	sut.PrometheusAPIV1ImportPrometheus(t, samples, at.QueryOpts{})
	sut.ForceFlush(t)

	// Create several snapshots for both vmstorage replicas using
	// VictoriaMetrics endpoints only (cluster version does not have Prometheus
	// endpoint)
	createSnapshot := func(i int) string {
		t.Helper()

		res := sut.Vmstorages[i].SnapshotCreate(t)
		if got, want := res.Status, "ok"; got != want {
			t.Fatalf("unexpected snapshot creation status: got %q, want %q", got, want)
		}
		if !snapshotNameRE.MatchString(res.Snapshot) {
			t.Fatalf("unexpected snapshot name format: %q", res.Snapshot)
		}
		return res.Snapshot
	}
	const numSnapshots = 4
	snapshots0 := make([]string, numSnapshots)
	snapshots1 := make([]string, numSnapshots)
	for i := range numSnapshots {
		snapshots0[i] = createSnapshot(0)
		snapshots1[i] = createSnapshot(1)
	}

	assertSnapshotList := func(i int, wantNames []string) {
		t.Helper()
		got := sut.Vmstorages[i].SnapshotList(t)
		want := &at.SnapshotListResponse{
			Status:    "ok",
			Snapshots: wantNames,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected response (-want, +got):\n%s", diff)
		}
	}
	assertSnapshotList(0, snapshots0)
	assertSnapshotList(1, snapshots1)

	// Delete non-existent snapshot.
	assertDeleteNonExistent := func(i int) {
		t.Helper()
		got := sut.Vmstorages[i].SnapshotDelete(t, "does-not-exist")
		want := &at.SnapshotDeleteResponse{
			Status: "error",
			Msg:    `cannot find snapshot "does-not-exist"`,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected response (-want, +got):\n%s", diff)
		}
	}
	assertDeleteNonExistent(0)
	assertDeleteNonExistent(1)

	// Delete the first snapshot.
	deleteSnapshot := func(i int, snapshotName string) {
		t.Helper()
		got := sut.Vmstorages[i].SnapshotDelete(t, snapshotName)
		want := &at.SnapshotDeleteResponse{
			Status: "ok",
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected response (-want, +got):\n%s", diff)
		}
	}
	deleteSnapshot(0, snapshots0[0])
	assertSnapshotList(0, snapshots0[1:])
	deleteSnapshot(1, snapshots1[0])
	assertSnapshotList(1, snapshots1[1:])

	// Delete the rest of the snapshots.
	deleteAllSnapshots := func(i int) {
		t.Helper()
		got := sut.Vmstorages[i].SnapshotDeleteAll(t)
		want := &at.SnapshotDeleteAllResponse{
			Status: "ok",
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected response (-want, +got):\n%s", diff)
		}
	}
	deleteAllSnapshots(0)
	assertSnapshotList(0, []string{})
	deleteAllSnapshots(1)
	assertSnapshotList(1, []string{})
}
