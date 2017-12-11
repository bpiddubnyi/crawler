package main

import (
	"fmt"
	"time"
)

type interval struct {
	up   bool
	from time.Time
	to   time.Time
}

func (i *interval) Duration() time.Duration {
	return i.to.Sub(i.from)
}

type stat struct {
	url         string
	wholeTime   time.Duration
	upTime      time.Duration
	longestDown *interval
}

func (s *stat) Summary() string {
	if s.longestDown == nil {
		return fmt.Sprintf("%s: uptime: 100%%", s.url)
	}

	uptimePerc := float32(s.upTime*100) / float32(s.wholeTime)
	return fmt.Sprintf("%s:\n\twhole time: %s \n\tuptime: %s (%.2f%%)\n\tlongest downtime %s:\n\t\tfrom: %s\n\t\tto:   %s",
		s.url, s.wholeTime, s.upTime, uptimePerc, s.longestDown.Duration(),
		s.longestDown.from.In(time.Local), s.longestDown.to.In(time.Local))
}

type serverUptime struct {
	url       string
	intervals []interval
}

func (u *serverUptime) Stat() *stat {
	s := &stat{url: u.url}
	for i, iv := range u.intervals {
		s.wholeTime += iv.Duration()
		if iv.up {
			s.upTime += iv.Duration()
		} else {
			if s.longestDown == nil || s.longestDown.Duration() < iv.Duration() {
				s.longestDown = &u.intervals[i]
			}
		}
	}
	return s
}
