package traffic

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/autowp/traffic/util"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func createTrafficService(t *testing.T) *Traffic {
	config := LoadConfig()

	wg := &sync.WaitGroup{}

	pool, err := pgxpool.Connect(context.Background(), config.DSN)
	require.NoError(t, err)

	rabbitMQ, err := amqp.Dial(config.RabbitMQ)
	require.NoError(t, err)

	logger := util.NewLogger(config.Sentry)

	s, err := NewTraffic(wg, pool, rabbitMQ, logger, config.MonitoringQueue)
	require.NoError(t, err)

	return s
}

func TestMonitoringAdd(t *testing.T) {

	s := createTrafficService(t)
	defer util.Close(s)

	err := s.Monitoring.Add(net.IPv4(192, 168, 0, 1), time.Now())
	require.NoError(t, err)

	err = s.Monitoring.Add(net.IPv6loopback, time.Now())
	require.NoError(t, err)
}

func TestMonitoringGC(t *testing.T) {

	s := createTrafficService(t)
	defer util.Close(s)

	err := s.Monitoring.Clear()
	require.NoError(t, err)

	err = s.Monitoring.Add(net.IPv4(192, 168, 0, 1), time.Now())
	require.NoError(t, err)

	affected, err := s.Monitoring.GC()
	require.NoError(t, err)
	require.Zero(t, affected)

	items, err := s.Monitoring.ListOfTop(10)
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestAutoWhitelist(t *testing.T) {

	s := createTrafficService(t)
	defer util.Close(s)

	ip := net.IPv4(66, 249, 73, 139) // google

	err := s.Ban.Add(ip, time.Hour, 9, "test")
	require.NoError(t, err)

	exists, err := s.Ban.Exists(ip)
	require.NoError(t, err)
	require.True(t, exists)

	err = s.Monitoring.Add(ip, time.Now())
	require.NoError(t, err)

	exists, err = s.Monitoring.ExistsIP(ip)
	require.NoError(t, err)
	require.True(t, exists)

	err = s.AutoWhitelist()
	require.NoError(t, err)

	exists, err = s.Ban.Exists(ip)
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = s.Monitoring.ExistsIP(ip)
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = s.Whitelist.Exists(ip)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestAutoBanByProfile(t *testing.T) {

	s := createTrafficService(t)
	defer util.Close(s)

	profile := AutobanProfile{
		Limit:  3,
		Reason: "Test",
		Group:  []string{"hour", "tenminute", "minute"},
		Time:   time.Hour,
	}

	ip1 := net.IPv4(127, 0, 0, 1)
	ip2 := net.IPv4(127, 0, 0, 2)

	err := s.Monitoring.ClearIP(ip1)
	require.NoError(t, err)
	err = s.Monitoring.ClearIP(ip2)
	require.NoError(t, err)

	err = s.Ban.Remove(ip1)
	require.NoError(t, err)
	err = s.Ban.Remove(ip2)
	require.NoError(t, err)

	err = s.Monitoring.Add(ip1, time.Now())
	require.NoError(t, err)
	for i := 0; i < 4; i++ {
		err = s.Monitoring.Add(ip2, time.Now())
		require.NoError(t, err)
	}

	err = s.AutoBanByProfile(profile)
	require.NoError(t, err)

	exists, err := s.Ban.Exists(ip1)
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = s.Ban.Exists(ip2)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestWhitelistedNotBanned(t *testing.T) {

	s := createTrafficService(t)
	defer util.Close(s)

	profile := AutobanProfile{
		Limit:  3,
		Reason: "TestWhitelistedNotBanned",
		Group:  []string{"hour", "tenminute", "minute"},
		Time:   time.Hour,
	}

	ip := net.IPv4(178, 154, 244, 21)

	err := s.Whitelist.Add(ip, "TestWhitelistedNotBanned")
	require.NoError(t, err)

	for i := 0; i < 4; i++ {
		err = s.Monitoring.Add(ip, time.Now())
		require.NoError(t, err)
	}

	err = s.AutoWhitelistIP(ip)
	require.NoError(t, err)

	err = s.AutoBanByProfile(profile)
	require.NoError(t, err)

	exists, err := s.Ban.Exists(ip)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestHttpBanPost(t *testing.T) {
	s := createTrafficService(t)
	defer util.Close(s)

	err := s.Ban.Remove(net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)

	r := gin.New()
	s.SetupRouter(r)

	w := httptest.NewRecorder()
	b, err := json.Marshal(map[string]interface{}{
		"ip":         "127.0.0.1",
		"duration":   60 * 1000 * 1000 * 1000,
		"by_user_id": 4,
		"reason":     "Test",
	})
	require.NoError(t, err)
	req, err := http.NewRequest("POST", "/ban", bytes.NewBuffer(b))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	exists, err := s.Ban.Exists(net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.True(t, exists)

	w = httptest.NewRecorder()
	req, err = http.NewRequest("DELETE", "/ban/127.0.0.1", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)

	exists, err = s.Ban.Exists(net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.False(t, exists)
}

func TestTop(t *testing.T) {
	s := createTrafficService(t)
	defer util.Close(s)

	r := gin.New()
	s.SetupRouter(r)

	err := s.Ban.Clear()
	require.NoError(t, err)

	err = s.Monitoring.Clear()
	require.NoError(t, err)

	err = s.Monitoring.Add(net.IPv4(192, 168, 0, 1), time.Now())
	require.NoError(t, err)

	now := time.Now()
	for i := 0; i < 10; i++ {
		err = s.Monitoring.Add(net.IPv6loopback, now)
		require.NoError(t, err)
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/top", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	body, err := ioutil.ReadAll(w.Body)
	require.Equal(t, `[{"ip":"::1","count":10,"ban":null,"in_whitelist":false},{"ip":"192.168.0.1","count":1,"ban":null,"in_whitelist":false}]`, string(body))
}
