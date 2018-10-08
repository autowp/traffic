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
	Ban        *Ban
	Monitoring *Monitoring
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

	ban, err := NewBan(db)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	monitoring, err := NewMonitoring(db)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	s := &Service{
		config:     config,
		log:        logger,
		db:         db,
		Whitelist:  whitelist,
		Ban:        ban,
		Monitoring: monitoring,
	}

	s.input = NewInput(config.Input, func(msg InputMessage) {
		err := s.Monitoring.Add(msg.IP, msg.Timestamp)
		if err != nil {
			s.log.Warning(err)
		}
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

func (s *Service) gc() {

	deletedIP, err := s.Monitoring.GC()
	if err != nil {
		log.Fatal(err)
		return
	}
	fmt.Printf("`%v` items of monitoring deleted\n", deletedIP)

	deletedBans, err := s.Ban.GC()
	if err != nil {
		log.Fatal(err)
		return
	}
	fmt.Printf("`%v` items of ban deleted\n", deletedBans)
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

	inWhitelist, err := s.Whitelist.Exists(ip)
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

	if err := s.Whitelist.Add(ip, desc); err != nil {
		return err
	}

	if err := s.Ban.Remove(ip); err != nil {
		return err
	}

	if err := s.Monitoring.ClearIP(ip); err != nil {
		return err
	}

	fmt.Println(" whitelisted")

	return nil
}
