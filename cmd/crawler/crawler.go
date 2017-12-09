package main

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/bpiddubnyi/uptime/db"
)

type client struct {
	c *http.Client
	a *net.TCPAddr
}

func (c *client) check(url string, rC chan<- db.Record) {
	rec := db.Record{URL: url, Time: time.Now(), LocalIP: c.a.IP.String()}
	_, err := c.c.Get(url)
	if err == nil {
		rec.Up = true
	}
	rC <- rec
}

func (c *client) Check(url string, period time.Duration, rC chan<- db.Record, shutdownC <-chan struct{}) {
	tick := time.NewTicker(period)
	defer tick.Stop()

	wg := sync.WaitGroup{}

theLoop:
	for {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.check(url, rC)
		}()

		select {
		case <-tick.C:
			continue theLoop

		case _, ok := <-shutdownC:
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

func setupClient(period time.Duration, addr *net.TCPAddr) *http.Client {
	return &http.Client{
		Timeout: period,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				LocalAddr: addr,
				Timeout:   period,
				KeepAlive: period,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

type crawler struct {
	urls    []string
	clients []*client
	period  time.Duration
	w       db.Writer
}

func newCrawler(urls, ips []string, period time.Duration, w db.Writer) (*crawler, error) {
	res := &crawler{
		urls:   urls,
		period: period,
		w:      w,
	}

	if len(ips) > 0 {
		res.clients = make([]*client, len(ips))
		for i, ip := range ips {
			addr, err := net.ResolveTCPAddr("tcp", ip+":0")
			if err != nil {
				return nil, fmt.Errorf("Failed to resolve tcp address %s: %s\n", ip, err)
			}

			res.clients[i] = &client{c: setupClient(period, addr), a: addr}
		}
	} else {
		addr, err := getLocalAddr()
		if err != nil {
			return nil, fmt.Errorf("Failed to create connection to get the local address: %s", err)
		}

		res.clients = make([]*client, 1)
		res.clients[0] = &client{c: setupClient(period, nil), a: addr}
	}

	return res, nil
}

func (c *crawler) Crawl(shutdownC <-chan struct{}) error {
	rC := make(chan db.Record, 500)
	errC := make(chan error)
	stopC := make(chan struct{})

	go c.w.Write(c.period, rC, errC)

	wg := sync.WaitGroup{}

	for _, cl := range c.clients {
		for _, url := range c.urls {
			wg.Add(1)
			go func(cl *client, url string) {
				cl.Check(url, c.period, rC, stopC)
				wg.Done()
			}(cl, url)
		}
	}

	var err error

	select {
	case _, ok := <-shutdownC:
		if !ok {
			close(stopC)
			wg.Wait()
			close(rC)
			err = <-errC
			break
		}
	case err = <-errC:
		close(stopC)
		wg.Wait()
		close(rC)
		break
	}

	close(errC)
	return err
}
