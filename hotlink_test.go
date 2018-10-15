package traffic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHotlinkAdd(t *testing.T) {

	config := LoadConfig()

	s, err := NewService(config)
	assert.NoError(t, err)

	err = s.Hotlink.Add("http://example.com/", "image/jpeg", time.Now())
	assert.NoError(t, err)

	s.Close()
}
