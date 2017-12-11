package stat

import (
	"reflect"
	"testing"
	"time"

	"github.com/bpiddubnyi/crawler/db"
)

const timeFormat = "02.01.2006 15:04:05"

func getTime(tS string, t *testing.T) time.Time {
	timeT, err := time.Parse(timeFormat, tS)
	if err != nil {
		t.Fatalf("Failed to parse time %s: %s", tS, err)
	}
	return timeT
}

func TestAggregate(t *testing.T) {
	type args struct {
		recs []db.Record
	}
	tests := []struct {
		name string
		args args
		want []Stat
	}{
		{
			name: "simple 100% uptime",
			args: args{
				recs: []db.Record{
					{
						URL:     "http://test.com",
						Up:      true,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:00:00", t),
					},
					{
						URL:     "http://test.com",
						Up:      true,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:01:00", t),
					},
					{
						URL:     "http://test.com",
						Up:      true,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:02:00", t),
					},
				},
			},
			want: []Stat{
				{
					URL:         "http://test.com",
					LocalIP:     "127.0.0.1",
					WholeTime:   2 * time.Minute,
					UpTime:      2 * time.Minute,
					LongestDown: nil,
				},
			},
		},
		{
			name: "simple 100% downtime",
			args: args{
				recs: []db.Record{
					{
						URL:     "http://test.com",
						Up:      false,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:00:00", t),
					},
					{
						URL:     "http://test.com",
						Up:      false,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:01:00", t),
					},
					{
						URL:     "http://test.com",
						Up:      false,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:02:00", t),
					},
				},
			},
			want: []Stat{
				{
					URL:       "http://test.com",
					LocalIP:   "127.0.0.1",
					WholeTime: 2 * time.Minute,
					UpTime:    0,
					LongestDown: &Interval{
						From: getTime("01.01.1972 00:00:00", t),
						To:   getTime("01.01.1972 00:02:00", t),
					},
				},
			},
		},
		{
			name: "2 urls 50% uptime",
			args: args{
				recs: []db.Record{
					{
						URL:     "http://test.com",
						Up:      false,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:00:00", t),
					},
					{
						URL:     "http://test.com",
						Up:      false,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:01:00", t),
					},
					{
						URL:     "http://test.com",
						Up:      true,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:02:00", t),
					},
					{
						URL:     "http://test.com",
						Up:      true,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:03:00", t),
					},
					{
						URL:     "http://shmest.com",
						Up:      true,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:00:00", t),
					},
					{
						URL:     "http://shmest.com",
						Up:      true,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:01:00", t),
					},
					{
						URL:     "http://shmest.com",
						Up:      false,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:02:00", t),
					},
					{
						URL:     "http://shmest.com",
						Up:      false,
						LocalIP: "127.0.0.1",
						Time:    getTime("01.01.1972 00:03:00", t),
					},
				},
			},
			want: []Stat{
				{
					URL:       "http://test.com",
					LocalIP:   "127.0.0.1",
					WholeTime: 3 * time.Minute,
					UpTime:    1 * time.Minute,
					LongestDown: &Interval{
						From: getTime("01.01.1972 00:00:00", t),
						To:   getTime("01.01.1972 00:02:00", t),
					},
				},
				{
					URL:       "http://shmest.com",
					LocalIP:   "127.0.0.1",
					WholeTime: 3 * time.Minute,
					UpTime:    2 * time.Minute,
					LongestDown: &Interval{
						From: getTime("01.01.1972 00:02:00", t),
						To:   getTime("01.01.1972 00:03:00", t),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Aggregate(tt.args.recs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Aggregate() = %v, want %v", got, tt.want)
			}
		})
	}
}
