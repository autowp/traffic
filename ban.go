package traffic

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const banGCPeriod = time.Hour * 10

// Ban Main Object
type Ban struct {
	db           *sql.DB
	loc          *time.Location
	gcStopTicker chan bool
	logger       *Logger
}

// NewBan constructor
func NewBan(wg *sync.WaitGroup, db *sql.DB, loc *time.Location, logger *Logger) (*Ban, error) {
	s := &Ban{
		db:           db,
		loc:          loc,
		gcStopTicker: make(chan bool),
		logger:       logger,
	}

	wg.Add(1)
	gcTicker := time.NewTicker(banGCPeriod)
	go func() {
		defer wg.Done()

		fmt.Println("Ban GC scheduler started")
	loop:
		for {
			select {
			case <-gcTicker.C:
				deleted, err := s.GC()
				if err != nil {
					s.logger.Fatal(err)
					return
				}
				fmt.Printf("`%v` items of ban deleted\n", deleted)
			case <-s.gcStopTicker:
				gcTicker.Stop()
				break loop
			}
		}

		fmt.Println("Ban GC scheduler stopped")
	}()

	return s, nil
}

// Close all connections
func (s *Ban) Close() {
	s.gcStopTicker <- true
	close(s.gcStopTicker)
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
	defer Close(stmt)

	upToStr := upTo.In(s.loc).Format("2006-01-02 15:04:05")
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
	defer Close(stmt)

	return nil
}

// Exists ban list already contains IP
func (s *Ban) Exists(ip net.IP) (bool, error) {

	nowStr := time.Now().In(s.loc).Format("2006-01-02 15:04:05")

	var exists bool
	err := s.db.QueryRow(`
		SELECT 1
		FROM banned_ip
		WHERE ip = INET6_ATON(?) AND up_to >= ?
	`, ip.String(), nowStr).Scan(&exists)
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
	stmt, err := s.db.Prepare("DELETE FROM banned_ip WHERE up_to < ?")
	if err != nil {
		return 0, err
	}

	nowStr := time.Now().In(s.loc).Format("2006-01-02 15:04:05")
	res, err := stmt.Exec(nowStr)
	if err != nil {
		return 0, err
	}
	defer Close(stmt)

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return affected, nil
}
