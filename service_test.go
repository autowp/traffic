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

	s.Monitoring.Add(net.IPv4(192, 168, 0, 1), time.Now())

	s.Monitoring.Add(net.IPv6loopback, time.Now())

	s.Close()
}
