package repository

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunr/go-url-shortener/internal/database"
	"github.com/voidrunr/go-url-shortener/internal/database/migrations"
)

func TestPostgresRepository(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping postgres integration test")
	}

	db, err := database.Open(dsn)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, migrations.Up(db))

	repo := NewPostgres(db)

	code := fmt.Sprintf("pg%d", time.Now().UnixNano())
	url := testURL(code, "https://example.com")

	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = $1", code)
	})

	t.Run("write and get", func(t *testing.T) {
		require.NoError(t, repo.Write(url))

		got, err := repo.Get(code)
		require.NoError(t, err)
		assert.Equal(t, url.Original, got.Original)
		assert.Equal(t, url.Code, got.Code)
	})

	t.Run("duplicate code returns ErrConflict", func(t *testing.T) {
		err := repo.Write(testURL(code, "https://other.com"))
		assert.ErrorIs(t, err, ErrConflict)
	})

	t.Run("unknown code returns ErrNotFound", func(t *testing.T) {
		_, err := repo.Get("missing")
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
