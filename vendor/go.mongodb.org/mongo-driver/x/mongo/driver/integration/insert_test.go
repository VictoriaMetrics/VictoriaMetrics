package integration

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/operation"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"
)

func TestInsert(t *testing.T) {
	t.Skip()
	var connOpts []topology.ConnectionOption
	var serverOpts []topology.ServerOption
	var topoOpts []topology.Option

	connOpts = append(connOpts, topology.WithHandshaker(func(h driver.Handshaker) driver.Handshaker {
		return operation.NewIsMaster().AppName("operationgen-test")
	}))
	// topoOpts = append(topoOpts, topology.WithServerSelectionTimeout(func(time.Duration) time.Duration { return 5 * time.Second }))
	serverOpts = append(serverOpts, topology.WithConnectionOptions(func(opts ...topology.ConnectionOption) []topology.ConnectionOption {
		return append(opts, connOpts...)
	}))
	topoOpts = append(topoOpts, topology.WithServerOptions(func(opts ...topology.ServerOption) []topology.ServerOption {
		return append(opts, serverOpts...)
	}))
	topo, err := topology.New(topoOpts...)
	if err != nil {
		t.Fatalf("Couldn't connect topology: %v", err)
	}
	_ = topo.Connect()

	doc := bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159))

	iop := operation.NewInsert(doc).Database("foo").Collection("bar").Deployment(topo)
	err = iop.Execute(context.Background())
	if err != nil {
		t.Fatalf("Couldn't execute insert operation: %v", err)
	}
	t.Log(iop.Result())

	fop := operation.NewFind(bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159))).
		Database("foo").Collection("bar").Deployment(topo).BatchSize(1)
	err = fop.Execute(context.Background())
	if err != nil {
		t.Fatalf("Couldn't execute find operation: %v", err)
	}
	cur, err := fop.Result(driver.CursorOptions{BatchSize: 2})
	if err != nil {
		t.Fatalf("Couldn't get cursor result from find operation: %v", err)
	}
	for cur.Next(context.Background()) {
		batch := cur.Batch()
		docs, err := batch.Documents()
		if err != nil {
			t.Fatalf("Couldn't iterate batch: %v", err)
		}
		for i, doc := range docs {
			t.Log(i, doc)
		}
	}
}
