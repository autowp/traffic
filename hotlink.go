package traffic

import (
	"database/sql"
	"net/url"
	"time"
)

const MAX_URL = 1000
const MAX_ACCEPT = 1000

// Hotlink Main Object
type Hotlink struct {
	db  *sql.DB
	loc *time.Location
}

// NewHotlink constructor
func NewHotlink(db *sql.DB, loc *time.Location) (*Hotlink, error) {
	return &Hotlink{
		db:  db,
		loc: loc,
	}, nil
}

// Add item to hotlinks
func (s *Hotlink) Add(uri string, accept string, timestamp time.Time) error {

	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	host := u.Host

	if len(host) <= 0 {
		return nil
	}

	whitelisted, err := s.IsHostWhitelisted(host)
	if err != nil {
		return err
	}

	if whitelisted {
		return nil
	}

	if len(uri) > MAX_URL {
		uri = uri[:MAX_URL]
	}

	stmt, err := s.db.Prepare(`
		INSERT INTO referer (host, url, count, last_date, accept)
		VALUES (?, ?, 1, ?, LEFT(?, ?))
		ON DUPLICATE KEY
		UPDATE count=count+1, host=VALUES(host), last_date=VALUES(last_date), accept=VALUES(accept)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	dateStr := timestamp.In(s.loc).Format("2006-01-02 15:04:05")
	_, err = stmt.Exec(host, uri, dateStr, accept, MAX_ACCEPT)
	if err != nil {
		return err
	}

	return nil
}

// GC Garbage Collect
func (s *Hotlink) GC() (int64, error) {

	stmt, err := s.db.Prepare("DELETE FROM referer WHERE day_date < DATE_SUB(?, INTERVAL 1 DAY)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(time.Now().In(s.loc).Format("2006-01-02 15:04:05"))
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return affected, nil
}

// IsHostWhitelisted IsHostWhitelisted
func (s *Hotlink) IsHostWhitelisted(host string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`
		SELECT true
		FROM referer_whitelist
		WHERE host = ?
	`, host).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
