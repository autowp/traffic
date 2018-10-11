package traffic

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchAuto(t *testing.T) {

	config := LoadConfig()

	s, err := NewService(config)

	assert.NoError(t, err)

	// match, _ := s.Whitelist.MatchAuto(net.IPv4(178, 154, 255, 146)) // yandex
	// assert.True(t, match)

	match, _ := s.Whitelist.MatchAuto(net.IPv4(66, 249, 73, 139)) // google
	assert.True(t, match)

	match, _ = s.Whitelist.MatchAuto(net.IPv4(157, 55, 39, 127)) // msn
	assert.True(t, match)

	match, _ = s.Whitelist.MatchAuto(net.IPv4(127, 0, 0, 1)) // loopback
	assert.False(t, match)

	s.Close()
}

func TestContains(t *testing.T) {

	config := LoadConfig()

	s, err := NewService(config)

	assert.NoError(t, err)

	ip := net.IPv4(66, 249, 73, 139)

	err = s.Whitelist.Add(ip, "test")
	assert.NoError(t, err)

	exists, err := s.Whitelist.Exists(ip)
	assert.NoError(t, err)
	assert.True(t, exists)

	s.Close()
}
