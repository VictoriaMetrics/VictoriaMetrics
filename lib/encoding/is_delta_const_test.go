package encoding

import (
	"testing"
)

// go test -timeout 30s -v -run ^TestIsDeltaConstFast$ github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding
func TestIsDeltaConstFast(t *testing.T) {
	type args struct {
		a []int64
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "7",
			args: args{
				a: []int64{1, 2, 3, 4, 5, 6},
			},
			want: true,
		},
		{
			name: "6",
			args: args{
				a: []int64{1, 2, 3, 4, 5},
			},
			want: true,
		},
		{
			name: "1",
			args: args{
				a: []int64{},
			},
			want: false,
		},
		{
			name: "2",
			args: args{
				a: []int64{1},
			},
			want: false,
		},
		{
			name: "3",
			args: args{
				a: []int64{1, 2},
			},
			want: true,
		},
		{
			name: "4",
			args: args{
				a: []int64{1, 2, 3},
			},
			want: true,
		},
		{
			name: "5",
			args: args{
				a: []int64{1, 2, 3, 4},
			},
			want: true,
		},
		{
			name: "8",
			args: args{
				a: []int64{1, 2, 3, 4, 5, 6, 7},
			},
			want: true,
		},
		{
			name: "9",
			args: args{
				a: []int64{1, 2, 3, 4, 5, 6, 7, 8},
			},
			want: true,
		},
		{
			name: "10",
			args: args{
				a: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			},
			want: true,
		},
		{
			name: "11",
			args: args{
				a: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
			want: true,
		},
		{
			name: "21",
			args: args{
				a: []int64{1, 2, 3, 3, 5, 6, 7, 8, 9, 10},
			},
			want: false,
		},
		{
			name: "22",
			args: args{
				a: []int64{1, 2, 3, 4, 4, 6, 7, 8, 9, 10},
			},
			want: false,
		},
		{
			name: "23",
			args: args{
				a: []int64{1, 2, 3, 4, 5, 5, 7, 8, 9, 10},
			},
			want: false,
		},
		{
			name: "24",
			args: args{
				a: []int64{1, 2, 3, 4, 5, 6, 6, 8, 9, 10},
			},
			want: false,
		},
		{
			name: "25",
			args: args{
				a: []int64{1, 2, 3, 4, 5, 6, 7, 7, 9, 10},
			},
			want: false,
		},
		{
			name: "26",
			args: args{
				a: []int64{1, 2, 3, 4, 5, 6, 7, 8, 8, 10},
			},
			want: false,
		},
		{
			name: "27",
			args: args{
				a: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 9},
			},
			want: false,
		},
		{
			name: "29",
			args: args{
				a: []int64{1, 2, 3, 3},
			},
			want: false,
		},
		{
			name: "30",
			args: args{
				a: []int64{1, 2, 3, 4, 4},
			},
			want: false,
		},
		{
			name: "31",
			args: args{
				a: []int64{3, 2, 1, 0, -1, -2, -3},
			},
			want: true,
		},
		{
			name: "32",
			args: args{
				a: []int64{3, 2, 1, 0, -1, -1, -3},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDeltaConst(tt.args.a); got != tt.want {
				t.Errorf("IsDeltaConst() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDeltaConst(t *testing.T) {
	f := func(a []int64, okExpected bool) {
		t.Helper()
		ok := IsDeltaConst(a)
		if ok != okExpected {
			t.Fatalf("unexpected isDeltaConst for a=%d; got %v; want %v", a, ok, okExpected)
		}
	}
	f([]int64{}, false)
	f([]int64{1}, false)
	f([]int64{1, 2}, true)
	f([]int64{1, 2, 3}, true)
	f([]int64{3, 2, 1}, true)
	f([]int64{3, 2, 1, 0, -1, -2}, true)
	f([]int64{3, 2, 1, 0, -1, -2, 2}, false)
	f([]int64{1, 1}, true)
	f([]int64{1, 2, 1}, false)
	f([]int64{1, 2, 4}, false)
}
