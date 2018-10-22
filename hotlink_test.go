package traffic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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

func TestHotlinkWhitelist(t *testing.T) {
	config := LoadConfig()

	s, err := NewService(config)
	assert.NoError(t, err)
	defer s.Close()

	router := s.GetRouter()

	w := httptest.NewRecorder()
	b, err := json.Marshal(map[string]interface{}{
		"host": "yandex.com",
	})
	assert.NoError(t, err)
	req, err := http.NewRequest("POST", "/hotlink/whitelist", bytes.NewBuffer(b))
	assert.NoError(t, err)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	blacklisted, err := s.Hotlink.IsHostBlacklisted("yandex.com")
	assert.NoError(t, err)
	assert.False(t, blacklisted)

	whitelisted, err := s.Hotlink.IsHostWhitelisted("yandex.com")
	assert.NoError(t, err)
	assert.True(t, whitelisted)
}

func TestHotlinkBlacklist(t *testing.T) {
	config := LoadConfig()

	s, err := NewService(config)
	assert.NoError(t, err)
	defer s.Close()

	router := s.GetRouter()

	w := httptest.NewRecorder()
	b, err := json.Marshal(map[string]interface{}{
		"host": "yandex.com",
	})
	assert.NoError(t, err)
	req, err := http.NewRequest("POST", "/hotlink/blacklist", bytes.NewBuffer(b))
	assert.NoError(t, err)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	body, err := ioutil.ReadAll(w.Body)

	fmt.Println(string(body))

	blacklisted, err := s.Hotlink.IsHostBlacklisted("yandex.com")
	assert.NoError(t, err)
	assert.True(t, blacklisted)

	whitelisted, err := s.Hotlink.IsHostWhitelisted("yandex.com")
	assert.NoError(t, err)
	assert.False(t, whitelisted)
}

func TestHotlinkTop(t *testing.T) {
	config := LoadConfig()

	s, err := NewService(config)
	assert.NoError(t, err)
	defer s.Close()

	router := s.GetRouter()

	err = s.Hotlink.Clear()
	assert.NoError(t, err)

	err = s.Hotlink.Add("http://example.com/path-to-file", "image/jpeg", time.Now())
	assert.NoError(t, err)

	for i := 0; i < 10; i++ {
		err = s.Hotlink.Add("http://second.com/path-to-file", "image/jpeg", time.Now())
		assert.NoError(t, err)
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/hotlink/monitoring", nil)
	assert.NoError(t, err)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	body, err := ioutil.ReadAll(w.Body)

	assert.Contains(t, string(body), `"host":"example.com","count":1`)
	assert.Contains(t, string(body), `"host":"second.com","count":10`)
}

func TestHotlinkClear(t *testing.T) {
	config := LoadConfig()

	s, err := NewService(config)
	assert.NoError(t, err)
	defer s.Close()

	router := s.GetRouter()

	err = s.Hotlink.Add("http://example.com/path-to-file", "image/jpeg", time.Now())
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", "/hotlink/monitoring", nil)
	assert.NoError(t, err)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	items, err := s.Hotlink.TopData()
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestHotlinkClearHost(t *testing.T) {
	config := LoadConfig()

	s, err := NewService(config)
	assert.NoError(t, err)
	defer s.Close()

	router := s.GetRouter()

	s.Hotlink.Clear()

	err = s.Hotlink.Add("http://example.com/path-to-file", "image/jpeg", time.Now())
	assert.NoError(t, err)

	err = s.Hotlink.Add("http://second.com/path-to-file", "image/jpeg", time.Now())
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", "/hotlink/monitoring?host=example.com", nil)
	assert.NoError(t, err)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	items, err := s.Hotlink.TopData()
	assert.NoError(t, err)
	assert.Len(t, items, 1)
}
