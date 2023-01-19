package flagutil

import (
	"testing"
	"time"
)

func TestTime_Set(t *testing.T) {

	tests := []struct {
		name         string
		locationName string
		layout       string
		value        string
		want         func() *time.Time
		wantErr      bool
	}{
		{
			name:    "empty value",
			layout:  "",
			value:   "",
			want:    func() *time.Time { return &time.Time{} },
			wantErr: false,
		},
		{
			name:   "correct value has layout",
			layout: time.RFC3339,
			value:  "2023-01-19T10:38:53+00:00",
			want: func() *time.Time {
				date := time.Date(2023, 1, 19, 10, 38, 53, 0, time.UTC)
				return &date
			},
			wantErr: false,
		},
		{
			name:         "correct value has layout with location",
			locationName: "Europe/Kiev",
			layout:       time.RFC3339,
			value:        "2023-01-19T10:38:53+00:00",
			want: func() *time.Time {
				date := time.Date(2023, 1, 19, 10, 38, 53, 0, time.UTC)
				return &date
			},
			wantErr: false,
		},
		{
			name:         "correct value has layout with america location",
			locationName: "America/Los_Angeles",
			layout:       "Jan 2, 2006 at 3:04pm (MST)",
			value:        "Jan 19, 2023 at 10:38pm (PST)",
			want: func() *time.Time {
				l, _ := time.LoadLocation("America/Los_Angeles")
				date := time.Date(2023, 1, 19, 22, 38, 00, 0, l)
				return &date
			},
			wantErr: false,
		},
		{
			name:    "correct value without layout",
			layout:  "",
			value:   "2023-01-19T10:38:53+00:00",
			want:    func() *time.Time { return &time.Time{} },
			wantErr: true,
		},
		{
			name:         "parse wrong time",
			locationName: "",
			layout:       "Jan 2, 2006 at 3:04pm (MST)",
			value:        "2006-01-02T15:04:05Z",
			want:         func() *time.Time { return &time.Time{} },
			wantErr:      true,
		},
		{
			name:         "got wrong layout",
			locationName: "",
			layout:       "wrong layout",
			value:        "2006-01-02T15:04:05Z",
			want:         func() *time.Time { return &time.Time{} },
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				l   *time.Location
				err error
			)

			if tt.locationName != "" {
				l, err = time.LoadLocation(tt.locationName)
				if err != nil {
					t.Fatalf("error get location: %s", err)
				}
			}

			tm := &Time{location: l, layout: tt.layout}
			if err = tm.Set(tt.value); (err != nil) != tt.wantErr {
				t.Fatalf("Set() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tm.Timestamp.Unix() != tt.want().Unix() {
				t.Errorf("Set() want = %v, got %v", tt.want(), tm.Timestamp)
			}
		})
	}
}
