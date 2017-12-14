package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bpiddubnyi/crawler/cmd/crawler/client"
	"github.com/bpiddubnyi/crawler/cmd/crawler/config"
	"github.com/bpiddubnyi/crawler/db/pq"
)

var (
	cfgFileName      string
	ipsRaw           string
	proxiesRaw       string
	pprofAddr        string
	dbURI            = "postgres://user:password@localhost/db?sslmode=disable"
	period           = 60
	showHelp         = false
	reconnectRetries = 5
	followRedirects  = false
	dbFlushPeriod    = 5
	nWorkers         = 40
)

func init() {
	flag.StringVar(&dbURI, "db", dbURI, "postgres connection string")
	flag.StringVar(&cfgFileName, "config", cfgFileName, "config file with urls to be monitored")
	flag.IntVar(&period, "period", period, "monitoring period in seconds")
	flag.BoolVar(&showHelp, "help", showHelp, "show this help message and exit")
	flag.StringVar(&ipsRaw, "ips", ipsRaw, "comma separated list of ip addresses")
	flag.IntVar(&reconnectRetries, "retry", reconnectRetries, "number of db connection attempts, convenient for docker-compose")
	flag.StringVar(&pprofAddr, "pprof", pprofAddr, "pprof web server listen address for profiling purposes (empty - disabled)")
	flag.BoolVar(&followRedirects, "follow", followRedirects, "follow HTTP redirects")
	flag.IntVar(&dbFlushPeriod, "flush", dbFlushPeriod, "database flush period in seconds")
	flag.StringVar(&proxiesRaw, "proxies", proxiesRaw, "comma separated proxy url list")
	flag.IntVar(&nWorkers, "workers", nWorkers, "number of workers per IP/proxy")
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

	if period < 1 {
		fmt.Printf("Error: period shoud be positive integer value\n")
		os.Exit(1)
	}

	if nWorkers < 1 {
		fmt.Printf("Error: workers should be positive integer value\n")
		os.Exit(1)
	}

	if reconnectRetries < 1 {
		fmt.Printf("Error: retry should be positive integer value\n")
		os.Exit(1)
	}

	if dbFlushPeriod < 1 {
		fmt.Printf("Error: flush period should be positive integer value\n")
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

	var proxies []string
	if len(proxiesRaw) > 0 {
		proxies = strings.Split(proxiesRaw, ",")
	}

	if len(pprofAddr) > 0 {
		go func() {
			err := http.ListenAndServe(pprofAddr, nil)
			if err != nil {
				fmt.Printf("Error: Failed to start pprof web server: %s\n", err)
			}
		}()
	}

	client, err := client.New(ips, proxies, time.Duration(period)*time.Second, followRedirects, db)
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

	log.Printf("Starting crawler [∫]\n")
	if err = client.Crawl(urls, time.Duration(dbFlushPeriod)*time.Second, nWorkers, shutdownC); err != nil {
		fmt.Printf("Error: Crawler failed: %s\n", err)
	}
}
