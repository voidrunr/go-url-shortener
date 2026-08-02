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
	"github.com/voidrunr/go-url-shortener/internal/model"
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

func TestPostgresRepository_WriteBatch(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping postgres integration test")
	}

	db, err := database.Open(dsn)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, migrations.Up(db))

	repo := NewPostgres(db)

	code1 := fmt.Sprintf("pgb%d", time.Now().UnixNano())
	code2 := fmt.Sprintf("pgb%d", time.Now().UnixNano()+1)

	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = ANY($1)", []string{code1, code2})
	})

	u1 := testURL(code1, "https://one.example")
	u2 := testURL(code2, "https://two.example")

	t.Run("write batch and get", func(t *testing.T) {
		require.NoError(t, repo.WriteBatch([]model.URL{u1, u2}))

		got1, err := repo.Get(code1)
		require.NoError(t, err)
		assert.Equal(t, u1.Original, got1.Original)

		got2, err := repo.Get(code2)
		require.NoError(t, err)
		assert.Equal(t, u2.Original, got2.Original)
	})

	t.Run("batch conflict writes nothing", func(t *testing.T) {
		code3 := fmt.Sprintf("pgb%d", time.Now().UnixNano()+2)
		t.Cleanup(func() {
			_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = $1", code3)
		})

		err := repo.WriteBatch([]model.URL{
			testURL(code3, "https://three.example"),
			testURL(code1, "https://conflict.example"),
		})
		assert.ErrorIs(t, err, ErrConflict)

		_, err = repo.Get(code3)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
