package actions

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

func Test_removeIgnoreFile(t *testing.T) {
	tests := []struct {
		name  string
		parts []common.Part
		want  []common.Part
	}{
		{
			name:  "got empty parts",
			parts: []common.Part{},
			want:  []common.Part{},
		},
		{
			name: "got parts without backup_complete.ignore file",
			parts: []common.Part{
				{Path: "some_file"},
				{Path: "some_file_1"},
			},
			want: []common.Part{
				{Path: "some_file"},
				{Path: "some_file_1"},
			},
		},
		{
			name: "got parts with backup_complete.ignore file",
			parts: []common.Part{
				{Path: "some_file"},
				{Path: "some_file_1"},
				{Path: "backup_complete.ignore"},
			},
			want: []common.Part{
				{Path: "some_file"},
				{Path: "some_file_1"},
			},
		},
		{
			name: "got parts with path backup_complete.ignore file",
			parts: []common.Part{
				{Path: "some_file"},
				{Path: "some_file_1"},
				{Path: "path/to/backup_complete.ignore"},
			},
			want: []common.Part{
				{Path: "some_file"},
				{Path: "some_file_1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeIgnoreFile(tt.parts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("removeIgnoreFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
