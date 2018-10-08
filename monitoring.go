package traffic

import (
	"database/sql"
	"net"
	"time"
)

// Monitoring Main Object
type Monitoring struct {
	db *sql.DB
}

// NewMonitoring constructor
func NewMonitoring(db *sql.DB) (*Monitoring, error) {
	return &Monitoring{
		db: db,
	}, nil
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
	defer stmt.Close()

	dateStr := timestamp.Format("2006-01-02 15:04:05")
	_, err = stmt.Exec(dateStr, dateStr, dateStr, dateStr, ip.String())
	if err != nil {
		return err
	}

	return nil
}

// GC Garbage Collect
func (s *Monitoring) GC() (int64, error) {

	stmt, err := s.db.Prepare("DELETE FROM ip_monitoring4 WHERE day_date < CURDATE()")
	if err != nil {
		return 0, err
	}
	res, err := stmt.Exec()
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

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
	defer stmt.Close()
	_, err = stmt.Exec(ip.String())
	if err != nil {
		return err
	}

	return nil
}
