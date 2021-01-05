package traffic

import (
	"context"
	"github.com/autowp/traffic/util"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"net"
	"sync"
	"testing"
	"time"
)

func createBanService(t *testing.T) *Ban {
	config := LoadConfig()

	wg := &sync.WaitGroup{}

	pool, err := pgxpool.Connect(context.Background(), config.DSN)
	require.NoError(t, err)

	logger := util.NewLogger(config.Sentry)

	s, err := NewBan(wg, pool, logger)
	require.NoError(t, err)

	return s
}

func TestAddRemove(t *testing.T) {

	s := createBanService(t)
	defer util.Close(s)

	ip := net.IPv4(66, 249, 73, 139)

	err := s.Add(ip, time.Hour, 1, "Test")
	require.NoError(t, err)

	exists, err := s.Exists(ip)
	require.NoError(t, err)
	require.True(t, exists)

	err = s.Remove(ip)
	require.NoError(t, err)

	exists, err = s.Exists(ip)
	require.NoError(t, err)
	require.False(t, exists)
}
