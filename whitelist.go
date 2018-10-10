package traffic

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"
)

// Whitelist Main Object
type Whitelist struct {
	db  *sql.DB
	loc *time.Location
}

// NewWhitelist constructor
func NewWhitelist(db *sql.DB, loc *time.Location) (*Whitelist, error) {
	return &Whitelist{
		db:  db,
		loc: loc,
	}, nil
}

// MatchAuto MatchAuto
func (s *Whitelist) MatchAuto(ip net.IP) (bool, string) {

	ipText := ip.String()

	hosts, err := net.LookupAddr(ipText)
	if err != nil {
		return false, ""
	}

	for _, host := range hosts {

		fmt.Print(host + " ")

		ipWithDashes := strings.Replace(ipText, ".", "-", -1)

		msnHost := "msnbot-" + ipWithDashes + ".search.msn.com."
		yandexComHost := "spider-" + ipWithDashes + ".yandex.com."
		googlebotHost := "crawl-" + ipWithDashes + ".googlebot.com."

		if host == msnHost {
			return true, "msnbot autodetect"
		}
		if host == yandexComHost {
			return true, "yandex.com autodetect"
		}
		if host == googlebotHost {
			return true, "googlebot autodetect"
		}
	}

	return false, ""
}

// Add IP to whitelist
func (s *Whitelist) Add(ip net.IP, desc string) error {
	stmt, err := s.db.Prepare(`
		INSERT INTO ip_whitelist (ip, description)
		VALUES (INET6_ATON(?), ?)
		ON DUPLICATE KEY UPDATE description=VALUES(description)
	`)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(ip.String(), desc)
	if err != nil {
		return err
	}
	defer stmt.Close()

	return nil
}

// Exists whitelist already contains IP
func (s *Whitelist) Exists(ip net.IP) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`
		SELECT true
		FROM ip_whitelist
		WHERE ip = INET6_ATON(?)
	`, ip.String()).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
