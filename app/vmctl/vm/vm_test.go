package vm

import "testing"

func TestAddExtraLabelsToImportPath(t *testing.T) {
	type args struct {
		path        string
		extraLabels []string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "ok w/o extra labels",
			args: args{
				path: "/api/v1/import",
			},
			want: "/api/v1/import",
		},
		{
			name: "ok one extra label",
			args: args{
				path:        "/api/v1/import",
				extraLabels: []string{"instance=host-1"},
			},
			want: "/api/v1/import?extra_label=instance=host-1",
		},
		{
			name: "ok two extra labels",
			args: args{
				path:        "/api/v1/import",
				extraLabels: []string{"instance=host-2", "job=vmagent"},
			},
			want: "/api/v1/import?extra_label=instance=host-2&extra_label=job=vmagent",
		},
		{
			name: "ok two extra with exist param",
			args: args{
				path:        "/api/v1/import?timeout=50",
				extraLabels: []string{"instance=host-2", "job=vmagent"},
			},
			want: "/api/v1/import?timeout=50&extra_label=instance=host-2&extra_label=job=vmagent",
		},
		{
			name: "bad incorrect format for extra label",
			args: args{
				path:        "/api/v1/import",
				extraLabels: []string{"label=value", "bad_label_wo_value"},
			},
			want:    "/api/v1/import",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AddExtraLabelsToImportPath(tt.args.path, tt.args.extraLabels)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddExtraLabelsToImportPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("AddExtraLabelsToImportPath() got = %v, want %v", got, tt.want)
			}
		})
	}
}
