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

const whitelistPeriod = time.Hour * 1
const banPeriod = time.Minute

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
}

// NewService constructor
func NewService(config Config) (*Service, error) {

	var err error

	logger := util.NewLogger(config.Sentry)

	pool, err := waitForDB(config.DSN)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	err = applyMigrations(config.Migrations)
	if err != nil && err != migrate.ErrNoChange {
		logger.Fatal(err)
		return nil, err
	}

	rabbitMQ, err := waitForAMQP(config.RabbitMQ)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	wg := &sync.WaitGroup{}

	traffic, err := NewTraffic(wg, pool, rabbitMQ, logger, config.MonitoringQueue)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	s := &Service{
		config:    config,
		logger:    logger,
		db:        pool,
		rabbitMQ:  rabbitMQ,
		waitGroup: wg,
		Traffic:   traffic,
	}

	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(sentrygin.New(sentrygin.Options{}))

	s.Traffic.SetupRouter(r)

	s.router = r

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

func waitForDB(dsn string) (*pgxpool.Pool, error) {
	start := time.Now()
	timeout := 60 * time.Second

	fmt.Println("Waiting for postgres")

	var pool *pgxpool.Pool
	var err error
	for {
		pool, err = pgxpool.Connect(context.Background(), dsn)
		if err != nil {
			return nil, err
		}

		db, err := pool.Acquire(context.Background())
		if err != nil {
			return nil, err
		}

		err = db.Conn().Ping(context.Background())
		db.Release()
		if err == nil {
			fmt.Println("Started.")
			break
		}

		if time.Since(start) > timeout {
			return nil, err
		}

		fmt.Println(err)
		fmt.Print(".")
		time.Sleep(100 * time.Millisecond)
	}

	return pool, nil
}

func waitForAMQP(url string) (*amqp.Connection, error) {
	start := time.Now()
	timeout := 60 * time.Second

	fmt.Println("Waiting for rabbitMQ")

	var rabbitMQ *amqp.Connection
	var err error
	for {
		rabbitMQ, err = amqp.Dial(url)
		if err == nil {
			fmt.Println("Started.")
			break
		}

		if time.Since(start) > timeout {
			return nil, err
		}

		fmt.Print(".")
		time.Sleep(100 * time.Millisecond)
	}

	return rabbitMQ, nil
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

	err := s.Traffic.Close()
	if err != nil {
		s.logger.Warning(err)
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
