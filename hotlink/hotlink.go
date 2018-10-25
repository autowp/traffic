package hotlink

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/autowp/traffic/util"
	"github.com/streadway/amqp"
)

// MaxURLLength limit maximum stored URL size
const MaxURLLength = 1000

// MaxAccept limit maximum stored accept header size
const MaxAccept = 1000

const gcPeriod = time.Hour * 1

// Hotlink Main Object
type Hotlink struct {
	db           *sql.DB
	loc          *time.Location
	queue        string
	conn         *amqp.Connection
	quit         chan bool
	gcStopTicker chan bool
	logger       *util.Logger
}

// InputMessage InputMessage
type InputMessage struct {
	URL       string    `json:"url"`
	Accept    string    `json:"accept"`
	Timestamp time.Time `json:"timestamp"`
}

// MonitoringItem MonitoringItem
type MonitoringItem struct {
	URL    string `json:"url"`
	Accept string `json:"accept"`
	Count  int    `json:"count"`
}

// TopItem TopItem
type TopItem struct {
	Host        string           `json:"host"`
	Count       int              `json:"count"`
	Whitelisted bool             `json:"whitelisted"`
	Blacklisted bool             `json:"blacklisted"`
	Links       []MonitoringItem `json:"links"`
}

// New constructor
func New(wg *sync.WaitGroup, db *sql.DB, loc *time.Location, rabbitmMQ *amqp.Connection, queue string, logger *util.Logger) (*Hotlink, error) {
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
		fmt.Println("Hotlink GC scheduler started")
		err := s.scheduleGC()
		if err != nil {
			s.logger.Warning(err)
			return
		}
		fmt.Println("Hotlink GC scheduler stopped")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("Hotlink listener started")
		err := s.listen()
		if err != nil {
			s.logger.Fatal(err)
		}
		fmt.Println("Hotlink listener stopped")
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
	gcTicker := time.NewTicker(gcPeriod)
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

			var message InputMessage
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
		INSERT INTO hotlink_referer (host, url, count, last_date, accept)
		VALUES (?, ?, 1, ?, LEFT(?, ?))
		ON DUPLICATE KEY
		UPDATE count=count+1, host=VALUES(host), last_date=VALUES(last_date), accept=VALUES(accept)
	`)
	if err != nil {
		return err
	}
	defer util.Close(stmt)

	dateStr := timestamp.In(s.loc).Format("2006-01-02 15:04:05")
	_, err = stmt.Exec(host, uri, dateStr, accept, MaxAccept)
	if err != nil {
		return err
	}

	return nil
}

// GC Garbage Collect
func (s *Hotlink) GC() (int64, error) {

	stmt, err := s.db.Prepare("DELETE FROM hotlink_referer WHERE last_date < DATE_SUB(?, INTERVAL 1 DAY)")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec(time.Now().In(s.loc).Format("2006-01-02 15:04:05"))
	if err != nil {
		return 0, err
	}
	defer util.Close(stmt)

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
		FROM hotlink_whitelist
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

// IsHostBlacklisted IsHostBlacklisted
func (s *Hotlink) IsHostBlacklisted(host string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`
		SELECT true
		FROM hotlink_blacklist
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

// AddToWhitelist adds item to hotlinks whitelist
func (s *Hotlink) AddToWhitelist(host string) error {

	err := s.DeleteFromBlacklist(host)
	if err != nil {
		return err
	}

	stmt, err := s.db.Prepare(`
		INSERT IGNORE INTO hotlink_whitelist (host)
		VALUES (?)
	`)
	if err != nil {
		return err
	}
	defer util.Close(stmt)

	_, err = stmt.Exec(host)
	if err != nil {
		return err
	}

	return nil
}

// AddToBlacklist adds item to hotlinks blacklist
func (s *Hotlink) AddToBlacklist(host string) error {

	err := s.DeleteFromWhitelist(host)
	if err != nil {
		return err
	}

	stmt, err := s.db.Prepare(`
		INSERT IGNORE INTO hotlink_blacklist (host)
		VALUES (?)
	`)
	if err != nil {
		return err
	}
	defer util.Close(stmt)

	_, err = stmt.Exec(host)
	if err != nil {
		return err
	}

	return nil
}

// DeleteFromWhitelist removes item from hotlinks whitelist
func (s *Hotlink) DeleteFromWhitelist(host string) error {

	stmt, err := s.db.Prepare("DELETE FROM hotlink_whitelist WHERE host = ?")
	if err != nil {
		return err
	}
	defer util.Close(stmt)

	_, err = stmt.Exec(host)
	if err != nil {
		return err
	}

	return nil
}

// DeleteFromBlacklist removes item from hotlinks blacklist
func (s *Hotlink) DeleteFromBlacklist(host string) error {

	stmt, err := s.db.Prepare("DELETE FROM hotlink_blacklist WHERE host = ?")
	if err != nil {
		return err
	}
	defer util.Close(stmt)

	_, err = stmt.Exec(host)
	if err != nil {
		return err
	}

	return nil
}

// Clear Clear
func (s *Hotlink) Clear() error {
	stmt, err := s.db.Prepare("DELETE FROM hotlink_referer")
	if err != nil {
		return err
	}
	defer util.Close(stmt)

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	return nil
}

// DeleteByHost DeleteByHost
func (s *Hotlink) DeleteByHost(host string) error {
	stmt, err := s.db.Prepare("DELETE FROM hotlink_referer WHERE host = ?")
	if err != nil {
		return err
	}
	defer util.Close(stmt)

	_, err = stmt.Exec(host)
	if err != nil {
		return err
	}

	return nil
}

func (s *Hotlink) getMonitoringItems(host string, count int) ([]MonitoringItem, error) {
	nowStr := time.Now().In(s.loc).Format("2006-01-02")

	rows, err := s.db.Query(`
		SELECT url, count, accept
		FROM hotlink_referer
		WHERE last_date >= DATE_SUB(?, INTERVAL 1 DAY) AND host = ?
		ORDER BY count DESC
		LIMIT ?
	`, nowStr, host, count)
	if err != nil {
		return nil, err
	}
	defer util.Close(rows)

	result := []MonitoringItem{}

	for rows.Next() {
		var item MonitoringItem
		if err := rows.Scan(&item.URL, &item.Count, &item.Accept); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	return result, nil
}

// TopData TopData
func (s *Hotlink) TopData() ([]TopItem, error) {

	nowStr := time.Now().In(s.loc).Format("2006-01-02")

	rows, err := s.db.Query(`
		SELECT host, SUM(count) AS count
		FROM hotlink_referer
		WHERE last_date >= DATE_SUB(?, INTERVAL 1 DAY)
		GROUP BY host
		ORDER BY count DESC
		LIMIT ?
	`, nowStr, 100)
	if err != nil {
		return nil, err
	}
	defer util.Close(rows)

	result := []TopItem{}

	for rows.Next() {
		var item TopItem
		if err := rows.Scan(&item.Host, &item.Count); err != nil {
			return nil, err
		}

		item.Links, err = s.getMonitoringItems(item.Host, 20)
		if err != nil {
			return nil, err
		}

		item.Whitelisted, err = s.IsHostWhitelisted(item.Host)
		if err != nil {
			return nil, err
		}
		item.Blacklisted, err = s.IsHostBlacklisted(item.Host)
		if err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	return result, nil
}
