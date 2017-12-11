package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bpiddubnyi/crawler/db/pq"
)

var (
	fromRaw  string
	toRaw    string
	server   string
	dbURI    = "postgres://user:password@localhost/db?sslmode=disable"
	showHelp bool
)

const (
	timeFormat      = "02.01.2006 15:04:05"
	shortTimeFormat = "15:04:05"
)

func init() {
	flag.StringVar(&fromRaw, "from", fromRaw, fmt.Sprintf("starting time in %s format", timeFormat))
	flag.StringVar(&toRaw, "to", toRaw, fmt.Sprintf("end time in %s format, current time by default", timeFormat))
	flag.StringVar(&server, "server", server, "server to query stats for")
	flag.BoolVar(&showHelp, "help", showHelp, "show this help messahe and exit")
	flag.StringVar(&dbURI, "db", dbURI, "postgres connection string")
}

func parseTimeString(str string) (time.Time, error) {
	cur := time.Now()

	t, err := time.ParseInLocation(timeFormat, str, cur.Location())
	if err == nil {
		return t, nil
	}

	t, err = time.ParseInLocation(shortTimeFormat, str, cur.Location())
	if err == nil {
		// This is ugly, but it seems like it's the only way to set the date without
		// touching the time
		t = t.AddDate(int(cur.Year()-t.Year()), int(cur.Month()-t.Month()), int(cur.Day()-t.Day()))
		return t, nil
	}
	return t, err
}

func main() {
	flag.Parse()
	flag.Args()

	if showHelp {
		flag.Usage()
		return
	}

	if len(fromRaw) == 0 {
		fmt.Println("Error: from is empty")
		flag.Usage()
		os.Exit(1)
	}

	from, err := parseTimeString(fromRaw)
	if err != nil {
		fmt.Printf("Error: Failed to parse 'from' time: %s\n", err)
		os.Exit(1)
	}

	to := time.Now()
	if len(toRaw) > 0 {
		to, err = parseTimeString(toRaw)
		if err != nil {
			fmt.Printf("Error: Failed to parse 'to' time: %s\n", err)
			os.Exit(1)
		}
	}

	domains := flag.Args()

	d, err := pq.New(dbURI, 1)
	if err != nil {
		fmt.Printf("Error: Failed to connect to db: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("from: ", from)
	fmt.Println("to: ", to)

	recs, err := d.GetRecords(from, to, domains...)
	if err != nil {
		fmt.Printf("Error: failed to get records: %s\n", err)
		os.Exit(1)
	}

	if len(recs) == 0 {
		return
	}

	var (
		curUptime        *serverUptime
		curInterval      *interval
		curIntIncomplete bool
	)

	for _, r := range recs {
		if curUptime != nil && curUptime.url != r.URL {
			if curInterval != nil && !curIntIncomplete {
				curUptime.intervals = append(curUptime.intervals, *curInterval)
			}
			fmt.Println(curUptime.Stat().Summary())
		}

		if curUptime == nil || curUptime.url != r.URL {
			curUptime = &serverUptime{url: r.URL, intervals: []interval{}}
			curInterval = &interval{up: r.Up, from: r.Time}
			curIntIncomplete = true

			continue
		}

		if curInterval.up == r.Up {
			if curIntIncomplete {
				curIntIncomplete = false
			}
			curInterval.to = r.Time
		} else {
			if curIntIncomplete {
				curIntIncomplete = false
			}
			curInterval.to = r.Time
			curUptime.intervals = append(curUptime.intervals, *curInterval)

			curInterval = &interval{up: r.Up, from: r.Time}
			curIntIncomplete = true
		}
	}
	if curInterval != nil && !curIntIncomplete {
		curUptime.intervals = append(curUptime.intervals, *curInterval)
	}
	fmt.Println(curUptime.Stat().Summary())
}
