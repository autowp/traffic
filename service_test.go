package traffic

import (
	"net"
	"testing"
	"time"
)

func TestService(t *testing.T) {

	config := Config{
		Input: InputConfig{
			Address: "amqp://guest:guest@localhost:5672/",
			Queue:   "input",
		},
		Rollbar: RollbarConfig{
			Token:       "",
			Environment: "testing",
			Period:      "1m",
		},
		DSN: "root:password@tcp(localhost)/traffic?charset=utf8mb4&parseTime=true&loc=UTC",
	}

	s := NewService(config)

	s.pushHit(InputMessage{
		IP:        net.IPv4(192, 168, 0, 1),
		Timestamp: time.Now(),
	})

	s.pushHit(InputMessage{
		IP:        net.IPv6loopback,
		Timestamp: time.Now(),
	})

	s.Close()
}
