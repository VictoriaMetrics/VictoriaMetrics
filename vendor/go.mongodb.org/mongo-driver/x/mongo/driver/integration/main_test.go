package integration

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/x/mongo/driver/auth"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"
)

var host *string
var connectionString connstring.ConnString
var dbName string

func TestMain(m *testing.M) {
	flag.Parse()

	mongodbURI := os.Getenv("MONGODB_URI")
	if mongodbURI == "" {
		mongodbURI = "mongodb://localhost:27017"
	}

	mongodbURI = addTLSConfigToURI(mongodbURI)
	mongodbURI = addCompressorToURI(mongodbURI)

	var err error
	connectionString, err = connstring.Parse(mongodbURI)
	if err != nil {
		fmt.Printf("Could not parse connection string: %v\n", err)
		os.Exit(1)
	}

	host = &connectionString.Hosts[0]

	dbName = fmt.Sprintf("mongo-go-driver-%d", os.Getpid())
	if connectionString.Database != "" {
		dbName = connectionString.Database
	}
	os.Exit(m.Run())
}

func noerr(t *testing.T, err error) {
	if err != nil {
		t.Helper()
		t.Errorf("Unexpected error: %v", err)
		t.FailNow()
	}
}

func autherr(t *testing.T, err error) {
	t.Helper()
	switch e := err.(type) {
	case topology.ConnectionError:
		_, ok := e.Wrapped.(*auth.Error)
		if !ok {
			t.Fatal("Expected auth error and didn't get one")
		}
	case *auth.Error:
		return
	default:
		t.Fatal("Expected auth error and didn't get one")
	}
}

// addTLSConfigToURI checks for the environmental variable indicating that the tests are being run
// on an SSL-enabled server, and if so, returns a new URI with the necessary configuration.
func addTLSConfigToURI(uri string) string {
	caFile := os.Getenv("MONGO_GO_DRIVER_CA_FILE")
	if len(caFile) == 0 {
		return uri
	}

	if !strings.ContainsRune(uri, '?') {
		if uri[len(uri)-1] != '/' {
			uri += "/"
		}

		uri += "?"
	} else {
		uri += "&"
	}

	return uri + "ssl=true&sslCertificateAuthorityFile=" + caFile
}

func addCompressorToURI(uri string) string {
	comp := os.Getenv("MONGO_GO_DRIVER_COMPRESSOR")
	if len(comp) == 0 {
		return uri
	}

	if !strings.ContainsRune(uri, '?') {
		if uri[len(uri)-1] != '/' {
			uri += "/"
		}

		uri += "?"
	} else {
		uri += "&"
	}

	return uri + "compressors=" + comp
}
