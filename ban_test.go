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

	assert.NoError(t, err)

	ip := net.IPv4(66, 249, 73, 139)

	err = s.Ban.Add(ip, time.Hour, 1, "Test")
	assert.NoError(t, err)

	exists, err := s.Ban.Exists(ip)
	assert.NoError(t, err)
	assert.True(t, exists)

	err = s.Ban.Remove(ip)
	assert.NoError(t, err)

	exists, err = s.Ban.Exists(ip)
	assert.NoError(t, err)
	assert.False(t, exists)

	s.Close()
}
