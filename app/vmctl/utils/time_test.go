package utils

import (
	"testing"
	"time"
)

func TestGetTime(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    func() time.Time
		wantErr bool
	}{
		{
			name:    "empty string",
			s:       "",
			want:    func() time.Time { return time.Time{} },
			wantErr: true,
		},
		{
			name: "only year",
			s:    "2019",
			want: func() time.Time {
				t := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
				return t
			},
		},
		{
			name: "year and month",
			s:    "2019-01",
			want: func() time.Time {
				t := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
				return t
			},
		},
		{
			name: "year and not first month",
			s:    "2019-02",
			want: func() time.Time {
				t := time.Date(2019, 2, 1, 0, 0, 0, 0, time.UTC)
				return t
			},
		},
		{
			name: "year, month and day",
			s:    "2019-02-01",
			want: func() time.Time {
				t := time.Date(2019, 2, 1, 0, 0, 0, 0, time.UTC)
				return t
			},
		},
		{
			name: "year, month and not first day",
			s:    "2019-02-10",
			want: func() time.Time {
				t := time.Date(2019, 2, 10, 0, 0, 0, 0, time.UTC)
				return t
			},
		},
		{
			name: "year, month, day and time",
			s:    "2019-02-02T00",
			want: func() time.Time {
				t := time.Date(2019, 2, 2, 0, 0, 0, 0, time.UTC)
				return t
			},
		},
		{
			name: "year, month, day and one hour time",
			s:    "2019-02-02T01",
			want: func() time.Time {
				t := time.Date(2019, 2, 2, 1, 0, 0, 0, time.UTC)
				return t
			},
		},
		{
			name: "time with zero minutes",
			s:    "2019-02-02T01:00",
			want: func() time.Time {
				t := time.Date(2019, 2, 2, 1, 0, 0, 0, time.UTC)
				return t
			},
		},
		{
			name: "time with one minute",
			s:    "2019-02-02T01:01",
			want: func() time.Time {
				t := time.Date(2019, 2, 2, 1, 1, 0, 0, time.UTC)
				return t
			},
		},
		{
			name: "time with zero seconds",
			s:    "2019-02-02T01:01:00",
			want: func() time.Time {
				t := time.Date(2019, 2, 2, 1, 1, 0, 0, time.UTC)
				return t
			},
		},
		{
			name: "timezone with one second",
			s:    "2019-02-02T01:01:01",
			want: func() time.Time {
				t := time.Date(2019, 2, 2, 1, 1, 1, 0, time.UTC)
				return t
			},
		},
		{
			name: "time with two second and timezone",
			s:    "2019-07-07T20:01:02Z",
			want: func() time.Time {
				t := time.Date(2019, 7, 7, 20, 1, 02, 0, time.UTC)
				return t
			},
		},
		{
			name: "time with seconds and timezone",
			s:    "2019-07-07T20:47:40+03:00",
			want: func() time.Time {
				l, _ := time.LoadLocation("Europe/Kiev")
				t := time.Date(2019, 7, 7, 20, 47, 40, 0, l)
				return t
			},
		},
		{
			name:    "negative time",
			s:       "-292273086-05-16T16:47:06Z",
			want:    func() time.Time { return time.Time{} },
			wantErr: true,
		},
		{
			name: "float timestamp representation",
			s:    "1562529662.324",
			want: func() time.Time {
				t := time.Date(2019, 7, 7, 20, 01, 02, 324e6, time.UTC)
				return t
			},
		},
		{
			name: "negative timestamp",
			s:    "-9223372036.855",
			want: func() time.Time {
				return time.Date(1970, 01, 01, 00, 00, 00, 00, time.UTC)
			},
			wantErr: false,
		},
		{
			name: "big timestamp",
			s:    "1223372036855",
			want: func() time.Time {
				t := time.Date(2008, 10, 7, 9, 33, 56, 855e6, time.UTC)
				return t
			},
			wantErr: false,
		},
		{
			name: "duration time",
			s:    "1h5m",
			want: func() time.Time {
				t := time.Now().Add(-1 * time.Hour).Add(-5 * time.Minute)
				return t
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTime(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			w := tt.want()
			if got.Unix() != w.Unix() {
				t.Errorf("ParseTime() got = %v, want %v", got, w)
			}
		})
	}
}
