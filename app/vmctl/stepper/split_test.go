package stepper

import (
	"reflect"
	"testing"
	"time"
)

type testTimeRange []string

func mustParseDatetime(t string) time.Time {
	result, err := time.Parse(time.RFC3339, t)
	if err != nil {
		panic(err)
	}
	return result
}

func Test_splitDateRange(t *testing.T) {
	type args struct {
		start       string
		end         string
		granularity string
	}
	tests := []struct {
		name    string
		args    args
		want    []testTimeRange
		wantErr bool
	}{
		{
			name: "validates start is before end",
			args: args{
				start:       "2022-02-01T00:00:00Z",
				end:         "2022-01-01T00:00:00Z",
				granularity: StepMonth,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "validates granularity value",
			args: args{
				start:       "2022-01-01T00:00:00Z",
				end:         "2022-02-01T00:00:00Z",
				granularity: "non-existent-format",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "month chunking",
			args: args{
				start:       "2022-01-03T11:11:11Z",
				end:         "2022-03-03T12:12:12Z",
				granularity: StepMonth,
			},
			want: []testTimeRange{
				{
					"2022-01-03T11:11:11Z",
					"2022-01-31T23:59:59.999999999Z",
				},
				{
					"2022-02-01T00:00:00Z",
					"2022-02-28T23:59:59.999999999Z",
				},
				{
					"2022-03-01T00:00:00Z",
					"2022-03-03T12:12:12Z",
				},
			},
			wantErr: false,
		},
		{
			name: "daily chunking",
			args: args{
				start:       "2022-01-03T11:11:11Z",
				end:         "2022-01-05T12:12:12Z",
				granularity: StepDay,
			},
			want: []testTimeRange{
				{
					"2022-01-03T11:11:11Z",
					"2022-01-04T11:11:11Z",
				},
				{
					"2022-01-04T11:11:11Z",
					"2022-01-05T11:11:11Z",
				},
				{
					"2022-01-05T11:11:11Z",
					"2022-01-05T12:12:12Z",
				},
			},
			wantErr: false,
		},
		{
			name: "hourly chunking",
			args: args{
				start:       "2022-01-03T11:11:11Z",
				end:         "2022-01-03T14:14:14Z",
				granularity: StepHour,
			},
			want: []testTimeRange{
				{
					"2022-01-03T11:11:11Z",
					"2022-01-03T12:11:11Z",
				},
				{
					"2022-01-03T12:11:11Z",
					"2022-01-03T13:11:11Z",
				},
				{
					"2022-01-03T13:11:11Z",
					"2022-01-03T14:11:11Z",
				},
				{
					"2022-01-03T14:11:11Z",
					"2022-01-03T14:14:14Z",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := mustParseDatetime(tt.args.start)
			end := mustParseDatetime(tt.args.end)

			got, err := SplitDateRange(start, end, tt.args.granularity)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitDateRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			var testExpectedResults [][]time.Time
			if tt.want != nil {
				testExpectedResults = make([][]time.Time, 0)
				for _, dr := range tt.want {
					testExpectedResults = append(testExpectedResults, []time.Time{
						mustParseDatetime(dr[0]),
						mustParseDatetime(dr[1]),
					})
				}
			}

			if !reflect.DeepEqual(got, testExpectedResults) {
				t.Errorf("splitDateRange() got = %v, want %v", got, testExpectedResults)
			}
		})
	}
}
