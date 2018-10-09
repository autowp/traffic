package traffic

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchAuto(t *testing.T) {

	config := LoadConfig()

	s, err := NewService(config)

	assert.Equal(t, nil, err)

	// match, _ := s.Whitelist.MatchAuto(net.IPv4(178, 154, 255, 146)) // yandex
	// assert.Equal(t, true, match)

	match, _ := s.Whitelist.MatchAuto(net.IPv4(66, 249, 73, 139)) // google
	assert.Equal(t, true, match)

	match, _ = s.Whitelist.MatchAuto(net.IPv4(157, 55, 39, 127)) // msn
	assert.Equal(t, true, match)

	match, _ = s.Whitelist.MatchAuto(net.IPv4(127, 0, 0, 1)) // loopback
	assert.Equal(t, false, match)

	s.Close()
}

func TestContains(t *testing.T) {

	config := LoadConfig()

	s, err := NewService(config)

	assert.Equal(t, nil, err)

	ip := net.IPv4(66, 249, 73, 139)
	fmt.Printf("IP=%v\n", ip)

	err = s.Whitelist.Add(ip, "test")
	assert.Equal(t, nil, err)

	exists, err := s.Whitelist.Exists(ip)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, exists)

	s.Close()
}
