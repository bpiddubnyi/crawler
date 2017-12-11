package stat

import (
	"fmt"
	"time"

	"github.com/bpiddubnyi/crawler/db"
)

type Interval struct {
	Up   bool
	From time.Time
	To   time.Time
}

func (i *Interval) Duration() time.Duration {
	return i.To.Sub(i.From)
}

type Stat struct {
	URL         string
	LocalIP     string
	WholeTime   time.Duration
	UpTime      time.Duration
	LongestDown *Interval
}

// func (s *Stat) Summary() string {
// 	if s.LongestDown == nil {
// 		return fmt.Sprintf("%s: uptime: 100%%", s.URL)
// 	}

// 	uptimePerc := float32(s.UpTime*100) / float32(s.WholeTime)
// 	return fmt.Sprintf("%s [from %s]:\n\twhole time: %s \n\tuptime: %s (%.2f%%)\n\tlongest downtime %s:\n\t\tfrom: %s\n\t\tto:   %s",
// 		s.URL, s.LocalIP, s.WholeTime, s.UpTime, uptimePerc, s.LongestDown.Duration(),
// 		s.LongestDown.From.In(time.Local), s.LongestDown.To.In(time.Local))
// }

type serverUptime struct {
	URL       string
	LocalIP   string
	Intervals []Interval
}

func (u *serverUptime) Stat() Stat {
	s := Stat{URL: u.URL, LocalIP: u.LocalIP}
	for i, iv := range u.Intervals {
		s.WholeTime += iv.Duration()
		if iv.Up {
			s.UpTime += iv.Duration()
		} else {
			if s.LongestDown == nil || s.LongestDown.Duration() < iv.Duration() {
				s.LongestDown = &u.Intervals[i]
			}
		}
	}
	return s
}

type Collector struct {
	DB db.RecordGetter
}

func (c *Collector) Collect(from, to time.Time, urls []string) ([]Stat, error) {
	recs, err := c.DB.GetRecords(from, to, urls...)
	if err != nil {
		return nil, fmt.Errorf("Failed to get records: %s", err)
	}

	if len(recs) == 0 {
		return nil, nil
	}

	var (
		curUptime        *serverUptime
		curInterval      *Interval
		curIntIncomplete bool
	)

	stat := []Stat{}
	for _, r := range recs {
		if curUptime != nil && (curUptime.URL != r.URL || curUptime.LocalIP != r.LocalIP) {
			if curInterval != nil && !curIntIncomplete {
				curUptime.Intervals = append(curUptime.Intervals, *curInterval)
			}
			stat = append(stat, curUptime.Stat())
		}

		if curUptime == nil || (curUptime.URL != r.URL || curUptime.LocalIP != r.LocalIP) {
			curUptime = &serverUptime{URL: r.URL, LocalIP: r.LocalIP, Intervals: []Interval{}}
			curInterval = &Interval{Up: r.Up, From: r.Time}
			curIntIncomplete = true

			continue
		}

		if curInterval.Up == r.Up {
			if curIntIncomplete {
				curIntIncomplete = false
			}
			curInterval.To = r.Time
		} else {
			if curIntIncomplete {
				curIntIncomplete = false
			}
			curInterval.To = r.Time
			curUptime.Intervals = append(curUptime.Intervals, *curInterval)

			curInterval = &Interval{Up: r.Up, From: r.Time}
			curIntIncomplete = true
		}
	}

	if curInterval != nil && !curIntIncomplete {
		curUptime.Intervals = append(curUptime.Intervals, *curInterval)
	}
	return append(stat, curUptime.Stat()), nil
}
