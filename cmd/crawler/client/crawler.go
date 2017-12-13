package client

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/bpiddubnyi/crawler/db"
)

type client struct {
	c *http.Client
	a string
}

func (c *client) Check(urlC <-chan string, rC chan<- *db.Record) {
	for url := range urlC {
		resp, err := c.c.Get(url)
		if err == nil {
			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
		} else {
			log.Println(err)
		}
		rC <- &db.Record{URL: url, Time: time.Now(), LocalIP: c.a, Up: err == nil}
	}
}

func getLocalAddr() (*net.TCPAddr, error) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return nil, err
	}
	return &net.TCPAddr{IP: conn.LocalAddr().(*net.UDPAddr).IP}, nil
}

func setupClient(timeout time.Duration, addr net.Addr, proxy *url.URL, follow bool) *http.Client {
	checkRedirect := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if follow {
		checkRedirect = nil
	}

	p := http.ProxyFromEnvironment
	if proxy != nil {
		p = func(*http.Request) (*url.URL, error) {
			return proxy, nil
		}
	}

	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: checkRedirect,
		Transport: &http.Transport{
			Proxy: p,
			DialContext: (&net.Dialer{
				LocalAddr: addr,
				Timeout:   timeout / 2,
			}).DialContext,
			MaxIdleConns:        1,
			TLSHandshakeTimeout: 10 * time.Second,
			DisableKeepAlives:   true,
		},
	}
}

// Client is a HTTP client wrapper
type Client struct {
	clients []*client
	period  time.Duration
	w       db.Writer
}

// New creates new crawler with one or more HTTP clients depending on number of ip addresses
// passed in ips.
// If ips is empty or nil, single client will be created with no IP specified.
// Period impacts both TCP and HTTP timeouts and actual url status check period.
// If follow is true, HTTP client will follow HTTP redirects.
func New(ips []string, proxies []string, period time.Duration, follow bool, w db.Writer) (*Client, error) {
	res := &Client{
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

			res.clients[i] = &client{c: setupClient(period, addr, nil, follow), a: addr.String()}
		}
	}

	if len(proxies) > 0 {
		for _, proxy := range proxies {
			proxyURL, err := url.Parse(proxy)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse proxy URL %s: %s", proxy, err)
			}
			res.clients = append(res.clients, &client{c: setupClient(period, nil, proxyURL, follow), a: proxyURL.String()})
		}
	}

	if len(res.clients) == 0 {
		addr, err := getLocalAddr()
		if err != nil {
			return nil, fmt.Errorf("Failed to create connection to get the local address: %s", err)
		}

		res.clients = make([]*client, 1)
		res.clients[0] = &client{c: setupClient(period-period/3, nil, nil, follow), a: addr.String()}
	}

	return res, nil
}

// Crawl periodically creates HTTP GET requests to urls, checks if it's
// possible to get any correct HTTP response, and saves result (is server up
// or down) to db.
func (c *Client) Crawl(urls []string, flushPeriod time.Duration, nWorkers int, shutdownC <-chan struct{}) error {
	rC := make(chan *db.Record, 500)
	errC := make(chan error)
	urlC := make(chan string, 500)
	t := time.NewTicker(c.period)
	defer t.Stop()

	go c.w.Write(flushPeriod, rC, errC)

	wg := sync.WaitGroup{}

	for _, cl := range c.clients {
		for i := 0; i < nWorkers; i++ {
			wg.Add(1)
			go func(c *client) {
				c.Check(urlC, rC)
				wg.Done()
			}(cl)
		}
	}

	var err error

theLoop:
	for {
		for _, url := range urls {
			select {
			case _, ok := <-shutdownC:
				if !ok {
					break theLoop
				}
			case urlC <- url:
				continue
			}
		}

		select {
		case <-t.C:
			continue theLoop
		case _, ok := <-shutdownC:
			if !ok {
				break theLoop
			}
		case err = <-errC:
			close(errC)
			errC = nil
			break theLoop
		}
	}

	close(urlC)
	wg.Wait()
	close(rC)
	if errC != nil {
		err = <-errC
		close(errC)
	}

	return err
}
