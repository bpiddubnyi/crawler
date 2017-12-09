package db

import (
	"time"
)

type Record struct {
	URL     string
	Time    time.Time
	LocalIP string
	Up      bool
}

type Writer interface {
	Write(flushPeriod time.Duration, rC <-chan Record, errC chan<- error)
}
