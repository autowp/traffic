package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/autowp/traffic/util"
	"github.com/streadway/amqp"
)

const monitoringGCPeriod = time.Hour * 1

// Monitoring Main Object
type Monitoring struct {
	db           *pgxpool.Pool
	queue        string
	conn         *amqp.Connection
	quit         chan bool
	logger       *util.Logger
	gcStopTicker chan bool
}

// MonitoringInputMessage InputMessage
type MonitoringInputMessage struct {
	IP        net.IP    `json:"ip"`
	Timestamp time.Time `json:"timestamp"`
}

// ListOfTopItem ListOfTopItem
type ListOfTopItem struct {
	IP    net.IP `json:"ip"`
	Count int    `json:"count"`
}

// NewMonitoring constructor
func NewMonitoring(wg *sync.WaitGroup, db *pgxpool.Pool, rabbitMQ *amqp.Connection, queue string, logger *util.Logger) (*Monitoring, error) {
	s := &Monitoring{
		db:           db,
		conn:         rabbitMQ,
		queue:        queue,
		quit:         make(chan bool),
		logger:       logger,
		gcStopTicker: make(chan bool),
	}

	wg.Add(1)

	go func() {
		defer wg.Done()
		fmt.Println("Monitoring GC scheduler started")
		err := s.scheduleGC()
		if err != nil {
			s.logger.Warning(err)
			return
		}
		fmt.Println("Monitoring GC scheduler stopped")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("Monitoring listener started")
		err := s.listen()
		if err != nil {
			s.logger.Fatal(err)
		}
		fmt.Println("Monitoring listener stopped")
	}()

	return s, nil
}

// Close all connections
func (s *Monitoring) Close() error {
	s.gcStopTicker <- true
	close(s.gcStopTicker)
	s.quit <- true
	close(s.quit)

	return nil
}

func (s *Monitoring) scheduleGC() error {
	gcTicker := time.NewTicker(monitoringGCPeriod)

	for {
		select {
		case <-gcTicker.C:
			deleted, err := s.GC()
			if err != nil {
				return err
			}
			fmt.Printf("`%v` items of Monitoring deleted\n", deleted)
		case <-s.gcStopTicker:
			gcTicker.Stop()
			return nil
		}
	}
}

// Listen for incoming messages
func (s *Monitoring) listen() error {
	if s.conn == nil {
		return fmt.Errorf("RabbitMQ connection not initialized")
	}

	ch, err := s.conn.Channel()
	if err != nil {
		return err
	}
	defer util.Close(ch)

	inQ, err := ch.QueueDeclare(
		s.queue, // name
		false,   // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume(
		inQ.Name, // queue
		"",       // consumer
		true,     // auto-ack
		false,    // exclusive
		false,    // no-local
		false,    // no-wait
		nil,      // args
	)
	if err != nil {
		return err
	}

	quit := false
	for !quit {
		select {
		case d := <-msgs:
			if d.ContentType != "application/json" {
				s.logger.Warning(fmt.Errorf("unexpected mime `%s`", d.ContentType))
				continue
			}

			var message MonitoringInputMessage
			err := json.Unmarshal(d.Body, &message)
			if err != nil {
				s.logger.Warning(fmt.Errorf("failed to parse json `%v`: %s", err, d.Body))
				continue
			}

			err = s.Add(message.IP, message.Timestamp)
			if err != nil {
				s.logger.Warning(err)
			}

		case <-s.quit:
			quit = true
		}
	}

	return nil
}

// Add item to Monitoring
func (s *Monitoring) Add(ip net.IP, timestamp time.Time) error {

	_, err := s.db.Exec(context.Background(), `
		INSERT INTO ip_monitoring (day_date, hour, tenminute, minute, ip, count)
		VALUES (
			$1::timestamptz,
			EXTRACT(HOUR FROM $1::timestamptz),
			FLOOR(EXTRACT(MINUTE FROM $1::timestamptz)/10),
			EXTRACT(MINUTE FROM $1::timestamptz),
			$2,
			1
		)
		ON CONFLICT(ip,day_date,hour,tenminute,minute) DO UPDATE SET count=ip_monitoring.count+1
	`, timestamp, ip)

	return err
}

// GC Garbage Collect
func (s *Monitoring) GC() (int64, error) {

	ct, err := s.db.Exec(context.Background(), "DELETE FROM ip_monitoring WHERE day_date < CURRENT_DATE")
	if err != nil {
		return 0, err
	}

	affected := ct.RowsAffected()

	return affected, nil
}

// Clear removes all collected data
func (s *Monitoring) Clear() error {
	_, err := s.db.Exec(context.Background(), "DELETE FROM ip_monitoring")
	return err
}

// ClearIP removes all data collected for IP
func (s *Monitoring) ClearIP(ip net.IP) error {
	_, err := s.db.Exec(context.Background(), "DELETE FROM ip_monitoring WHERE ip = $1", ip)

	return err
}

// ListOfTop ListOfTop
func (s *Monitoring) ListOfTop(limit int) ([]ListOfTopItem, error) {

	rows, err := s.db.Query(context.Background(), `
		SELECT ip, SUM(count) AS c
		FROM ip_monitoring
		WHERE day_date = CURRENT_DATE
		GROUP BY ip
		ORDER BY c DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ListOfTopItem{}

	for rows.Next() {
		var item ListOfTopItem
		if err := rows.Scan(&item.IP, &item.Count); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	return result, nil
}

// ListByBanProfile ListByBanProfile
func (s *Monitoring) ListByBanProfile(profile AutobanProfile) ([]net.IP, error) {
	group := append([]string{"ip"}, profile.Group...)

	rows, err := s.db.Query(context.Background(), `
		SELECT ip, SUM(count) AS c
		FROM ip_monitoring
		WHERE day_date = CURRENT_DATE
		GROUP BY `+strings.Join(group, ", ")+`
		HAVING SUM(count) > $1
		LIMIT 1000
	`, profile.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []net.IP{}

	for rows.Next() {
		var ip net.IP
		var c int
		if err := rows.Scan(&ip, &c); err != nil {
			return nil, err
		}

		result = append(result, ip)
	}

	return result, nil
}

// ExistsIP ban list already contains IP
func (s *Monitoring) ExistsIP(ip net.IP) (bool, error) {
	var exists bool
	err := s.db.QueryRow(context.Background(), `
		SELECT true
		FROM ip_monitoring
		WHERE ip = $1
		LIMIT 1
	`, ip).Scan(&exists)
	if err != nil {
		if err != pgx.ErrNoRows {
			return false, err
		}

		return false, nil
	}

	return true, nil
}
