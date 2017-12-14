
# Crawler

## Simple web server uptime stat collector

[![Docker Build Status](https://img.shields.io/docker/build/bpiddubnyi/crawler.svg)](https://hub.docker.com/r/bpiddubnyi/crawler/)
![Docker Automated build](https://img.shields.io/docker/automated/bpiddubnyi/crawler.svg)
[![Build Status](https://travis-ci.org/bpiddubnyi/crawler.svg?branch=master)](https://travis-ci.org/bpiddubnyi/crawler)

## Install

Native:

```sh
go get -u github.com/bpiddubnyi/crawler/...
```

Docker:

```sh
docker pull bpiddubnyi/crawler:latest
```

## Usage

Crawler consists of two components: `crawler` daemon itself and stat aggregator `crawler-stat`.

### crawler

`crawler` has plenty of configuration parameters, most of them has sane default values.

Required:

* `-db` postgres connection string in `postgres://user:password@host/db?params` format
* `-config` path to file with newline separated list of server URLs

Optional:

* `-ips` comma separated list of IP addersses, to connect to servers from. If empty, default address will be used. Notice that you need to have correct routing table setup to use more than one source IP.
* `-proxies` comma separated list if proxy server URLs. If empty, no proxy will be used.
* `-period` server check time period in seconds, 60 seconds by default
* `-retry` number of db connection attempts, convenient for use within docker-compose
* `-follow` follow HTTP redirects, false by default
* `-flush` db flush period in seconds, 5 seconds by default
* `-workers` number of workers per IP/proxy, 40 by default

Usage example:

```sh
crawler -db 'postgres://user:password@localhost:5432/db?sslmode=disable' -config ./urls.list -proxies 'https://127.0.0.1:8080,http://192.168.0.1:8080'
```

### crawler-stat

Usage: `crawler-stat [options] [urls...]`

URL list is optional, if no url provided stats for all servers will be aggregated.

Options:

Required:

* `-db` postgres connection string in `postgres://user:password@host/db?params` format
* `-from` starting time in `02.01.2006 15:04:05` or short `15:04:05` for current day

Optional:

* `-to` ending time, format is the same as in `-from`, current time by default

Usage example:

```sh
crawler-stat  -from '12.12.2017 12:00:00' -db 'postgres://crawl:crawl@psql/crawl?sslmode=disable' http://google.com http://vk.com http://ok.ru http://yadi.sk http://kinopoisk.ru http://google.ca http://gog.com
```

Example output:

```
http://gog.com [from 172.18.0.3:0]:
    whole time: 18h11m17.196925s
    uptime: 18h11m17.196925s (100.00%)
http://google.ca [from 172.18.0.3:0]:
    whole time: 18h12m7.303226s
    uptime: 18h12m7.303226s (100.00%)
http://google.com [from 172.18.0.3:0]:
    whole time: 18h12m7.367126s
    uptime: 18h12m7.367126s (100.00%)
http://kinopoisk.ru [from 172.18.0.3:0]:
    whole time: 18h12m22.729383s
    uptime: 5m10.333929s (0.47%)
    longest downtime 18h7m12.395454s:
        from: 2017-12-13 21:42:18.060736 +0000 UTC
        to:   2017-12-14 15:49:30.45619 +0000 UTC
http://ok.ru [from 172.18.0.3:0]:
    whole time: 18h11m16.983191s
    uptime: 5m9.64153s (0.47%)
    longest downtime 18h6m7.341661s:
        from: 2017-12-13 21:42:14.403822 +0000 UTC
        to:   2017-12-14 15:48:21.745483 +0000 UTC
http://vk.com [from 172.18.0.3:0]:
    whole time: 18h12m27.067851s
    uptime: 5m19.661897s (0.49%)
    longest downtime 18h7m7.405954s:
        from: 2017-12-13 21:42:24.176653 +0000 UTC
        to:   2017-12-14 15:49:31.582607 +0000 UTC
http://yadi.sk [from 172.18.0.3:0]:
    whole time: 18h12m20.327683s
    uptime: 5m10.758647s (0.47%)
    longest downtime 18h7m9.569036s:
        from: 2017-12-13 21:42:19.89158 +0000 UTC
        to:   2017-12-14 15:49:29.460616 +0000 UTC
```

For convenience there is an example of  `docker-compose` configuration provided:

```yml
version: '2'
services:
  crawler:
    build: .
    depends_on:
     - "psql"
    entrypoint:
     - crawler
     - -db=postgres://crawl:crawl@psql/crawl?sslmode=disable
     - -period=60
     - -config=/tmp/crawler_config
    volumes:
     - ./extra/example_config:/tmp/crawler_config

  psql:
    build: ./db/pq/
    environment:
     - POSTGRES_PASSWORD=crawl
     - POSTGRES_DB=crawl
     - POSTGRES_USER=crawl
```