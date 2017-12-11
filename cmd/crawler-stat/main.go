package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bpiddubnyi/crawler/cmd/crawler-stat/stat"
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

func printStat(s stat.Stat) {
	uptimePerc := float32(s.UpTime*100) / float32(s.WholeTime)
	fmt.Printf("%s [from %s]:\n\twhole time: %s \n\tuptime: %s (%.2f%%)\n",
		s.URL, s.LocalIP, s.WholeTime, s.UpTime, uptimePerc)

	if s.LongestDown == nil {
		return
	}

	fmt.Printf("\tlongest downtime %s:\n\t\tfrom: %s\n\t\tto:   %s\n", s.LongestDown.Duration(),
		s.LongestDown.From.In(time.Local), s.LongestDown.To.In(time.Local))
}

func printUsage() {
	fmt.Printf("Usage: %s [options] url...\n", os.Args[0])
	fmt.Printf("Options:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Parse()

	if showHelp {
		printUsage()
		return
	}

	if len(fromRaw) == 0 {
		fmt.Println("Error: from is empty")
		printUsage()
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

	urls := flag.Args()

	d, err := pq.New(dbURI, 1)
	if err != nil {
		fmt.Printf("Error: Failed to connect to db: %s\n", err)
		os.Exit(1)
	}

	recs, err := d.GetRecords(from, to, urls...)
	if err != nil {
		fmt.Printf("Error: Failed to get records: %s", err)
		os.Exit(1)
	}

	stats := stat.Aggregate(recs)
	for _, s := range stats {
		printStat(s)
	}
}
