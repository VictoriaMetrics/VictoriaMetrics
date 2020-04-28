package options

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/internal"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

var tClientOptions = reflect.TypeOf(&ClientOptions{})

func TestClientOptions(t *testing.T) {
	t.Run("ApplyURI/doesn't overwrite previous errors", func(t *testing.T) {
		uri := "not-mongo-db-uri://"
		want := internal.WrapErrorf(
			errors.New(`scheme must be "mongodb" or "mongodb+srv"`), "error parsing uri",
		)
		co := Client().ApplyURI(uri).ApplyURI("mongodb://localhost/")
		got := co.Validate()
		if !cmp.Equal(got, want, cmp.Comparer(compareErrors)) {
			t.Errorf("Did not received expected error. got %v; want %v", got, want)
		}
	})
	t.Run("Validate/returns error", func(t *testing.T) {
		want := errors.New("validate error")
		co := &ClientOptions{err: want}
		got := co.Validate()
		if !cmp.Equal(got, want, cmp.Comparer(compareErrors)) {
			t.Errorf("Did not receive expected error. got %v; want %v", got, want)
		}
	})
	t.Run("Set", func(t *testing.T) {
		testCases := []struct {
			name        string
			fn          interface{} // method to be run
			arg         interface{} // argument for method
			field       string      // field to be set
			dereference bool        // Should we compare a pointer or the field
		}{
			{"AppName", (*ClientOptions).SetAppName, "example-application", "AppName", true},
			{"Auth", (*ClientOptions).SetAuth, Credential{Username: "foo", Password: "bar"}, "Auth", true},
			{"Compressors", (*ClientOptions).SetCompressors, []string{"zstd", "snappy", "zlib"}, "Compressors", true},
			{"ConnectTimeout", (*ClientOptions).SetConnectTimeout, 5 * time.Second, "ConnectTimeout", true},
			{"Dialer", (*ClientOptions).SetDialer, testDialer{Num: 12345}, "Dialer", true},
			{"HeartbeatInterval", (*ClientOptions).SetHeartbeatInterval, 5 * time.Second, "HeartbeatInterval", true},
			{"Hosts", (*ClientOptions).SetHosts, []string{"localhost:27017", "localhost:27018", "localhost:27019"}, "Hosts", true},
			{"LocalThreshold", (*ClientOptions).SetLocalThreshold, 5 * time.Second, "LocalThreshold", true},
			{"MaxConnIdleTime", (*ClientOptions).SetMaxConnIdleTime, 5 * time.Second, "MaxConnIdleTime", true},
			{"MaxPoolSize", (*ClientOptions).SetMaxPoolSize, uint64(250), "MaxPoolSize", true},
			{"MinPoolSize", (*ClientOptions).SetMinPoolSize, uint64(10), "MinPoolSize", true},
			{"PoolMonitor", (*ClientOptions).SetPoolMonitor, &event.PoolMonitor{}, "PoolMonitor", false},
			{"Monitor", (*ClientOptions).SetMonitor, &event.CommandMonitor{}, "Monitor", false},
			{"ReadConcern", (*ClientOptions).SetReadConcern, readconcern.Majority(), "ReadConcern", false},
			{"ReadPreference", (*ClientOptions).SetReadPreference, readpref.SecondaryPreferred(), "ReadPreference", false},
			{"Registry", (*ClientOptions).SetRegistry, bson.NewRegistryBuilder().Build(), "Registry", false},
			{"ReplicaSet", (*ClientOptions).SetReplicaSet, "example-replicaset", "ReplicaSet", true},
			{"RetryWrites", (*ClientOptions).SetRetryWrites, true, "RetryWrites", true},
			{"ServerSelectionTimeout", (*ClientOptions).SetServerSelectionTimeout, 5 * time.Second, "ServerSelectionTimeout", true},
			{"Direct", (*ClientOptions).SetDirect, true, "Direct", true},
			{"SocketTimeout", (*ClientOptions).SetSocketTimeout, 5 * time.Second, "SocketTimeout", true},
			{"TLSConfig", (*ClientOptions).SetTLSConfig, &tls.Config{}, "TLSConfig", false},
			{"WriteConcern", (*ClientOptions).SetWriteConcern, writeconcern.New(writeconcern.WMajority()), "WriteConcern", false},
			{"ZlibLevel", (*ClientOptions).SetZlibLevel, 6, "ZlibLevel", true},
		}

		opt1, opt2, optResult := Client(), Client(), Client()
		for idx, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				fn := reflect.ValueOf(tc.fn)
				if fn.Kind() != reflect.Func {
					t.Fatal("fn argument must be a function")
				}
				if fn.Type().NumIn() < 2 || fn.Type().In(0) != tClientOptions {
					t.Fatal("fn argument must have a *ClientOptions as the first argument and one other argument")
				}
				if _, exists := tClientOptions.Elem().FieldByName(tc.field); !exists {
					t.Fatalf("field (%s) does not exist in ClientOptions", tc.field)
				}
				args := make([]reflect.Value, 2)
				client := reflect.New(tClientOptions.Elem())
				args[0] = client
				want := reflect.ValueOf(tc.arg)
				args[1] = want

				if !want.IsValid() || !want.CanInterface() {
					t.Fatal("arg property of test case must be valid")
				}

				_ = fn.Call(args)

				// To avoid duplication we're piggybacking on the Set* tests to make the
				// MergeClientOptions test simpler and more thorough.
				// To do this we set the odd numbered test cases to the first opt, the even and
				// divisible by three test cases to the second, and the result of merging the two to
				// the result option. This gives us coverage of options set by the first option, by
				// the second, and by both.
				if idx%2 != 0 {
					args[0] = reflect.ValueOf(opt1)
					_ = fn.Call(args)
				}
				if idx%2 == 0 || idx%3 == 0 {
					args[0] = reflect.ValueOf(opt2)
					_ = fn.Call(args)
				}
				args[0] = reflect.ValueOf(optResult)
				_ = fn.Call(args)

				got := client.Elem().FieldByName(tc.field)
				if !got.IsValid() || !got.CanInterface() {
					t.Fatal("cannot create concrete instance from retrieved field")
				}

				if got.Kind() == reflect.Ptr && tc.dereference {
					got = got.Elem()
				}

				if !cmp.Equal(
					got.Interface(), want.Interface(),
					cmp.AllowUnexported(readconcern.ReadConcern{}, writeconcern.WriteConcern{}, readpref.ReadPref{}),
					cmp.Comparer(func(r1, r2 *bsoncodec.Registry) bool { return r1 == r2 }),
					cmp.Comparer(func(cfg1, cfg2 *tls.Config) bool { return cfg1 == cfg2 }),
					cmp.Comparer(func(fp1, fp2 *event.PoolMonitor) bool { return fp1 == fp2 }),
				) {
					t.Errorf("Field not set properly. got %v; want %v", got.Interface(), want.Interface())
				}
			})
		}
		t.Run("MergeClientOptions/all set", func(t *testing.T) {
			want := optResult
			got := MergeClientOptions(nil, opt1, opt2)
			if diff := cmp.Diff(
				got, want,
				cmp.AllowUnexported(readconcern.ReadConcern{}, writeconcern.WriteConcern{}, readpref.ReadPref{}),
				cmp.Comparer(func(r1, r2 *bsoncodec.Registry) bool { return r1 == r2 }),
				cmp.Comparer(func(cfg1, cfg2 *tls.Config) bool { return cfg1 == cfg2 }),
				cmp.Comparer(func(fp1, fp2 *event.PoolMonitor) bool { return fp1 == fp2 }),
				cmp.AllowUnexported(ClientOptions{}),
			); diff != "" {
				t.Errorf("diff:\n%s", diff)
				t.Errorf("Merged client options do not match. got %v; want %v", got, want)
			}
		})

		// go-cmp dont support error comparisons (https://github.com/google/go-cmp/issues/24)
		// Use specifique test for this
		t.Run("MergeClientOptions/err", func(t *testing.T) {
			opt1, opt2 := Client(), Client()
			opt1.err = errors.New("Test error")

			got := MergeClientOptions(nil, opt1, opt2)
			if got.err.Error() != "Test error" {
				t.Errorf("Merged client options do not match. got %v; want %v", got.err.Error(), opt1.err.Error())
			}
		})
	})
	t.Run("ApplyURI", func(t *testing.T) {
		baseClient := func() *ClientOptions {
			return Client().SetHosts([]string{"localhost"})
		}
		testCases := []struct {
			name   string
			uri    string
			result *ClientOptions
		}{
			{
				"ParseError",
				"not-mongo-db-uri://",
				&ClientOptions{err: internal.WrapErrorf(
					errors.New(`scheme must be "mongodb" or "mongodb+srv"`), "error parsing uri",
				)},
			},
			{
				"ReadPreference Invalid Mode",
				"mongodb://localhost/?maxStaleness=200",
				&ClientOptions{
					err:   fmt.Errorf("unknown read preference %v", ""),
					Hosts: []string{"localhost"},
				},
			},
			{
				"ReadPreference Primary With Options",
				"mongodb://localhost/?readPreference=Primary&maxStaleness=200",
				&ClientOptions{
					err:   errors.New("can not specify tags or max staleness on primary"),
					Hosts: []string{"localhost"},
				},
			},
			{
				"TLS addCertFromFile error",
				"mongodb://localhost/?ssl=true&sslCertificateAuthorityFile=testdata/doesntexist",
				&ClientOptions{
					err:   &os.PathError{Op: "open", Path: "testdata/doesntexist"},
					Hosts: []string{"localhost"},
				},
			},
			{
				"TLS ClientCertificateKey",
				"mongodb://localhost/?ssl=true&sslClientCertificateKeyFile=testdata/doesntexist",
				&ClientOptions{
					err:   &os.PathError{Op: "open", Path: "testdata/doesntexist"},
					Hosts: []string{"localhost"},
				},
			},
			{
				"AppName",
				"mongodb://localhost/?appName=awesome-example-application",
				baseClient().SetAppName("awesome-example-application"),
			},
			{
				"AuthMechanism",
				"mongodb://localhost/?authMechanism=mongodb-x509",
				baseClient().SetAuth(Credential{AuthSource: "$external", AuthMechanism: "mongodb-x509"}),
			},
			{
				"AuthMechanismProperties",
				"mongodb://foo@localhost/?authMechanism=gssapi&authMechanismProperties=SERVICE_NAME:mongodb-fake",
				baseClient().SetAuth(Credential{
					AuthSource:              "$external",
					AuthMechanism:           "gssapi",
					AuthMechanismProperties: map[string]string{"SERVICE_NAME": "mongodb-fake"},
					Username:                "foo",
				}),
			},
			{
				"AuthSourceNoUsername",
				"mongodb://localhost/?authSource=random-database-example",
				&ClientOptions{err: internal.WrapErrorf(
					errors.New("authsource without username is invalid"), "error parsing uri",
				)},
			},
			{
				"AuthSource",
				"mongodb://foo@localhost/?authSource=random-database-example",
				baseClient().SetAuth(Credential{AuthSource: "random-database-example", Username: "foo"}),
			},
			{
				"Username",
				"mongodb://foo@localhost/",
				baseClient().SetAuth(Credential{AuthSource: "admin", Username: "foo"}),
			},
			{
				"Password",
				"mongodb://foo:bar@localhost/",
				baseClient().SetAuth(Credential{
					AuthSource: "admin", Username: "foo",
					Password: "bar", PasswordSet: true,
				}),
			},
			{
				"Connect",
				"mongodb://localhost/?connect=direct",
				baseClient().SetDirect(true),
			},
			{
				"ConnectTimeout",
				"mongodb://localhost/?connectTimeoutms=5000",
				baseClient().SetConnectTimeout(5 * time.Second),
			},
			{
				"Compressors",
				"mongodb://localhost/?compressors=zlib,snappy",
				baseClient().SetCompressors([]string{"zlib", "snappy"}).SetZlibLevel(6),
			},
			{
				"DatabaseNoAuth",
				"mongodb://localhost/example-database",
				baseClient(),
			},
			{
				"DatabaseAsDefault",
				"mongodb://foo@localhost/example-database",
				baseClient().SetAuth(Credential{AuthSource: "example-database", Username: "foo"}),
			},
			{
				"HeartbeatInterval",
				"mongodb://localhost/?heartbeatIntervalms=12000",
				baseClient().SetHeartbeatInterval(12 * time.Second),
			},
			{
				"Hosts",
				"mongodb://localhost:27017,localhost:27018,localhost:27019/",
				baseClient().SetHosts([]string{"localhost:27017", "localhost:27018", "localhost:27019"}),
			},
			{
				"LocalThreshold",
				"mongodb://localhost/?localThresholdMS=200",
				baseClient().SetLocalThreshold(200 * time.Millisecond),
			},
			{
				"MaxConnIdleTime",
				"mongodb://localhost/?maxIdleTimeMS=300000",
				baseClient().SetMaxConnIdleTime(5 * time.Minute),
			},
			{
				"MaxPoolSize",
				"mongodb://localhost/?maxPoolSize=256",
				baseClient().SetMaxPoolSize(256),
			},
			{
				"ReadConcern",
				"mongodb://localhost/?readConcernLevel=linearizable",
				baseClient().SetReadConcern(readconcern.Linearizable()),
			},
			{
				"ReadPreference",
				"mongodb://localhost/?readPreference=secondaryPreferred",
				baseClient().SetReadPreference(readpref.SecondaryPreferred()),
			},
			{
				"ReadPreferenceTagSets",
				"mongodb://localhost/?readPreference=secondaryPreferred&readPreferenceTags=foo:bar",
				baseClient().SetReadPreference(readpref.SecondaryPreferred(readpref.WithTags("foo", "bar"))),
			},
			{
				"MaxStaleness",
				"mongodb://localhost/?readPreference=secondaryPreferred&maxStaleness=250",
				baseClient().SetReadPreference(readpref.SecondaryPreferred(readpref.WithMaxStaleness(250 * time.Second))),
			},
			{
				"RetryWrites",
				"mongodb://localhost/?retryWrites=true",
				baseClient().SetRetryWrites(true),
			},
			{
				"ReplicaSet",
				"mongodb://localhost/?replicaSet=rs01",
				baseClient().SetReplicaSet("rs01"),
			},
			{
				"ServerSelectionTimeout",
				"mongodb://localhost/?serverSelectionTimeoutMS=45000",
				baseClient().SetServerSelectionTimeout(45 * time.Second),
			},
			{
				"SocketTimeout",
				"mongodb://localhost/?socketTimeoutMS=15000",
				baseClient().SetSocketTimeout(15 * time.Second),
			},
			{
				"TLS CACertificate",
				"mongodb://localhost/?ssl=true&sslCertificateAuthorityFile=testdata/ca.pem",
				baseClient().SetTLSConfig(&tls.Config{RootCAs: x509.NewCertPool()}),
			},
			{
				"TLS Insecure",
				"mongodb://localhost/?ssl=true&sslInsecure=true",
				baseClient().SetTLSConfig(&tls.Config{InsecureSkipVerify: true}),
			},
			{
				"TLS ClientCertificateKey",
				"mongodb://localhost/?ssl=true&sslClientCertificateKeyFile=testdata/nopass/certificate.pem",
				baseClient().SetTLSConfig(&tls.Config{Certificates: make([]tls.Certificate, 1)}),
			},
			{
				"TLS ClientCertificateKey with password",
				"mongodb://localhost/?ssl=true&sslClientCertificateKeyFile=testdata/certificate.pem&sslClientCertificateKeyPassword=passphrase",
				baseClient().SetTLSConfig(&tls.Config{Certificates: make([]tls.Certificate, 1)}),
			},
			{
				"TLS Username",
				"mongodb://localhost/?ssl=true&authMechanism=mongodb-x509&sslClientCertificateKeyFile=testdata/nopass/certificate.pem",
				baseClient().SetAuth(Credential{
					AuthMechanism: "mongodb-x509", AuthSource: "$external",
					Username: `C=US,ST=New York,L=New York City, Inc,O=MongoDB\,OU=WWW`,
				}),
			},
			{
				"WriteConcern J",
				"mongodb://localhost/?journal=true",
				baseClient().SetWriteConcern(writeconcern.New(writeconcern.J(true))),
			},
			{
				"WriteConcern WString",
				"mongodb://localhost/?w=majority",
				baseClient().SetWriteConcern(writeconcern.New(writeconcern.WMajority())),
			},
			{
				"WriteConcern W",
				"mongodb://localhost/?w=3",
				baseClient().SetWriteConcern(writeconcern.New(writeconcern.W(3))),
			},
			{
				"WriteConcern WTimeout",
				"mongodb://localhost/?wTimeoutMS=45000",
				baseClient().SetWriteConcern(writeconcern.New(writeconcern.WTimeout(45 * time.Second))),
			},
			{
				"ZLibLevel",
				"mongodb://localhost/?zlibCompressionLevel=4",
				baseClient().SetZlibLevel(4),
			},
			{
				"TLS tlsCertificateFile and tlsPrivateKeyFile",
				"mongodb://localhost/?tlsCertificateFile=testdata/nopass/cert.pem&tlsPrivateKeyFile=testdata/nopass/key.pem",
				baseClient().SetTLSConfig(&tls.Config{Certificates: make([]tls.Certificate, 1)}),
			},
			{
				"TLS only tlsCertificateFile",
				"mongodb://localhost/?tlsCertificateFile=testdata/nopass/cert.pem",
				&ClientOptions{err: internal.WrapErrorf(
					errors.New("the tlsPrivateKeyFile URI option must be provided if the tlsCertificateFile option is specified"),
					"error parsing uri",
				)},
			},
			{
				"TLS only tlsPrivateKeyFile",
				"mongodb://localhost/?tlsPrivateKeyFile=testdata/nopass/key.pem",
				&ClientOptions{err: internal.WrapErrorf(
					errors.New("the tlsCertificateFile URI option must be provided if the tlsPrivateKeyFile option is specified"),
					"error parsing uri",
				)},
			},
			{
				"TLS tlsCertificateFile and tlsPrivateKeyFile and tlsCertificateKeyFile",
				"mongodb://localhost/?tlsCertificateFile=testdata/nopass/cert.pem&tlsPrivateKeyFile=testdata/nopass/key.pem&tlsCertificateKeyFile=testdata/nopass/certificate.pem",
				&ClientOptions{err: internal.WrapErrorf(
					errors.New("the sslClientCertificateKeyFile/tlsCertificateKeyFile URI option cannot be provided "+
						"along with tlsCertificateFile or tlsPrivateKeyFile"),
					"error parsing uri",
				)},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := Client().ApplyURI(tc.uri)
				tc.result.uri = tc.uri // manually add URI to avoid writing it in each test
				if diff := cmp.Diff(
					tc.result, result,
					cmp.AllowUnexported(readconcern.ReadConcern{}, writeconcern.WriteConcern{}, readpref.ReadPref{}),
					cmp.Comparer(func(r1, r2 *bsoncodec.Registry) bool { return r1 == r2 }),
					cmp.Comparer(compareTLSConfig),
					cmp.Comparer(compareErrors),
					cmp.AllowUnexported(ClientOptions{}),
				); diff != "" {
					t.Errorf("URI did not apply correctly: (-want +got)\n%s", diff)
				}
			})
		}
	})
}

type testDialer struct {
	Num int
}

func (testDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return nil, nil
}

func compareTLSConfig(cfg1, cfg2 *tls.Config) bool {
	if cfg1 == nil && cfg2 == nil {
		return true
	}

	if cfg1 == nil || cfg2 == nil {
		return true
	}

	if (cfg1.RootCAs == nil && cfg1.RootCAs != nil) || (cfg1.RootCAs != nil && cfg1.RootCAs == nil) {
		return false
	}

	if len(cfg1.Certificates) != len(cfg2.Certificates) {
		return false
	}

	if cfg1.InsecureSkipVerify != cfg2.InsecureSkipVerify {
		return false
	}

	return true
}

func compareErrors(err1, err2 error) bool {
	if err1 == nil && err2 == nil {
		return true
	}

	if err1 == nil || err2 == nil {
		return false
	}

	ospe1, ok1 := err1.(*os.PathError)
	ospe2, ok2 := err2.(*os.PathError)
	if ok1 && ok2 {
		return ospe1.Op == ospe2.Op && ospe1.Path == ospe2.Path
	}

	if err1.Error() != err2.Error() {
		return false
	}

	return true
}
