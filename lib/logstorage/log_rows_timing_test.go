package logstorage

import (
	"testing"
)

func BenchmarkLogRowsMustAdd(b *testing.B) {
	rows := newBenchRows(map[string]string{
		"input.type":         "filestream",
		"ecs.version":        "8.0.0",
		"host.hostname":      "foobar-baz-abc",
		"host.architecture":  "x86_64",
		"host.name":          "foobar-baz-abc",
		"host.os.codename":   "bionic",
		"host.os.type":       "linux",
		"host.os.platform":   "ubuntu",
		"host.os.version":    "18.04.6 LTS (Bionic Beaver)",
		"host.os.family":     "debian",
		"host.os.name":       "Ubuntu",
		"host.os.kernel":     "4.15.0-211-generic",
		"host.id":            "a634d50249af449dbcb3ce724822568a",
		"host.containerized": "false",
		"host.ip":            `["10.0.0.42","10.224.112.1","172.20.0.1","172.18.0.1","172.19.0.1","fc00:f853:ccd:e793::1","fe80::1","172.21.0.1","172.17.0.1"]`,
		"host.mac":           `["02-42-42-90-52-D9","02-42-C6-48-A6-84","02-42-FD-91-7E-17","52-54-00-F5-13-E7","54-E1-AD-89-1A-4C","F8-34-41-3C-C0-85"]`,
		"agent.ephemeral_id": "6c251f67-7210-4cef-8f72-a9546cbb48cc",
		"agent.id":           "e97243c5-5ef3-4dc1-8828-504f68731e87",
		"agent.name":         "foobar-baz-abc",
		"agent.type":         "filebeat",
		"agent.version":      "8.8.0",
		"log.file.path":      "/var/log/auth.log",
		"log.offset":         "37908",
	}, []string{
		"Jun  4 20:34:07 foobar-baz-abc sudo: pam_unix(sudo:session): session opened for user root by (uid=0)",
		"Jun  4 20:34:07 foobar-baz-abc sudo: pam_unix(sudo:session): session opened for user root by (uid=1)",
		"Jun  4 20:34:07 foobar-baz-abc sudo: pam_unix(sudo:session): session opened for user root by (uid=2)",
		"Jun  4 20:34:07 foobar-baz-abc sudo: pam_unix(sudo:session): session opened for user root by (uid=3)",
		"Jun  4 20:34:07 foobar-baz-abc sudo: pam_unix(sudo:session): session opened for user root by (uid=4)",
	})
	streamFields := []string{
		"host.hostname",
		"agent.name",
		"log.file.path",
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(rows)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchmarkLogRowsMustAdd(rows, streamFields)
		}
	})
}

func benchmarkLogRowsMustAdd(rows [][]Field, streamFields []string) {
	lr := GetLogRows(streamFields, nil)
	var tid TenantID
	for i, fields := range rows {
		tid.AccountID = uint32(i)
		tid.ProjectID = uint32(2 * i)
		timestamp := int64(i) * 1000
		lr.MustAdd(tid, timestamp, fields)
	}
	PutLogRows(lr)
}

func newBenchRows(constFields map[string]string, messages []string) [][]Field {
	rows := make([][]Field, 0, len(messages))
	for _, msg := range messages {
		row := make([]Field, 0, len(constFields)+1)
		for k, v := range constFields {
			row = append(row, Field{
				Name:  k,
				Value: v,
			})
		}
		row = append(row, Field{
			Name:  "_msg",
			Value: msg,
		})
		rows = append(rows, row)
	}
	return rows
}
