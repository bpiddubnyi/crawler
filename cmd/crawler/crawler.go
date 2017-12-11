package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/bpiddubnyi/crawler/db"
)

type client struct {
	c *http.Client
	a *net.TCPAddr
}

func (c *client) check(url string, rC chan<- *db.Record) {
	resp, err := c.c.Get(url)
	if err == nil {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
	rC <- &db.Record{URL: url, Time: time.Now(), LocalIP: c.a.IP.String(), Up: err == nil}
}

func (c *client) Check(wait time.Duration, url string, period time.Duration, rC chan<- *db.Record, shutdownC <-chan struct{}) {
	time.Sleep(wait)

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

func setupClient(timeout time.Duration, addr *net.TCPAddr, follow bool) *http.Client {
	checkRedirect := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if follow {
		checkRedirect = nil
	}

	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: checkRedirect,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				LocalAddr: addr,
				Timeout:   timeout,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:        1,
			TLSHandshakeTimeout: timeout,
			DisableKeepAlives:   true,
		},
	}
}

type crawler struct {
	clients []*client
	period  time.Duration
	w       db.Writer
}

func newCrawler(ips []string, period time.Duration, follow bool, w db.Writer) (*crawler, error) {
	res := &crawler{
		period: period,
		w:      w,
	}

	if len(ips) > 0 {
		res.clients = make([]*client, len(ips))
		for i, ip := range ips {
			addr, err := net.ResolveTCPAddr("tcp", ip+":0")
			if err != nil {
				return nil, fmt.Errorf("Failed to resolve tcp address %s: %s", ip, err)
			}

			res.clients[i] = &client{c: setupClient(period-period/3, addr, follow), a: addr}
		}
	} else {
		addr, err := getLocalAddr()
		if err != nil {
			return nil, fmt.Errorf("Failed to create connection to get the local address: %s", err)
		}

		res.clients = make([]*client, 1)
		res.clients[0] = &client{c: setupClient(period-period/3, nil, follow), a: addr}
	}

	return res, nil
}

func (c *crawler) Crawl(urls []string, shutdownC <-chan struct{}) error {
	rC := make(chan *db.Record, 500)
	errC := make(chan error)
	stopC := make(chan struct{})

	go c.w.Write(5*time.Second, rC, errC)

	wg := sync.WaitGroup{}

	for i, url := range urls {
		for _, cl := range c.clients {
			wg.Add(1)
			go func(cl *client, url string) {
				cl.Check((c.period*time.Duration(i))/time.Duration(len(urls)), url, c.period, rC, stopC)
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
