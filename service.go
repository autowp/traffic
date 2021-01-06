package traffic

import (
	"context"
	"fmt"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/autowp/traffic/util"
	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // enable postgres migrations
	_ "github.com/golang-migrate/migrate/v4/source/file"       // enable file migration source
)

// Service Main Object
type Service struct {
	config     Config
	logger     *util.Logger
	db         *pgxpool.Pool
	rabbitMQ   *amqp.Connection
	waitGroup  *sync.WaitGroup
	router     *gin.Engine
	httpServer *http.Server
	Traffic    *Traffic
	pool       *pgxpool.Pool
}

// NewService constructor
func NewService(config Config) (*Service, error) {
	s := &Service{
		config:    config,
		logger:    util.NewLogger(config.Sentry),
		db:        nil,
		rabbitMQ:  nil,
		waitGroup: &sync.WaitGroup{},
		Traffic:   nil,
	}
	return s, nil
}

func (s *Service) initModel() error {
	if s.Traffic != nil {
		return nil
	}

	err := s.waitForDB()
	if err != nil {
		return err
	}

	traffic, err := NewTraffic(s.pool, s.logger)
	if err != nil {
		s.logger.Fatal(err)
		return err
	}

	s.Traffic = traffic

	return nil
}

func (s *Service) Migrate() error {
	err := s.waitForDB()
	if err != nil {
		return err
	}

	err = applyMigrations(s.config.Migrations)
	if err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

func (s *Service) Serve() error {

	err := s.initModel()
	if err != nil {
		return err
	}

	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(sentrygin.New(sentrygin.Options{}))

	s.Traffic.SetupRouter(r)

	s.router = r

	s.httpServer = &http.Server{Addr: s.config.HTTP.Listen, Handler: s.router}
	s.waitGroup.Add(1)
	go func() {
		defer s.waitGroup.Done()
		fmt.Println("HTTP server started")
		err := s.httpServer.ListenAndServe()
		if err != nil {
			// cannot panic, because this probably is an intentional close
			log.Printf("Httpserver: ListenAndServe() error: %s", err)
		}
		fmt.Println("HTTP server stopped")
	}()

	return nil
}

func (s *Service) SchedulerHourly() error {
	err := s.initModel()
	if err != nil {
		return err
	}

	deleted, err := s.Traffic.Monitoring.GC()
	if err != nil {
		s.logger.Fatal(err)
		return err
	}
	fmt.Printf("`%v` items of monitoring deleted\n", deleted)

	deleted, err = s.Traffic.Ban.GC()
	if err != nil {
		s.logger.Fatal(err)
		return err
	}
	fmt.Printf("`%v` items of ban deleted\n", deleted)

	err = s.Traffic.AutoWhitelist()
	if err != nil {
		s.logger.Warning(err)
		return err
	}

	return nil
}

func (s *Service) SchedulerMinutely() error {
	err := s.initModel()
	if err != nil {
		return err
	}

	err = s.Traffic.AutoBan()
	if err != nil {
		s.logger.Warning(err)
	}

	return nil
}

func (s *Service) ListenAMQP(quit chan bool) error {
	err := s.initModel()
	if err != nil {
		return err
	}

	err = s.waitForAMQP()
	if err != nil {
		return err
	}

	s.waitGroup.Add(1)
	go func() {
		defer s.waitGroup.Done()
		fmt.Println("Monitoring listener started")
		err := s.Traffic.Monitoring.Listen(s.rabbitMQ, s.config.MonitoringQueue, quit)
		if err != nil {
			s.logger.Fatal(err)
		}
		fmt.Println("Monitoring listener stopped")
	}()

	return nil
}

func (s *Service) waitForDB() error {

	if s.pool != nil {
		return nil
	}

	start := time.Now()
	timeout := 60 * time.Second

	fmt.Println("Waiting for postgres")

	var pool *pgxpool.Pool
	var err error
	for {
		pool, err = pgxpool.Connect(context.Background(), s.config.DSN)
		if err != nil {
			return err
		}

		db, err := pool.Acquire(context.Background())
		if err != nil {
			return err
		}

		err = db.Conn().Ping(context.Background())
		db.Release()
		if err == nil {
			fmt.Println("Started.")
			break
		}

		if time.Since(start) > timeout {
			return err
		}

		fmt.Println(err)
		fmt.Print(".")
		time.Sleep(100 * time.Millisecond)
	}

	s.pool = pool

	return nil
}

func (s *Service) waitForAMQP() error {

	if s.rabbitMQ != nil {
		return nil
	}

	start := time.Now()
	timeout := 60 * time.Second

	fmt.Println("Waiting for rabbitMQ")

	var rabbitMQ *amqp.Connection
	var err error
	for {
		rabbitMQ, err = amqp.Dial(s.config.RabbitMQ)
		if err == nil {
			fmt.Println("Started.")
			break
		}

		if time.Since(start) > timeout {
			return err
		}

		fmt.Print(".")
		time.Sleep(100 * time.Millisecond)
	}

	s.rabbitMQ = rabbitMQ

	return nil
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

	if s.httpServer != nil {
		err := s.httpServer.Shutdown(context.Background())
		if err != nil {
			panic(err) // failure/timeout shutting down the server gracefully
		}
	}

	s.waitGroup.Wait()

	if s.db != nil {
		s.db.Close()
	}

	if s.rabbitMQ != nil {
		err := s.rabbitMQ.Close()
		if err != nil {
			s.logger.Warning(err)
		}
	}
}

// GetRouter GetRouter
func (s *Service) GetRouter() *gin.Engine {
	return s.router
}
