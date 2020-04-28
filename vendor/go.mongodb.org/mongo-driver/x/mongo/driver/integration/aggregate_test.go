package integration

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/operation"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"
)

func setUpMonitor() (*event.CommandMonitor, chan *event.CommandStartedEvent, chan *event.CommandSucceededEvent, chan *event.CommandFailedEvent) {
	started := make(chan *event.CommandStartedEvent, 1)
	succeeded := make(chan *event.CommandSucceededEvent, 1)
	failed := make(chan *event.CommandFailedEvent, 1)

	return &event.CommandMonitor{
		Started: func(ctx context.Context, e *event.CommandStartedEvent) {
			started <- e
		},
		Succeeded: func(ctx context.Context, e *event.CommandSucceededEvent) {
			succeeded <- e
		},
		Failed: func(ctx context.Context, e *event.CommandFailedEvent) {
			failed <- e
		},
	}, started, succeeded, failed
}

func skipIfBelow32(ctx context.Context, t *testing.T, topo *topology.Topology) {
	server, err := topo.SelectServerLegacy(ctx, description.WriteSelector())
	noerr(t, err)

	versionCmd := bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "serverStatus", 1))
	serverStatus, err := testutil.RunCommand(t, server.Server, dbName, versionCmd)
	version, err := serverStatus.LookupErr("version")

	if testutil.CompareVersions(t, version.StringValue(), "3.2") < 0 {
		t.Skip()
	}
}

func TestAggregate(t *testing.T) {
	t.Run("TestMaxTimeMSInGetMore", func(t *testing.T) {
		ctx := context.Background()
		monitor, started, succeeded, failed := setUpMonitor()
		dbName := "TestAggMaxTimeDB"
		collName := "TestAggMaxTimeColl"
		top := testutil.MonitoredTopology(t, dbName, monitor)
		clearChannels(started, succeeded, failed)
		skipIfBelow32(ctx, t, top)

		clearChannels(started, succeeded, failed)
		err := operation.NewInsert(
			bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "x", 1)),
			bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "x", 1)),
			bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "x", 1)),
		).Collection(collName).Database(dbName).
			Deployment(top).ServerSelector(description.WriteSelector()).Execute(context.Background())
		noerr(t, err)

		clearChannels(started, succeeded, failed)
		op := operation.NewAggregate(bsoncore.BuildDocumentFromElements(nil)).
			Collection(collName).Database(dbName).Deployment(top).ServerSelector(description.WriteSelector()).
			CommandMonitor(monitor).BatchSize(2)
		err = op.Execute(context.Background())
		noerr(t, err)
		batchCursor, err := op.Result(driver.CursorOptions{MaxTimeMS: 10, BatchSize: 2, CommandMonitor: monitor})
		noerr(t, err)

		var e *event.CommandStartedEvent
		select {
		case e = <-started:
		case <-time.After(2000 * time.Millisecond):
			t.Fatal("timed out waiting for aggregate")
		}

		require.Equal(t, "aggregate", e.CommandName)

		clearChannels(started, succeeded, failed)
		// first Next() should automatically return true
		require.True(t, batchCursor.Next(ctx), "expected true from first Next, got false")
		clearChannels(started, succeeded, failed)
		batchCursor.Next(ctx) // should do getMore

		select {
		case e = <-started:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timed out waiting for getMore")
		}
		require.Equal(t, "getMore", e.CommandName)
		_, err = e.Command.LookupErr("maxTimeMS")
		noerr(t, err)
	})
	t.Run("Multiple Batches", func(t *testing.T) {
		ds := []bsoncore.Document{
			bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "_id", 1)),
			bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "_id", 2)),
			bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "_id", 3)),
			bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "_id", 4)),
			bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "_id", 5)),
		}
		wc := writeconcern.New(writeconcern.WMajority())
		testutil.AutoInsertDocs(t, wc, ds...)

		op := operation.NewAggregate(bsoncore.BuildArray(nil,
			bsoncore.BuildDocumentValue(
				bsoncore.BuildDocumentElement(nil,
					"$match", bsoncore.BuildDocumentElement(nil,
						"_id", bsoncore.AppendInt32Element(nil, "$gt", 2),
					),
				),
			),
			bsoncore.BuildDocumentValue(
				bsoncore.BuildDocumentElement(nil,
					"$sort", bsoncore.AppendInt32Element(nil, "_id", 1),
				),
			),
		)).Collection(testutil.ColName(t)).Database(dbName).Deployment(testutil.Topology(t)).
			ServerSelector(description.WriteSelector()).BatchSize(2)
		err := op.Execute(context.Background())
		noerr(t, err)
		cursor, err := op.Result(driver.CursorOptions{BatchSize: 2})
		noerr(t, err)

		var got []bsoncore.Document
		for i := 0; i < 2; i++ {
			if !cursor.Next(context.Background()) {
				t.Error("Cursor should have results, but does not have a next result")
			}
			docs, err := cursor.Batch().Documents()
			noerr(t, err)
			got = append(got, docs...)
		}
		readers := ds[2:]
		for i, g := range got {
			if !bytes.Equal(g[:len(readers[i])], readers[i]) {
				t.Errorf("Did not get expected document. got %v; want %v", bson.Raw(g[:len(readers[i])]), readers[i])
			}
		}

		if cursor.Next(context.Background()) {
			t.Error("Cursor should be exhausted but has more results")
		}
	})
	t.Run("AllowDiskUse", func(t *testing.T) {
		ds := []bsoncore.Document{
			bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "_id", 1)),
			bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "_id", 2)),
		}
		wc := writeconcern.New(writeconcern.WMajority())
		testutil.AutoInsertDocs(t, wc, ds...)

		op := operation.NewAggregate(bsoncore.BuildArray(nil)).Collection(testutil.ColName(t)).Database(dbName).
			Deployment(testutil.Topology(t)).ServerSelector(description.WriteSelector()).AllowDiskUse(true)
		err := op.Execute(context.Background())
		if err != nil {
			t.Errorf("Expected no error from allowing disk use, but got %v", err)
		}
	})

}

func clearChannels(s chan *event.CommandStartedEvent, succ chan *event.CommandSucceededEvent, f chan *event.CommandFailedEvent) {
	for len(s) > 0 {
		<-s
	}
	for len(succ) > 0 {
		<-succ
	}
	for len(f) > 0 {
		<-f
	}
}
