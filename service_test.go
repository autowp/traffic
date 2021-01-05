package traffic

import (
	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMigrations(t *testing.T) {

	config := LoadConfig()

	err := applyMigrations(config.Migrations)
	if err != migrate.ErrNoChange {
		require.NoError(t, err)
	}
}
