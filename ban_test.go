package traffic

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAddRemove(t *testing.T) {

	config := LoadConfig()

	s, err := NewService(config)

	assert.Equal(t, nil, err)

	ip := net.IPv4(66, 249, 73, 139)

	err = s.Ban.Add(ip, time.Hour, 1, "Test")
	assert.Equal(t, nil, err)

	exists, err := s.Ban.Exists(ip)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, exists)

	err = s.Ban.Remove(ip)
	assert.Equal(t, nil, err)

	exists, err = s.Ban.Exists(ip)
	assert.Equal(t, nil, err)
	assert.Equal(t, false, exists)

	s.Close()
}
