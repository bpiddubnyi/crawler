package main

import (
	"flag"
	"fmt"
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

type client struct {
	c *http.Client
	a *net.TCPAddr
}

func (c *client) check(url string) {
	t := time.Now()

	_, err := c.c.Get(url)
	if err != nil {
		fmt.Printf("%s %s %s: down (%s)\n", c.a.IP.String(), t.UTC().String(), url, err)
	} else {
		fmt.Printf("%s %s %s: up\n", c.a.IP.String(), t.UTC().String(), url)
	}
}

func (c *client) Check(url string, period time.Duration, shutdown chan struct{}) {
	tick := time.NewTicker(period)
	defer tick.Stop()

	wg := sync.WaitGroup{}

theLoop:
	for {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.check(url)
		}()

		select {
		case <-tick.C:
			continue theLoop

		case _, ok := <-shutdown:
			if !ok {
				break theLoop
			}
		}
	}
	wg.Wait()
}

func getLocalAddr() (*net.TCPAddr, error) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return nil, err
	}
	return &net.TCPAddr{IP: conn.LocalAddr().(*net.UDPAddr).IP}, nil
}

func setupClient(period int, addr *net.TCPAddr) *http.Client {
	return &http.Client{
		Timeout: time.Duration(period) * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				LocalAddr: addr,
				Timeout:   time.Duration(period) * time.Second,
				KeepAlive: time.Duration(period) * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

var (
	dbURI       = "postgres://user:password@localhost/db"
	cfgFileName string
	period      = 30
	showHelp    = false
	ipsRaw      string
)

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

	var clients []*client
	if len(ipsRaw) > 0 {
		ips := strings.Split(ipsRaw, ",")
		clients = make([]*client, len(ips))
		for i, ip := range ips {
			addr, err := net.ResolveTCPAddr("tcp", ip+":0")
			if err != nil {
				fmt.Printf("Failed to resolve tcp address %s: %s\n", ip, err)
				os.Exit(1)
			}

			clients[i] = &client{c: setupClient(period, addr), a: addr}
		}
	} else {
		addr, err := getLocalAddr()
		if err != nil {
			fmt.Printf("Error: Failed to create connection to get the local address: %s", err)
			os.Exit(1)
		}

		clients = make([]*client, 1)
		clients[0] = &client{c: setupClient(period, nil), a: addr}
	}

	stopC := make(chan os.Signal, 2)
	signal.Notify(stopC, syscall.SIGTERM, syscall.SIGINT)

	shutdownC := make(chan struct{})

	wg := sync.WaitGroup{}
	for _, u := range urls {
		for _, c := range clients {
			wg.Add(1)
			go func(c *client, u string) {
				defer wg.Done()
				c.Check(u, time.Duration(period)*time.Second, shutdownC)
			}(c, u)
		}
	}

	sig := <-stopC
	fmt.Printf("Info: Received %s signal. Shutting down gracefully\n", sig)
	close(shutdownC)
	wg.Wait()
}
