package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bpiddubnyi/crawler/cmd/crawler/client"
	"github.com/bpiddubnyi/crawler/cmd/crawler/config"
	"github.com/bpiddubnyi/crawler/db/pq"

	"net/http"
	_ "net/http/pprof"
)

var (
	dbURI            = "postgres://user:password@localhost/db?sslmode=disable"
	cfgFileName      string
	period           = 30
	showHelp         = false
	ipsRaw           string
	reconnectRetries = 5
	pprofPort        = 0
	followRedirects  = false
	dbFlushPeriod    = 5
)

func init() {
	flag.StringVar(&dbURI, "db", dbURI, "postgres connection string")
	flag.StringVar(&cfgFileName, "config", cfgFileName, "config file with urls to be monitored")
	flag.IntVar(&period, "period", period, "monitoring period in seconds")
	flag.BoolVar(&showHelp, "help", showHelp, "show this help message and exit")
	flag.StringVar(&ipsRaw, "ips", ipsRaw, "comma separated list of ip addresses")
	flag.IntVar(&reconnectRetries, "retry", reconnectRetries, "number of db connection attempts, convenient for docker-compose")
	flag.IntVar(&pprofPort, "pprof", pprofPort, "pprof web server port for profiling purposes (0 - disabled)")
	flag.BoolVar(&followRedirects, "follow", followRedirects, "follow HTTP redirects")
	flag.IntVar(&dbFlushPeriod, "flush", dbFlushPeriod, "database flush period in seconds")
}

func main() {
	flag.Parse()

	if showHelp {
		flag.Usage()
		return
	}

	if len(cfgFileName) == 0 {
		fmt.Printf("Error: empty config filename\n")
		flag.Usage()
		os.Exit(1)
	}

	cfgFile, err := os.Open(cfgFileName)
	if err != nil {
		fmt.Printf("Error: failed to open config: %s\n", err)
		os.Exit(1)
	}

	urls, err := config.Parse(cfgFile)
	if err != nil {
		fmt.Printf("Error: failed to parse config: %s\n", err)
		os.Exit(1)
	}
	cfgFile.Close()

	db, err := pq.New(dbURI, reconnectRetries)
	if err != nil {
		fmt.Printf("Error: failed to create db connection: %s\n", err)
		os.Exit(1)
	}

	var ips []string
	if len(ipsRaw) > 0 {
		ips = strings.Split(ipsRaw, ",")
	}

	if pprofPort > 0 {
		go func() {
			log.Println(http.ListenAndServe(fmt.Sprintf("localhost:%d", pprofPort), nil))
		}()
	}

	client, err := client.New(ips, time.Duration(period)*time.Second, followRedirects, db)
	if err != nil {
		fmt.Printf("Error: failed to create crawler: %s\n", err)
		os.Exit(1)
	}

	sigC := make(chan os.Signal, 2)
	signal.Notify(sigC, syscall.SIGTERM, syscall.SIGINT)

	shutdownC := make(chan struct{})

	go func() {
		sig := <-sigC
		fmt.Printf("Info: Received %s signal. Shutting down gracefully\n", sig)
		close(shutdownC)
	}()

	fmt.Printf("Starting crawler [∫]\n")
	if err = client.Crawl(urls, time.Duration(dbFlushPeriod)*time.Second, shutdownC); err != nil {
		fmt.Printf("Error: Crawler failed: %s\n", err)
	}
}
