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
const whitelistPeriod = time.Hour * 1
const banPeriod = time.Minute
const banByUserID = 9

// Service Main Object
type Service struct {
	config              Config
	input               *Input
	log                 *Logger
	db                  *sql.DB
	gcStopTicker        chan struct{}
	Whitelist           *Whitelist
	Ban                 *Ban
	Monitoring          *Monitoring
	Loc                 *time.Location
	whitelistStopTicker chan struct{}
	banStopTicker       chan struct{}
}

// AutobanProfile AutobanProfile
type AutobanProfile struct {
	Limit  int
	Reason string
	Group  []string
	Time   time.Duration
}

// AutobanProfiles AutobanProfiles
var AutobanProfiles = []AutobanProfile{
	AutobanProfile{
		Limit:  4000,
		Reason: "daily limit",
		Group:  []string{},
		Time:   time.Hour * 10 * 24,
	},
	AutobanProfile{
		Limit:  1800,
		Reason: "hourly limit",
		Group:  []string{"hour"},
		Time:   time.Hour * 5 * 24,
	},
	AutobanProfile{
		Limit:  600,
		Reason: "ten min limit",
		Group:  []string{"hour", "tenminute"},
		Time:   time.Hour * 24,
	},
	AutobanProfile{
		Limit:  150,
		Reason: "min limit",
		Group:  []string{"hour", "tenminute", "minute"},
		Time:   time.Hour * 12,
	},
}

// NewService constructor
func NewService(config Config) (*Service, error) {

	logger := NewLogger(config.Rollbar)

	loc, _ := time.LoadLocation("UTC")

	db, err := sql.Open("mysql", config.DSN)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	whitelist, err := NewWhitelist(db, loc)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	ban, err := NewBan(db, loc)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	monitoring, err := NewMonitoring(db, loc)
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
		Loc:        loc,
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

	gcTicker := time.NewTicker(gcPeriod)
	s.gcStopTicker = make(chan struct{})
	go func() {
		for {
			select {
			case <-gcTicker.C:
				s.gc()
			case <-s.gcStopTicker:
				gcTicker.Stop()
				return
			}
		}
	}()

	whitelistTicker := time.NewTicker(whitelistPeriod)
	s.whitelistStopTicker = make(chan struct{})
	go func() {
		for {
			select {
			case <-whitelistTicker.C:
				s.autoWhitelist()
			case <-s.whitelistStopTicker:
				whitelistTicker.Stop()
				return
			}
		}
	}()

	banTicker := time.NewTicker(banPeriod)
	s.banStopTicker = make(chan struct{})
	go func() {
		for {
			select {
			case <-banTicker.C:
				s.autoBan()
			case <-s.banStopTicker:
				banTicker.Stop()
				return
			}
		}
	}()

	s.gc()
	s.autoWhitelist()

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

	ips, err := s.Monitoring.ListOfTopIP(1000)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		fmt.Printf("Check IP %v\n", ip)
		if err := s.autoWhitelistIP(ip); err != nil {
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

	match, desc := s.Whitelist.MatchAuto(ip)

	if !match {
		fmt.Println("")
		return nil
	}

	if inWhitelist {
		fmt.Println("whitelist, skip")
	} else {
		if err := s.Whitelist.Add(ip, desc); err != nil {
			return err
		}
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

func (s *Service) autoBanByProfile(profile AutobanProfile) error {

	ips, err := s.Monitoring.ListByBanProfile(profile)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		exists, err := s.Whitelist.Exists(ip)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		fmt.Printf("%s %v\n", profile.Reason, ip)

		if err := s.Ban.Add(ip, profile.Time, banByUserID, profile.Reason); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) autoBan() error {
	for _, profile := range AutobanProfiles {
		if err := s.autoBanByProfile(profile); err != nil {
			return err
		}
	}

	return nil
}
