package traffic

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/autowp/traffic/util"
)

const banGCPeriod = time.Hour * 10

// BanItem BanItem
type BanItem struct {
	IP       net.IP    `json:"ip"`
	Until    time.Time `json:"up_to"`
	ByUserID int       `json:"by_user_id"`
	Reason   string    `json:"reason"`
}

// Ban Main Object
type Ban struct {
	db           *sql.DB
	loc          *time.Location
	gcStopTicker chan bool
	logger       *util.Logger
}

// NewBan constructor
func NewBan(wg *sync.WaitGroup, db *sql.DB, loc *time.Location, logger *util.Logger) (*Ban, error) {
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
		INSERT INTO ip_ban (ip, until, by_user_id, reason)
		VALUES (INET6_ATON(?), ?, ?, ?)
		ON DUPLICATE KEY UPDATE until=VALUES(until), by_user_id=VALUES(by_user_id), reason=VALUES(reason)
	`)
	if err != nil {
		return err
	}
	defer util.Close(stmt)

	upToStr := upTo.In(s.loc).Format("2006-01-02 15:04:05")
	_, err = stmt.Exec(ip.String(), upToStr, byUserID, reason)
	if err != nil {
		return err
	}

	return nil
}

// Remove IP from list of banned
func (s *Ban) Remove(ip net.IP) error {

	stmt, err := s.db.Prepare("DELETE FROM ip_ban WHERE ip = INET6_ATON(?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ip.String())
	if err != nil {
		return err
	}
	defer util.Close(stmt)

	return nil
}

// Exists ban list already contains IP
func (s *Ban) Exists(ip net.IP) (bool, error) {

	nowStr := time.Now().In(s.loc).Format("2006-01-02 15:04:05")

	var exists bool
	err := s.db.QueryRow(`
		SELECT 1
		FROM ip_ban
		WHERE ip = INET6_ATON(?) AND until >= ?
	`, ip.String(), nowStr).Scan(&exists)
	if err != nil {
		if err != sql.ErrNoRows {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

// Get ban info
func (s *Ban) Get(ip net.IP) (*BanItem, error) {

	nowStr := time.Now().In(s.loc).Format("2006-01-02 15:04:05")

	item := BanItem{}
	err := s.db.QueryRow(`
		SELECT ip, until, reason, by_user_id
		FROM ip_ban
		WHERE ip = INET6_ATON(?) AND until >= ?
	`, ip.String(), nowStr).Scan(&item.IP, &item.Until, &item.Reason, &item.ByUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}

	return &item, nil
}

// GC Grablage Collect
func (s *Ban) GC() (int64, error) {
	stmt, err := s.db.Prepare("DELETE FROM ip_ban WHERE until < ?")
	if err != nil {
		return 0, err
	}

	nowStr := time.Now().In(s.loc).Format("2006-01-02 15:04:05")
	res, err := stmt.Exec(nowStr)
	if err != nil {
		return 0, err
	}
	defer util.Close(stmt)

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return affected, nil
}

// Clear removes all collected data
func (s *Ban) Clear() error {
	stmt, err := s.db.Prepare("DELETE FROM ip_ban")
	if err != nil {
		return err
	}
	defer util.Close(stmt)
	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	return nil
}
