package tests

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// TestSingleVlagentRemoteWrite performs tests for remote write data ingestion
// by vlagent application
func TestSingleVlagentRemoteWrite(t *testing.T) {
	os.RemoveAll(t.Name())
	tc := at.NewTestCase(t)
	defer tc.Stop()

	// test data ingestion into
	const instance = "vlsingle"
	const r1Port = "50425"
	sutFlags := []string{
		"-httpListenAddr=127.0.0.1:" + r1Port,
		"-storageDataPath=" + tc.Dir() + "/" + instance,
		"-retentionPeriod=100y",
	}

	sut := tc.MustStartVlsingle(instance, sutFlags)
	remoteWriteURL := fmt.Sprintf("http://%s/internal/insert", sut.HTTPAddr())

	vlagent := tc.MustStartDefaultVlagent([]string{remoteWriteURL})
	vlagent.JSONLineWrite(t, []string{
		`{"_msg":"ingest jsonline","_time": "2025-06-05T14:30:19.088007Z", "foo":"bar"}`,
		`{"_msg":"ingest jsonline","_time": "2025-06-05T14:30:19.088007Z", "bar":"foo"}`,
	}, at.QueryOptsLogs{})

	sut.ForceFlush(t)
	got := sut.LogsQLQuery(t, "ingest jsonline", at.QueryOptsLogs{})
	wantLogLines := []string{
		`{"_msg":"ingest jsonline","_stream":"{}","_time":"2025-06-05T14:30:19.088007Z","bar":"foo"}`,
		`{"_msg":"ingest jsonline","_stream":"{}","_time":"2025-06-05T14:30:19.088007Z","foo":"bar"}`,
	}
	assertLogsQLResponseEqual(t, got, &at.LogsQLQueryResponse{LogLines: wantLogLines})

	// stop log storage and check data buffering works correctly
	tc.StopApp(instance)

	// ingest some data vlagent must hold it in memory
	vlagent.JSONLineWrite(t, []string{
		`{"_msg":"ingest jsonline2","_time": "2025-06-05T14:30:19.088007Z", "foo":"bar"}`,
		`{"_msg":"ingest jsonline2","_time": "2025-06-05T14:30:19.088007Z", "bar":"foo"}`,
	}, at.QueryOptsLogs{})

	vlagent.WaitQueueEmptyAfter(t, func() {
		// start storage and check if buffered data correctly ingested
		sut = tc.MustStartVlsingle(instance, sutFlags)
	})

	sut.ForceFlush(t)
	got = sut.LogsQLQuery(t, "ingest jsonline2", at.QueryOptsLogs{})
	wantLogLines = []string{
		`{"_msg":"ingest jsonline2","_stream":"{}","_time":"2025-06-05T14:30:19.088007Z","bar":"foo"}`,
		`{"_msg":"ingest jsonline2","_stream":"{}","_time":"2025-06-05T14:30:19.088007Z","foo":"bar"}`,
	}
	assertLogsQLResponseEqual(t, got, &at.LogsQLQueryResponse{LogLines: wantLogLines})
}

