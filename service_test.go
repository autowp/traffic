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

	assert.NoError(t, err)

	s.Monitoring.Add(net.IPv4(192, 168, 0, 1), time.Now())

	s.Monitoring.Add(net.IPv6loopback, time.Now())

	s.Close()
}

func TestAutoWhitelist(t *testing.T) {

	config := LoadConfig()

	s, err := NewService(config)
	assert.NoError(t, err)

	ip := net.IPv4(66, 249, 73, 139) // google

	err = s.Ban.Add(ip, time.Hour, 9, "test")
	assert.NoError(t, err)

	exists, err := s.Ban.Exists(ip)
	assert.NoError(t, err)
	assert.True(t, exists)

	err = s.Monitoring.Add(ip, time.Now())
	assert.NoError(t, err)

	exists, err = s.Monitoring.ExistsIP(ip)
	assert.NoError(t, err)
	assert.True(t, exists)

	err = s.autoWhitelist()
	assert.NoError(t, err)

	exists, err = s.Ban.Exists(ip)
	assert.NoError(t, err)
	assert.False(t, exists)

	exists, err = s.Monitoring.ExistsIP(ip)
	assert.NoError(t, err)
	assert.False(t, exists)

	exists, err = s.Whitelist.Exists(ip)
	assert.NoError(t, err)
	assert.True(t, exists)

	s.Close()
}

func TestAutoBanByProfile(t *testing.T) {

	config := LoadConfig()

	s, err := NewService(config)
	assert.NoError(t, err)

	profile := AutobanProfile{
		Limit:  3,
		Reason: "Test",
		Group:  []string{"hour", "tenminute", "minute"},
		Time:   time.Hour,
	}

	ip1 := net.IPv4(127, 0, 0, 1)
	ip2 := net.IPv4(127, 0, 0, 2)

	err = s.Monitoring.ClearIP(ip1)
	assert.NoError(t, err)
	err = s.Monitoring.ClearIP(ip2)
	assert.NoError(t, err)

	err = s.Ban.Remove(ip1)
	assert.NoError(t, err)
	err = s.Ban.Remove(ip2)
	assert.NoError(t, err)

	s.Monitoring.Add(ip1, time.Now())
	for i := 0; i < 4; i++ {
		err = s.Monitoring.Add(ip2, time.Now())
		assert.NoError(t, err)
	}

	err = s.autoBanByProfile(profile)
	assert.NoError(t, err)

	exists, err := s.Ban.Exists(ip1)
	assert.NoError(t, err)
	assert.False(t, exists)

	exists, err = s.Ban.Exists(ip2)
	assert.NoError(t, err)
	assert.True(t, exists)
}
