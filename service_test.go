package traffic

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

	s, err := NewService(config)

	assert.Equal(t, nil, err)

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
