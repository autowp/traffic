package traffic

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
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
	db           *pgxpool.Pool
	gcStopTicker chan bool
	logger       *util.Logger
}

// NewBan constructor
func NewBan(wg *sync.WaitGroup, db *pgxpool.Pool, logger *util.Logger) (*Ban, error) {

	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	s := &Ban{
		db:           db,
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
func (s *Ban) Close() error {
	s.gcStopTicker <- true
	close(s.gcStopTicker)

	return nil
}

// Add IP to list of banned
func (s *Ban) Add(ip net.IP, duration time.Duration, byUserID int, reason string) error {
	reason = strings.TrimSpace(reason)
	upTo := time.Now().Add(duration)

	ct, err := s.db.Exec(context.Background(), `
		INSERT INTO ip_ban (ip, until, by_user_id, reason)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT(ip) DO UPDATE SET until=EXCLUDED.until, by_user_id=EXCLUDED.by_user_id, reason=EXCLUDED.reason
	`, ip, upTo, byUserID, reason)
	if err != nil {
		return err
	}

	affected := ct.RowsAffected()

	if affected == 1 {
		s.logger.Warningf("%v was banned. Reason: %s", ip.String(), reason)
	}

	return nil
}

// Remove IP from list of banned
func (s *Ban) Remove(ip net.IP) error {

	_, err := s.db.Exec(context.Background(), "DELETE FROM ip_ban WHERE ip = $1", ip)

	return err
}

// Exists ban list already contains IP
func (s *Ban) Exists(ip net.IP) (bool, error) {

	var exists bool
	err := s.db.QueryRow(context.Background(), `
		SELECT true
		FROM ip_ban
		WHERE ip = $1 AND until >= NOW()
	`, ip).Scan(&exists)
	if err != nil {
		if err != pgx.ErrNoRows {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

// Get ban info
func (s *Ban) Get(ip net.IP) (*BanItem, error) {

	item := BanItem{}
	err := s.db.QueryRow(context.Background(), `
		SELECT ip, until, reason, by_user_id
		FROM ip_ban
		WHERE ip = $1 AND until >= NOW()
	`, ip).Scan(&item.IP, &item.Until, &item.Reason, &item.ByUserID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}

	return &item, nil
}

// GC Garbage Collect
func (s *Ban) GC() (int64, error) {
	ct, err := s.db.Exec(context.Background(), "DELETE FROM ip_ban WHERE until < NOW()")
	if err != nil {
		return 0, err
	}

	affected := ct.RowsAffected()

	return affected, nil
}

// Clear removes all collected data
func (s *Ban) Clear() error {
	_, err := s.db.Exec(context.Background(), "DELETE FROM ip_ban")

	return err
}
