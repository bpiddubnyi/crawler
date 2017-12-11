package pq

import (
	"database/sql"
	"time"

	"github.com/bpiddubnyi/crawler/db"
	"github.com/lib/pq"
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

func (d *DB) Write(flushPeriod time.Duration, rC <-chan *db.Record, errC chan<- error) {
	tx, err := d.conn.Begin()
	if err != nil {
		errC <- err
		return
	}

	stmt, err := tx.Prepare(pq.CopyIn("uptime_log", "url", "time", "local_ip", "up"))
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
			_, err = stmt.Exec(r.URL, r.Time.UTC(), r.LocalIP, r.Up)
			if err != nil {
				stmt = nil
				break theLoop
			}
		case <-t.C:
			_, err = stmt.Exec()
			if err != nil {
				stmt.Close()
				tx.Rollback()
				break
			}
			err = tx.Commit()
			if err != nil {
				break theLoop
			}

			tx, err = d.conn.Begin()
			if err != nil {
				break theLoop
			}

			stmt, err = tx.Prepare(pq.CopyIn("uptime_log", "url", "time", "local_ip", "up"))
			if err != nil {
				break theLoop
			}
		}
	}

	if stmt != nil {
		stmt.Exec()
		tx.Commit()
	}

	errC <- err
}

func (d *DB) GetRecords(from, to time.Time) ([]db.Record, error) {
	rows, err := d.conn.Query(`SELECT url, time, local_ip, up FROM uptime_log
		WHERE time >= $1 AND time <= $2
		ORDER BY time`, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}

	res := []db.Record{}
	for rows.Next() {
		r := db.Record{}
		err = rows.Scan(&r.URL, &r.Time, &r.LocalIP, &r.Up)
		if err != nil {
			return nil, err
		}
		res = append(res, r)
	}

	return res, nil
}

func (d *DB) GetDomainRecords(url string, from, to time.Time) ([]db.Record, error) {
	rows, err := d.conn.Query(`SELECT url, time, local_ip, up FROM uptime_log
		WHERE time >= $1 AND time <= $2 AND url=$3
		ORDER BY time`, from.UTC(), to.UTC(), url)
	if err != nil {
		return nil, err
	}

	res := []db.Record{}
	for rows.Next() {
		r := db.Record{}
		err = rows.Scan(&r.URL, &r.Time, &r.LocalIP, &r.Up)
		if err != nil {
			return nil, err
		}
		res = append(res, r)
	}

	return res, nil
}
