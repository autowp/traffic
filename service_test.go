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

func TestAutoWhitelist(t *testing.T) {

	config := LoadConfig()

	s, err := NewService(config)
	assert.Equal(t, nil, err)

	ip := net.IPv4(66, 249, 73, 139) // google

	err = s.Ban.Add(ip, time.Hour, 9, "test")
	assert.Equal(t, nil, err)

	exists, err := s.Ban.Exists(ip)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, exists)

	err = s.Monitoring.Add(ip, time.Now())
	assert.Equal(t, nil, err)

	exists, err = s.Monitoring.ExistsIP(ip)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, exists)

	err = s.autoWhitelist()
	assert.Equal(t, nil, err)

	exists, err = s.Ban.Exists(ip)
	assert.Equal(t, nil, err)
	assert.Equal(t, false, exists)

	exists, err = s.Monitoring.ExistsIP(ip)
	assert.Equal(t, nil, err)
	assert.Equal(t, false, exists)

	exists, err = s.Whitelist.Exists(ip)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, exists)

	s.Close()
}
