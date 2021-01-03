package traffic

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/autowp/traffic/util"
	"github.com/gin-gonic/gin"
	pgx "github.com/jackc/pgx/v4"
	"github.com/streadway/amqp"

	"github.com/golang-migrate/migrate"
	_ "github.com/golang-migrate/migrate/database/postgres" // enable postgres migrations
	_ "github.com/golang-migrate/migrate/source/file"       // enable file migration source
)

const whitelistPeriod = time.Hour * 1
const banPeriod = time.Minute
const banByUserID = 9

// Service Main Object
type Service struct {
	config              Config
	logger              *util.Logger
	db                  *pgx.Conn
	Whitelist           *Whitelist
	Ban                 *Ban
	Monitoring          *Monitoring
	Loc                 *time.Location
	whitelistStopTicker chan bool
	banStopTicker       chan bool
	rabbitMQ            *amqp.Connection
	waitGroup           *sync.WaitGroup
	router              *gin.Engine
	httpServer          *http.Server
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
	{
		Limit:  10000,
		Reason: "daily limit",
		Group:  []string{},
		Time:   time.Hour * 10 * 24,
	},
	{
		Limit:  3600,
		Reason: "hourly limit",
		Group:  []string{"hour"},
		Time:   time.Hour * 5 * 24,
	},
	{
		Limit:  1200,
		Reason: "ten min limit",
		Group:  []string{"hour", "tenminute"},
		Time:   time.Hour * 24,
	},
	{
		Limit:  700,
		Reason: "min limit",
		Group:  []string{"hour", "tenminute", "minute"},
		Time:   time.Hour * 12,
	},
}

// NewService constructor
func NewService(config Config) (*Service, error) {

	var err error

	logger := util.NewLogger(config.Sentry)

	loc, _ := time.LoadLocation("UTC")

	start := time.Now()
	timeout := 60 * time.Second

	fmt.Println("Waiting for postgres")

	var db *pgx.Conn
	for {
		db, err = pgx.Connect(context.Background(), config.DSN)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		err = db.Ping(context.Background())
		if err == nil {
			fmt.Println("Started.")
			break
		}

		if time.Since(start) > timeout {
			logger.Fatal(err)
			return nil, err
		}

		fmt.Println(err)
		fmt.Print(".")
		time.Sleep(100 * time.Millisecond)
	}

	err = applyMigrations(config.Migrations)
	if err != nil && err != migrate.ErrNoChange {
		logger.Fatal(err)
		return nil, err
	}

	start = time.Now()
	timeout = 60 * time.Second

	fmt.Println("Waiting for rabbitMQ")

	var rabbitMQ *amqp.Connection
	for {
		rabbitMQ, err = amqp.Dial(config.RabbitMQ)
		if err == nil {
			fmt.Println("Started.")
			break
		}

		if time.Since(start) > timeout {
			logger.Fatal(err)
			return nil, err
		}

		fmt.Print(".")
		time.Sleep(100 * time.Millisecond)
	}

	wg := &sync.WaitGroup{}

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

	s := &Service{
		config:     config,
		logger:     logger,
		db:         db,
		Whitelist:  whitelist,
		Ban:        ban,
		Monitoring: monitoring,
		Loc:        loc,
		rabbitMQ:   rabbitMQ,
		waitGroup:  wg,
	}

	s.setupRouter()

	whitelistTicker := time.NewTicker(whitelistPeriod)
	s.whitelistStopTicker = make(chan bool)
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("AutoWhitelist scheduler started")
	loop:
		for {
			select {
			case <-whitelistTicker.C:
				err := s.autoWhitelist()
				if err != nil {
					logger.Warning(err)
				}
			case <-s.whitelistStopTicker:
				whitelistTicker.Stop()
				break loop
			}
		}
		fmt.Println("AutoWhitelist scheduler stopped")
	}()

	banTicker := time.NewTicker(banPeriod)
	s.banStopTicker = make(chan bool)
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("AutoBan scheduler started")
	loop:
		for {
			select {
			case <-banTicker.C:
				err := s.autoBan()
				if err != nil {
					logger.Warning(err)
				}
			case <-s.banStopTicker:
				banTicker.Stop()
				break loop
			}
		}

		fmt.Println("AutoBan scheduler stopped")
	}()

	s.httpServer = &http.Server{Addr: s.config.HTTP.Listen, Handler: s.router}
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("HTTP server started")
		err := s.httpServer.ListenAndServe()
		if err != nil {
			// cannot panic, because this probably is an intentional close
			log.Printf("Httpserver: ListenAndServe() error: %s", err)
		}
		fmt.Println("HTTP server stopped")
	}()

	return s, nil
}

func applyMigrations(config MigrationsConfig) error {
	fmt.Println("Apply migrations")

	dir := config.Dir
	if dir == "" {
		ex, err := os.Executable()
		if err != nil {
			return err
		}
		exPath := filepath.Dir(ex)
		dir = exPath + "/migrations"
	}

	m, err := migrate.New("file://"+dir, config.DSN)
	if err != nil {
		return err
	}

	err = m.Up()
	if err != nil {
		return err
	}
	fmt.Println("Migrations applied")

	return nil
}

// Close Destructor
func (s *Service) Close() {
	s.banStopTicker <- true
	close(s.banStopTicker)
	s.whitelistStopTicker <- true
	close(s.whitelistStopTicker)

	if s.httpServer != nil {
		err := s.httpServer.Shutdown(context.Background())
		if err != nil {
			panic(err) // failure/timeout shutting down the server gracefully
		}
	}

	s.Monitoring.Close()
	s.Ban.Close()

	s.waitGroup.Wait()

	if s.db != nil {
		err := s.db.Close(context.Background())
		if err != nil {
			s.logger.Warning(err)
		}
	}

	if s.rabbitMQ != nil {
		err := s.rabbitMQ.Close()
		if err != nil {
			s.logger.Warning(err)
		}
	}
}

func (s *Service) autoWhitelist() error {

	items, err := s.Monitoring.ListOfTop(1000)
	if err != nil {
		return err
	}

	for _, item := range items {
		fmt.Printf("Check IP %v\n", item.IP)
		if err := s.autoWhitelistIP(item.IP); err != nil {
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
