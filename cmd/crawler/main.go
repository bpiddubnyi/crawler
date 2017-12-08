package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bpiddubnyi/uptime/cmd/crawler/config"
)

var (
	dbURI       = "postgres://user:password@localhost/db"
	cfgFileName string
	period      = 30
	showHelp    = false
	ipsRaw      string
)

type client struct {
	c *http.Client
	a *net.TCPAddr
}

func init() {
	flag.StringVar(&dbURI, "db", dbURI, "postgres connection string")
	flag.StringVar(&cfgFileName, "config", cfgFileName, "config file with urls to be monitored")
	flag.IntVar(&period, "period", period, "monitoring period in seconds")
	flag.BoolVar(&showHelp, "help", showHelp, "show this help message and exit")
	flag.StringVar(&ipsRaw, "ips", ipsRaw, "comma separated list of ip addresses")

}

func main() {
	flag.Parse()

	if showHelp {
		flag.Usage()
		return
	}

	if len(cfgFileName) == 0 {
		fmt.Printf("Error: config filename is empty\n")
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

	var clients []client
	if len(ipsRaw) > 0 {
		ips := strings.Split(ipsRaw, ",")
		clients = make([]client, len(ips))
		for i, ip := range ips {
			ad, err := net.ResolveTCPAddr("tcp", ip+":0")
			if err != nil {
				fmt.Printf("Failed to resolve tcp address %s: %s\n", ip, err)
				os.Exit(1)
			}

			clients[i] = client{
				c: &http.Client{
					Timeout: time.Duration(period) * time.Second,
					Transport: &http.Transport{
						Proxy: http.ProxyFromEnvironment,
						DialContext: (&net.Dialer{
							LocalAddr: ad,
							Timeout:   time.Duration(period) * time.Second,
							KeepAlive: time.Duration(period) * time.Second,
							DualStack: true,
						}).DialContext,
						MaxIdleConns:          100,
						IdleConnTimeout:       90 * time.Second,
						TLSHandshakeTimeout:   10 * time.Second,
						ExpectContinueTimeout: 1 * time.Second,
					},
				},
				a: ad,
			}
		}
	} else {

		conn, err := net.Dial("udp", "8.8.8.8:53")
		if err != nil {
			fmt.Printf("Error: Failed to create connection to get the local address: %s", err)
			os.Exit(1)
		}
		a := &net.TCPAddr{IP: conn.LocalAddr().(*net.UDPAddr).IP}

		clients = make([]client, 1)
		clients[0] = client{
			c: &http.Client{
				Timeout: time.Duration(period) * time.Second,
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					DialContext: (&net.Dialer{
						Timeout:   time.Duration(period) * time.Second,
						KeepAlive: time.Duration(period) * time.Second,
						DualStack: true,
					}).DialContext,
					MaxIdleConns:          100,
					IdleConnTimeout:       90 * time.Second,
					TLSHandshakeTimeout:   10 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
				},
			},
			a: a,
		}
	}

	stopC := make(chan os.Signal, 2)
	signal.Notify(stopC, syscall.SIGTERM, syscall.SIGINT)

	tick := time.NewTicker(time.Duration(period) * time.Second)

	wg := sync.WaitGroup{}

theLoop:
	for {
		select {
		case sig := <-stopC:
			log.Printf("%s signal received. Stop gracefully", sig)
			break theLoop

		case t := <-tick.C:
			for _, url := range urls {
				wg.Add(1)
				go func(url string, t time.Time) {
					defer wg.Done()

					for _, c := range clients {
						_, err := c.c.Get(url)
						if err != nil {
							fmt.Printf("%s %s %s: down (%s)\n", c.a, t, url, err)
						} else {
							fmt.Printf("%s %s %s: up\n", c.a, t, url)
						}
					}
				}(url, t)
			}
		}
	}
	wg.Wait()

}
