package pq

import (
	"database/sql"
	"time"

	"github.com/bpiddubnyi/crawler/db"
	_ "github.com/lib/pq"
)

type DB struct {
	conn *sql.DB
}

func New(uri string, retries int) (*DB, error) {
	var (
		res *DB = &DB{}
		err error
	)

	res.conn, err = sql.Open("postgres", uri)
	if err != nil {
		return nil, err
	}

	for i := 0; i < retries; i++ {
		err = res.conn.Ping()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return nil, err
	}

	return res, nil
}

func (db *DB) Write(flushPeriod time.Duration, rC <-chan db.Record, errC chan<- error) {
	tx, err := db.conn.Begin()
	if err != nil {
		errC <- err
		return
	}

	t := time.NewTicker(flushPeriod)
	defer t.Stop()

theLoop:
	for {
		select {
		case r, ok := <-rC:
			if !ok {
				break theLoop
			}
			_, err = tx.Exec("INSERT INTO uptime_log (url, time, local_ip, up) VALUES ($1, $2, $3, $4)",
				r.URL, r.Time.UTC(), r.LocalIP, r.Up)
			if err != nil {
				tx = nil
				break theLoop
			}
		case <-t.C:
			tx.Commit()
			tx, err = db.conn.Begin()
			if err != nil {
				break theLoop
			}
		}
	}

	if tx != nil {
		tx.Commit()
	}

	errC <- err
}
