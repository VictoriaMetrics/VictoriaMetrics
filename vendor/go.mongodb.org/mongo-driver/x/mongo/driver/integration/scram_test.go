package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/internal/testutil"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"
)

type scramTestCase struct {
	username    string
	password    string
	mechanisms  []string
	altPassword string
}

func TestSCRAM(t *testing.T) {
	if os.Getenv("AUTH") != "auth" {
		t.Skip("Skipping because authentication is required")
	}

	server, err := testutil.Topology(t).SelectServerLegacy(context.Background(), description.WriteSelector())
	noerr(t, err)

	if !server.Description().WireVersion.Includes(7) {
		t.Skip("Skipping because MongoDB 4.0 is needed for SCRAM-SHA-256")
	}

	// Unicode constants for testing
	var romanFour = "\u2163" // ROMAN NUMERAL FOUR -> SASL prepped is "IV"
	var romanNine = "\u2168" // ROMAN NUMERAL NINE -> SASL prepped is "IX"

	testUsers := []scramTestCase{
		// SCRAM spec test steps 1-3
		{username: "sha1", password: "sha1", mechanisms: []string{"SCRAM-SHA-1"}},
		{username: "sha256", password: "sha256", mechanisms: []string{"SCRAM-SHA-256"}},
		{username: "both", password: "both", mechanisms: []string{"SCRAM-SHA-1", "SCRAM-SHA-256"}},
		// SCRAM spec test step 4
		{username: "IX", password: "IX", mechanisms: []string{"SCRAM-SHA-256"}, altPassword: "I\u00ADX"},
		{username: romanNine, password: romanFour, mechanisms: []string{"SCRAM-SHA-256"}, altPassword: "I\u00ADV"},
	}

	// Verify that test (root) user is authenticated.  If this fails, the
	// rest of the test can't succeed.
	wc := writeconcern.New(writeconcern.WMajority())
	collOne := testutil.ColName(t)
	testutil.DropCollection(t, testutil.DBName(t), collOne)
	testutil.InsertDocs(t, testutil.DBName(t),
		collOne, wc, bsoncore.BuildDocument(nil, bsoncore.AppendStringElement(nil, "name", "scram_test")),
	)

	// Test step 1: Create users for test cases
	err = createScramUsers(t, server.Server, testUsers)
	if err != nil {
		t.Fatal(err)
	}

	// Step 2 and 3a: For each auth mechanism, "SCRAM-SHA-1", "SCRAM-SHA-256"
	// and "negotiate" (a fake, placeholder mechanism), iterate over each user
	// and ensure that each mechanism that should succeed does so and each
	// that should fail does so.
	for _, m := range []string{"SCRAM-SHA-1", "SCRAM-SHA-256", "negotiate"} {
		for _, c := range testUsers {
			t.Run(
				fmt.Sprintf("%s %s", c.username, m),
				func(t *testing.T) {
					err := testScramUserAuthWithMech(t, c, m)
					if m == "negotiate" || hasAuthMech(c.mechanisms, m) {
						noerr(t, err)
					} else {
						autherr(t, err)
					}
				},
			)
		}
	}

	// Step 3b: test non-existing user with negotiation fails with
	// an auth.Error type.
	bogus := scramTestCase{username: "eliot", password: "trustno1"}
	err = testScramUserAuthWithMech(t, bogus, "negotiate")
	autherr(t, err)

	// XXX Step 4: test alternate password forms
	for _, c := range testUsers {
		if c.altPassword == "" {
			continue
		}
		c.password = c.altPassword
		t.Run(
			fmt.Sprintf("%s alternate password", c.username),
			func(t *testing.T) {
				err := testScramUserAuthWithMech(t, c, "SCRAM-SHA-256")
				noerr(t, err)
			},
		)
	}

}

func hasAuthMech(mechs []string, m string) bool {
	for _, v := range mechs {
		if v == m {
			return true
		}
	}
	return false
}

func testScramUserAuthWithMech(t *testing.T, c scramTestCase, mech string) error {
	t.Helper()
	cs := testutil.ConnString(t)
	cs.Username = c.username
	cs.Password = c.password
	cs.AuthSource = testutil.DBName(t)
	switch mech {
	case "negotiate":
		cs.AuthMechanism = ""
	default:
		cs.AuthMechanism = mech
	}
	return runScramAuthTest(t, cs)
}

func runScramAuthTest(t *testing.T, cs connstring.ConnString) error {
	t.Helper()
	topology := testutil.TopologyWithConnString(t, cs)
	ss, err := topology.SelectServerLegacy(context.Background(), description.WriteSelector())
	noerr(t, err)

	cmd := bsoncore.BuildDocument(nil, bsoncore.AppendInt32Element(nil, "dbstats", 1))
	_, err = testutil.RunCommand(t, ss.Server, testutil.DBName(t), cmd)
	return err
}

func createScramUsers(t *testing.T, s *topology.Server, cases []scramTestCase) error {
	db := testutil.DBName(t)
	for _, c := range cases {
		var values []bsoncore.Value
		for _, v := range c.mechanisms {
			values = append(values, bsoncore.Value{Type: bsontype.String, Data: bsoncore.AppendString(nil, v)})
		}
		newUserCmd := bsoncore.BuildDocumentFromElements(nil,
			bsoncore.AppendStringElement(nil, "createUser", c.username),
			bsoncore.AppendStringElement(nil, "pwd", c.password),
			bsoncore.AppendArrayElement(nil, "roles", bsoncore.BuildArray(nil,
				bsoncore.Value{Type: bsontype.EmbeddedDocument, Data: bsoncore.BuildDocumentFromElements(nil,
					bsoncore.AppendStringElement(nil, "role", "readWrite"),
					bsoncore.AppendStringElement(nil, "db", db),
				)},
			)),
			bsoncore.AppendArrayElement(nil, "mechanisms", bsoncore.BuildArray(nil, values...)),
		)
		_, err := testutil.RunCommand(t, s, db, newUserCmd)
		if err != nil {
			return fmt.Errorf("Couldn't create user '%s' on db '%s': %v", c.username, testutil.DBName(t), err)
		}
	}
	return nil
}
