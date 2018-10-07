package traffic

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestService(t *testing.T) {

	config := LoadConfig()

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
