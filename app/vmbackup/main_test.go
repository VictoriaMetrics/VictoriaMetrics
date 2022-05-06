package main

import (
	"flag"
	"testing"
)

func Test_newDstFS(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "empty dst flag",
			args:    []string{"-dst", "", "-storageDataPath", "victoria-metrics-data"},
			wantErr: true,
		},
		{
			name:    "wrong dst flag",
			args:    []string{"-dst", "123123", "-storageDataPath", "victoria-metrics-data"},
			wantErr: true,
		},
		{
			name:    "empty path for dst flag",
			args:    []string{"-dst", "fs://", "-storageDataPath", "victoria-metrics-data"},
			wantErr: true,
		},
		{
			name:    "dst flag has the same dir as storageDataPath flag",
			args:    []string{"-dst", "fs:///path/to/local/backup/victoria-metrics-data", "-storageDataPath", "victoria-metrics-data"},
			wantErr: true,
		},
		{
			name:    "dst flag is s3 file system",
			args:    []string{"-dst", "s3://bucket/path/to/backup/dir", "-storageDataPath", "victoria-metrics-data"},
			wantErr: false,
		},
		{
			name:    "dst flag do not contain storageDataPath",
			args:    []string{"-dst", "fs:///bucket/path/to/backup/dir", "-storageDataPath", "victoria-metrics-data"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parseFlags(tt.args); err != nil {
				t.Fatalf("error parse flags : %s", err)
			}
			_, err := newDstFS()

			if (err != nil) != tt.wantErr {
				t.Errorf("newDstFS() error = %#v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func parseFlags(args []string) error {
	flags := flag.NewFlagSet("test_program", flag.ContinueOnError)
	flags.StringVar(dst, "dst", "", "")
	flags.StringVar(storageDataPath, "storageDataPath", "", "")

	if err := flags.Parse(args); err != nil {
		return err
	}
	return nil
}
