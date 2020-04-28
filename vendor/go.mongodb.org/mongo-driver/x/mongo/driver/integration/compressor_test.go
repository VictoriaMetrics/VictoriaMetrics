package integration

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/operation"
)

func TestCompression(t *testing.T) {
	comp := os.Getenv("MONGO_GO_DRIVER_COMPRESSOR")
	if len(comp) == 0 {
		t.Skip("Skipping because no compressor specified")
	}

	wc := writeconcern.New(writeconcern.WMajority())
	collOne := testutil.ColName(t)

	testutil.DropCollection(t, testutil.DBName(t), collOne)
	testutil.InsertDocs(t, testutil.DBName(t), collOne, wc,
		bsoncore.BuildDocument(nil, bsoncore.AppendStringElement(nil, "name", "compression_test")),
	)

	cmd := operation.NewCommand(bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "serverStatus", 1))).
		Deployment(testutil.Topology(t)).
		Database(testutil.DBName(t))

	ctx := context.Background()
	err := cmd.Execute(ctx)
	noerr(t, err)
	result := cmd.Result()

	serverVersion, err := result.LookupErr("version")
	noerr(t, err)

	if testutil.CompareVersions(t, serverVersion.StringValue(), "3.4") < 0 {
		t.Skip("skipping compression test for version < 3.4")
	}

	networkVal, err := result.LookupErr("network")
	noerr(t, err)

	require.Equal(t, networkVal.Type, bson.TypeEmbeddedDocument)

	compressionVal, err := networkVal.Document().LookupErr("compression")
	noerr(t, err)

	compressorDoc, err := compressionVal.Document().LookupErr(comp)
	noerr(t, err)

	compressorKey := "compressor"
	compareTo36 := testutil.CompareVersions(t, serverVersion.StringValue(), "3.6")
	if compareTo36 < 0 {
		compressorKey = "compressed"
	}
	compressor, err := compressorDoc.Document().LookupErr(compressorKey)
	noerr(t, err)

	bytesIn, err := compressor.Document().LookupErr("bytesIn")
	noerr(t, err)

	require.True(t, bytesIn.IsNumber())
	require.True(t, bytesIn.Int64() > 0)
}