func TestSingleVlagentRemoteWriteReplication(t *testing.T) {
	os.RemoveAll(t.Name())
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const (
		instanceReplica0 = "vlsingle-0"
		vlsinglePortR0   = "53541"
		instanceReplica1 = "vlsingle-1"
		vlsinglePortR1   = "53124"
		vlagentInstance  = "vlagent"
	)
	sutFlagsR0 := []string{
		"-httpListenAddr=127.0.0.1:" + vlsinglePortR0,
		"-storageDataPath=" + path.Join(tc.Dir(), instanceReplica0),
		"-retentionPeriod=100y",
	}
	sutFlagsR1 := []string{
		"-httpListenAddr=127.0.0.1:" + vlsinglePortR1,
		"-storageDataPath=" + path.Join(tc.Dir(), instanceReplica1),
		"-retentionPeriod=100y",
	}

	sutR0 := tc.MustStartVlsingle(instanceReplica0, sutFlagsR0)
	sutR1 := tc.MustStartVlsingle(instanceReplica1, sutFlagsR1)

	vlagentRemoteWriteURLs := []string{
		fmt.Sprintf("http://%s/internal/insert", sutR0.HTTPAddr()),
		fmt.Sprintf("http://%s/internal/insert", sutR1.HTTPAddr()),
	}
	vlagentFlags := []string{
		"-remoteWrite.tmpDataPath=" + fmt.Sprintf("%s/%s-%d", os.TempDir(), vlagentInstance, time.Now().UnixNano()),
	}
	vlagent := tc.MustStartVlagent(vlagentInstance, vlagentRemoteWriteURLs, vlagentFlags)

	// ingest data and check if it properly replicated to the vlsingles
	vlagent.JSONLineWrite(t, []string{
		`{"_msg":"ingest jsonline","_time": "2025-06-05T14:30:19.088007Z", "foo":"bar"}`,
		`{"_msg":"ingest jsonline","_time": "2025-06-05T14:30:19.088007Z", "bar":"foo"}`,
	}, at.QueryOptsLogs{})

	wantLogLines := []string{
		`{"_msg":"ingest jsonline","_stream":"{}","_time":"2025-06-05T14:30:19.088007Z","bar":"foo"}`,
		`{"_msg":"ingest jsonline","_stream":"{}","_time":"2025-06-05T14:30:19.088007Z","foo":"bar"}`,
	}

	sutR0.ForceFlush(t)
	gotR0 := sutR0.LogsQLQuery(t, "ingest jsonline", at.QueryOptsLogs{})
	assertLogsQLResponseEqual(t, gotR0, &at.LogsQLQueryResponse{LogLines: wantLogLines})

	sutR1.ForceFlush(t)
	gotR1 := sutR1.LogsQLQuery(t, "ingest jsonline", at.QueryOptsLogs{})
	assertLogsQLResponseEqual(t, gotR1, &at.LogsQLQueryResponse{LogLines: wantLogLines})

	// stop log storage and check data buffering works correctly at vlagent
	tc.StopApp(instanceReplica0)

	// ingest some data vlagent must hold it in memory
	vlagent.JSONLineWrite(t, []string{
		`{"_msg":"ingest jsonline2","_stream":"{}","_time":"2025-06-05T14:30:19.088007Z","bar":"foo"}`,
		`{"_msg":"ingest jsonline2","_stream":"{}","_time":"2025-06-05T14:30:19.088007Z","foo":"bar"}`,
	}, at.QueryOptsLogs{})

	// check alive storage received data
	wantLogLines = []string{
		`{"_msg":"ingest jsonline2","_stream":"{}","_time":"2025-06-05T14:30:19.088007Z","bar":"foo"}`,
		`{"_msg":"ingest jsonline2","_stream":"{}","_time":"2025-06-05T14:30:19.088007Z","foo":"bar"}`,
	}

	sutR1.ForceFlush(t)
	gotR1 = sutR1.LogsQLQuery(t, "ingest jsonline2", at.QueryOptsLogs{})
	assertLogsQLResponseEqual(t, gotR1, &at.LogsQLQueryResponse{LogLines: wantLogLines})

	// stop vmagent, it must buffer data on-disk
	tc.StopApp(vlagentInstance)

	vlagent = tc.MustStartVlagent(vlagentInstance, vlagentRemoteWriteURLs, vlagentFlags)
	vlagent.WaitQueueEmptyAfter(t, func() {
		// start storage and check if buffered data correctly ingested
		sutR0 = tc.MustStartVlsingle(instanceReplica0, sutFlagsR0)
	})

	sutR0.ForceFlush(t)
	gotR0 = sutR0.LogsQLQuery(t, "ingest jsonline2", at.QueryOptsLogs{})
	assertLogsQLResponseEqual(t, gotR0, &at.LogsQLQueryResponse{LogLines: wantLogLines})
}
