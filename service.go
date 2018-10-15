package traffic

import (
	"database/sql"
	"fmt"
	"net"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql" // enable mysql driver
	"github.com/streadway/amqp"
)

const gcPeriod = time.Hour * 1
const whitelistPeriod = time.Hour * 1
const banPeriod = time.Minute
const banByUserID = 9

// Service Main Object
type Service struct {
	config              Config
	logger              *Logger
	db                  *sql.DB
	Whitelist           *Whitelist
	Ban                 *Ban
	Monitoring          *Monitoring
	Hotlink             *Hotlink
	Loc                 *time.Location
	whitelistStopTicker chan bool
	banStopTicker       chan bool
	rabbitMQ            *amqp.Connection
	waitGroup           *sync.WaitGroup
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

	wg := &sync.WaitGroup{}

	rabbitMQ, err := amqp.Dial(config.RabbitMQ)
	if err != nil {
		return nil, err
	}

	whitelist, err := NewWhitelist(db, loc)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	ban, err := NewBan(wg, db, loc, logger)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	monitoring, err := NewMonitoring(wg, db, loc, rabbitMQ, config.MonitoringQueue, logger)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	hotlink, err := NewHotlink(wg, db, loc, rabbitMQ, config.HotlinkQueue, logger)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	s := &Service{
		config:     config,
		logger:     logger,
		db:         db,
		Whitelist:  whitelist,
		Ban:        ban,
		Monitoring: monitoring,
		Loc:        loc,
		Hotlink:    hotlink,
		rabbitMQ:   rabbitMQ,
		waitGroup:  wg,
	}

	whitelistTicker := time.NewTicker(whitelistPeriod)
	s.whitelistStopTicker = make(chan bool)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-whitelistTicker.C:
				err := s.autoWhitelist()
				if err != nil {
					logger.Warning(err)
				}
			case <-s.whitelistStopTicker:
				whitelistTicker.Stop()
				return
			}
		}
	}()

	banTicker := time.NewTicker(banPeriod)
	s.banStopTicker = make(chan bool)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-banTicker.C:
				err := s.autoBan()
				if err != nil {
					logger.Warning(err)
				}
			case <-s.banStopTicker:
				banTicker.Stop()
				return
			}
		}
	}()

	return s, nil
}

// Close Destructor
func (s *Service) Close() {
	s.banStopTicker <- true
	close(s.banStopTicker)
	s.whitelistStopTicker <- true
	close(s.whitelistStopTicker)

	s.Monitoring.Close()
	s.Hotlink.Close()
	s.Ban.Close()

	s.waitGroup.Wait()

	err := s.db.Close()
	if err != nil {
		s.logger.Warning(err)
	}

	err = s.rabbitMQ.Close()
	if err != nil {
		s.logger.Warning(err)
	}
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
