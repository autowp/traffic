package traffic

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

const monitoringGCPeriod = time.Hour * 1

// Monitoring Main Object
type Monitoring struct {
	db           *sql.DB
	loc          *time.Location
	queue        string
	conn         *amqp.Connection
	quit         chan bool
	logger       *Logger
	gcStopTicker chan bool
}

// MonitoringInputMessage InputMessage
type MonitoringInputMessage struct {
	IP        net.IP    `json:"ip"`
	Timestamp time.Time `json:"timestamp"`
}

// NewMonitoring constructor
func NewMonitoring(wg *sync.WaitGroup, db *sql.DB, loc *time.Location, rabbitmMQ *amqp.Connection, queue string, logger *Logger) (*Monitoring, error) {
	s := &Monitoring{
		db:           db,
		loc:          loc,
		conn:         rabbitmMQ,
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
func (s *Monitoring) Close() {
	s.gcStopTicker <- true
	close(s.gcStopTicker)
	s.quit <- true
	close(s.quit)
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
			fmt.Printf("`%v` items of monitoring deleted\n", deleted)
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
	defer Close(ch)

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

// Add item to monitoring
func (s *Monitoring) Add(ip net.IP, timestamp time.Time) error {
	stmt, err := s.db.Prepare(`
		INSERT INTO ip_monitoring4 (day_date, hour, tenminute, minute, ip, count)
		VALUES (?, HOUR(?), FLOOR(MINUTE(?)/10), MINUTE(?), INET6_ATON(?), 1)
		ON DUPLICATE KEY UPDATE count=count+1
	`)
	if err != nil {
		return err
	}
	defer Close(stmt)

	dateStr := timestamp.In(s.loc).Format("2006-01-02 15:04:05")
	_, err = stmt.Exec(dateStr, dateStr, dateStr, dateStr, ip.String())
	if err != nil {
		return err
	}

	return nil
}

// GC Garbage Collect
func (s *Monitoring) GC() (int64, error) {

	stmt, err := s.db.Prepare("DELETE FROM ip_monitoring4 WHERE day_date < ?")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(time.Now().In(s.loc).Format("2006-01-02"))
	if err != nil {
		return 0, err
	}
	defer Close(stmt)

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return affected, nil
}

// ClearIP removes all data collected for IP
func (s *Monitoring) ClearIP(ip net.IP) error {
	stmt, err := s.db.Prepare("DELETE FROM ip_monitoring4 WHERE ip = INET6_ATON(?)")
	if err != nil {
		return err
	}
	defer Close(stmt)
	_, err = stmt.Exec(ip.String())
	if err != nil {
		return err
	}

	return nil
}

// ListOfTopIP ListOfTopIP
func (s *Monitoring) ListOfTopIP(limit int) ([]net.IP, error) {

	nowStr := time.Now().In(s.loc).Format("2006-01-02")

	rows, err := s.db.Query(`
		SELECT ip, SUM(count) AS c
		FROM ip_monitoring4
		WHERE day_date = ?
		GROUP BY ip
		ORDER BY c DESC
		LIMIT ?
	`, nowStr, limit)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

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

// ListByBanProfile ListByBanProfile
func (s *Monitoring) ListByBanProfile(profile AutobanProfile) ([]net.IP, error) {
	group := append([]string{"ip"}, profile.Group...)

	nowStr := time.Now().In(s.loc).Format("2006-01-02")

	rows, err := s.db.Query(`
		SELECT ip, SUM(count) AS c
		FROM ip_monitoring4
		WHERE day_date = ?
		GROUP BY `+strings.Join(group, ", ")+`
		HAVING c > ?
		LIMIT 1000
	`, nowStr, profile.Limit)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

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
	err := s.db.QueryRow(`
		SELECT 1
		FROM ip_monitoring4
		WHERE ip = INET6_ATON(?)
		LIMIT 1
	`, ip.String()).Scan(&exists)
	if err != nil {
		if err != sql.ErrNoRows {
			return false, err
		}

		return false, nil
	}

	return true, nil
}
