package stat

import (
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

func Aggregate(recs []db.Record) []Stat {
	if len(recs) == 0 {
		return nil
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
	return append(stat, curUptime.Stat())
}
