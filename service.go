package traffic

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"time"

	_ "github.com/go-sql-driver/mysql" // enable mysql driver
)

const gcPeriod = time.Hour * 1

// Service Main Object
type Service struct {
	config     Config
	input      *Input
	log        *Logger
	db         *sql.DB
	stopTicker chan struct{}
	Whitelist  *Whitelist
}

// NewService constructor
func NewService(config Config) (*Service, error) {

	logger := NewLogger(config.Rollbar)

	db, err := sql.Open("mysql", config.DSN)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	whitelist, err := NewWhitelist(db)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	s := &Service{
		config:    config,
		log:       logger,
		db:        db,
		Whitelist: whitelist,
	}

	s.input = NewInput(config.Input, func(msg InputMessage) {
		s.pushHit(msg)
	}, func(err error) {
		s.log.Warning(err)
	})

	go func() {
		err := s.input.Listen()
		if err != nil {
			s.log.Fatal(err)
		}
	}()

	ticker := time.NewTicker(gcPeriod)
	s.stopTicker = make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				s.gc()
			case <-s.stopTicker:
				ticker.Stop()
				return
			}
		}
	}()

	s.gc()

	return s, nil
}

// Close Destructor
func (s *Service) Close() {
	s.input.Close()
	s.db.Close()
}

func (s *Service) pushHit(msg InputMessage) {
	stmt, err := s.db.Prepare(`
		INSERT INTO ip_monitoring4 (day_date, hour, tenminute, minute, ip, count)
		VALUES (?, HOUR(?), FLOOR(MINUTE(?)/10), MINUTE(?), INET6_ATON(?), 1)
		ON DUPLICATE KEY UPDATE count=count+1
	`)
	if err != nil {
		log.Fatal(err)
	}

	dateStr := msg.Timestamp.Format("2006-01-02 15:04:05")
	_, err = stmt.Exec(dateStr, dateStr, dateStr, dateStr, msg.IP.String())
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
}

func (s *Service) gc() int64 {

	stmt, err := s.db.Prepare("DELETE FROM ip_monitoring4 WHERE day_date < CURDATE()")
	if err != nil {
		log.Fatal(err)
	}
	res, err := stmt.Exec()
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	deletedIP, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}

	stmt, err = s.db.Prepare("DELETE FROM banned_ip WHERE up_to < NOW()")
	if err != nil {
		log.Fatal(err)
	}
	res, err = stmt.Exec()
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	deletedBans, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}

	return deletedIP + deletedBans
}

func (s *Service) clearIPMonitoring(ip net.IP) error {
	stmt, err := s.db.Prepare("DELETE FROM ip_monitoring4 WHERE ip = INET6_ATON(?)")
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

func (s *Service) unban(ip net.IP) error {

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

func (s *Service) autoWhitelist() error {

	rows, err := s.db.Query(`
		SELECT ip, SUM(count) AS count
		FROM ip_monitoring4
		WHERE day_date = CURDATE()
		GROUP BY ip
		ORDER count DESC
		LIMIT 1000
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var ip net.IP
		if err := rows.Scan(&ip); err != nil {
			return err
		}

		err := s.autoWhitelistIP(ip)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) autoWhitelistIP(ip net.IP) error {
	ipText := ip.String()

	fmt.Print(ipText + ": ")

	inWhitelist, err := s.Whitelist.exists(ip)
	if err != nil {
		return err
	}

	if inWhitelist {
		fmt.Println("whitelist, skip")
		return nil
	}

	match, desc := s.Whitelist.MatchAuto(ip)

	if !match {
		fmt.Println("")
		return nil
	}

	if err := s.Whitelist.add(ip, desc); err != nil {
		return err
	}

	if err := s.unban(ip); err != nil {
		return err
	}

	if err := s.clearIPMonitoring(ip); err != nil {
		return err
	}

	fmt.Println(" whitelisted")

	return nil
}
