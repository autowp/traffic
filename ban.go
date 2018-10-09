package traffic

import (
	"database/sql"
	"net"
	"strings"
	"time"
)

// Ban Main Object
type Ban struct {
	db *sql.DB
}

// NewBan constructor
func NewBan(db *sql.DB) (*Ban, error) {
	return &Ban{
		db: db,
	}, nil
}

// Add IP to list of banned
func (s *Ban) Add(ip net.IP, duration time.Duration, byUserID int, reason string) error {
	reason = strings.TrimSpace(reason)
	upTo := time.Now().Add(duration)

	stmt, err := s.db.Prepare(`
		INSERT INTO banned_ip (ip, up_to, by_user_id, reason)
		VALUES (INET6_ATON(?), ?, ?, ?)
		ON DUPLICATE KEY UPDATE up_to=VALUES(up_to), by_user_id=VALUES(by_user_id), reason=VALUES(reason)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	upToStr := upTo.Format("2006-01-02 15:04:05")
	_, err = stmt.Exec(ip.String(), upToStr, byUserID, reason)
	if err != nil {
		return err
	}

	return nil
}

// Remove IP from list of banned
func (s *Ban) Remove(ip net.IP) error {

	stmt, err := s.db.Prepare("DELETE FROM banned_ip WHERE ip = INET6_ATON(?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ip.String())
	if err != nil {
		return err
	}
	defer stmt.Close()

	return nil
}

// Exists ban list already contains IP
func (s *Ban) Exists(ip net.IP) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`
		SELECT 1
		FROM banned_ip
		WHERE ip = INET6_ATON(?)
	`, ip.String()).Scan(&exists)
	if err != nil {
		if err != sql.ErrNoRows {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

// GC Grablage Collect
func (s *Ban) GC() (int64, error) {
	stmt, err := s.db.Prepare("DELETE FROM banned_ip WHERE up_to < NOW()")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec()
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
