package traffic

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

// MaxURLLength limit maximum stored URL size
const MaxURLLength = 1000

// MaxAccept limit maximum stored accept header size
const MaxAccept = 1000

const hotlinkGCPeriod = time.Hour * 1

// Hotlink Main Object
type Hotlink struct {
	db           *sql.DB
	loc          *time.Location
	queue        string
	conn         *amqp.Connection
	quit         chan bool
	gcStopTicker chan bool
	logger       *Logger
}

// HotlinkInputMessage HotlinkInputMessage
type HotlinkInputMessage struct {
	URL       string    `json:"url"`
	Accept    string    `json:"accept"`
	Timestamp time.Time `json:"timestamp"`
}

// NewHotlink constructor
func NewHotlink(wg *sync.WaitGroup, db *sql.DB, loc *time.Location, rabbitmMQ *amqp.Connection, queue string, logger *Logger) (*Hotlink, error) {
	s := &Hotlink{
		db:           db,
		loc:          loc,
		conn:         rabbitmMQ,
		queue:        queue,
		quit:         make(chan bool),
		gcStopTicker: make(chan bool),
		logger:       logger,
	}

	wg.Add(1)

	go func() {
		defer wg.Done()
		err := s.scheduleGC()
		if err != nil {
			s.logger.Warning(err)
			return
		}
		fmt.Println("Hotlink GC scheduler stopped")
	}()

	return s, nil
}

// Close all connections
func (s *Hotlink) Close() {
	s.gcStopTicker <- true
	close(s.gcStopTicker)

	s.quit <- true
	close(s.quit)

}

func (s *Hotlink) scheduleGC() error {
	gcTicker := time.NewTicker(hotlinkGCPeriod)
	for {
		select {
		case <-gcTicker.C:
			deleted, err := s.GC()
			if err != nil {
				return err
			}
			fmt.Printf("`%v` items of hotlinks deleted\n", deleted)
		case <-s.gcStopTicker:
			gcTicker.Stop()
			return nil
		}
	}
}

// Listen for incoming messages
func (s *Hotlink) listen() error {
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

			var message HotlinkInputMessage
			err := json.Unmarshal(d.Body, &message)
			if err != nil {
				s.logger.Warning(fmt.Errorf("failed to parse json `%v`: %s", err, d.Body))
				continue
			}

			err = s.Add(message.URL, message.Accept, message.Timestamp)
			if err != nil {
				s.logger.Warning(err)
			}

		case <-s.quit:
			quit = true
		}
	}

	return nil
}

// Add item to hotlinks
func (s *Hotlink) Add(uri string, accept string, timestamp time.Time) error {

	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	host := u.Host

	if len(host) <= 0 {
		return nil
	}

	whitelisted, err := s.IsHostWhitelisted(host)
	if err != nil {
		return err
	}

	if whitelisted {
		return nil
	}

	if len(uri) > MaxURLLength {
		uri = uri[:MaxURLLength]
	}

	stmt, err := s.db.Prepare(`
		INSERT INTO referer (host, url, count, last_date, accept)
		VALUES (?, ?, 1, ?, LEFT(?, ?))
		ON DUPLICATE KEY
		UPDATE count=count+1, host=VALUES(host), last_date=VALUES(last_date), accept=VALUES(accept)
	`)
	if err != nil {
		return err
	}
	defer Close(stmt)

	dateStr := timestamp.In(s.loc).Format("2006-01-02 15:04:05")
	_, err = stmt.Exec(host, uri, dateStr, accept, MaxAccept)
	if err != nil {
		return err
	}

	return nil
}

// GC Garbage Collect
func (s *Hotlink) GC() (int64, error) {

	stmt, err := s.db.Prepare("DELETE FROM referer WHERE day_date < DATE_SUB(?, INTERVAL 1 DAY)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(time.Now().In(s.loc).Format("2006-01-02 15:04:05"))
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

// IsHostWhitelisted IsHostWhitelisted
func (s *Hotlink) IsHostWhitelisted(host string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`
		SELECT true
		FROM referer_whitelist
		WHERE host = ?
	`, host).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
