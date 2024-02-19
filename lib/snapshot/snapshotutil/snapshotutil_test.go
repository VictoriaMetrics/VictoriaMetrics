package snapshotutil

import (
	"testing"
)

func Test_Validate(t *testing.T) {
	tests := []struct {
		name         string
		snapshotName string
		want         bool
	}{
		{
			name:         "empty snapshot name",
			snapshotName: "",
			want:         false,
		},
		{
			name:         "short snapshot name",
			snapshotName: "",
			want:         false,
		},
		{
			name:         "short first part of the snapshot name",
			snapshotName: "2022050312163-16EB56ADB4110CF2",
			want:         false,
		},
		{
			name:         "short second part of the snapshot name",
			snapshotName: "20220503121638-16EB56ADB4110CF",
			want:         true,
		},
		{
			name:         "correct snapshot name",
			snapshotName: "20220503121638-16EB56ADB4110CF2",
			want:         true,
		},
		{
			name:         "invalid time part snapshot name",
			snapshotName: "00000000000000-16EB56ADB4110CF2",
			want:         false,
		},
		{
			name:         "not enough parts of the snapshot name",
			snapshotName: "2022050312163816EB56ADB4110CF2",
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.snapshotName); (err == nil) != tt.want {
				t.Errorf("checkSnapshotName() = %v, want %v", err, tt.want)
			}
		})
	}
}
