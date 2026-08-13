package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunr/go-url-shortener/internal/database"
	"github.com/voidrunr/go-url-shortener/internal/database/migrations"
	"github.com/voidrunr/go-url-shortener/internal/model"
)

func deleteByOriginals(t *testing.T, db *sql.DB, originals ...string) {
	t.Helper()

	if len(originals) == 0 {
		return
	}

	placeholders := make([]string, 0, len(originals))
	args := make([]any, 0, len(originals))
	for i, original := range originals {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, original)
	}

	query := fmt.Sprintf("DELETE FROM shortener_urls WHERE original_url IN (%s)", strings.Join(placeholders, ", "))
	_, err := db.Exec(query, args...)
	require.NoError(t, err)
}

func TestPostgresRepository(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping postgres integration test")
	}

	db, err := database.Open(dsn)
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, migrations.Up(db))

	repo := NewPostgres(db)

	deleteByOriginals(t, db, "https://example.com", "https://other.com")

	code := fmt.Sprintf("pg%d", time.Now().UnixNano())
	url := testURL(code, "https://example.com")

	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = $1", code)
	})

	t.Run("write and get", func(t *testing.T) {
		require.NoError(t, repo.Write(context.Background(), url))

		got, err := repo.Get(context.Background(), code)
		require.NoError(t, err)
		assert.Equal(t, url.Original, got.Original)
		assert.Equal(t, url.Code, got.Code)
	})

	t.Run("duplicate code returns ErrConflict", func(t *testing.T) {
		err := repo.Write(context.Background(), testURL(code, "https://other.com"))
		assert.ErrorIs(t, err, ErrConflict)
	})

	t.Run("duplicate original returns DuplicateURLError", func(t *testing.T) {
		code2 := fmt.Sprintf("pgd%d", time.Now().UnixNano())
		t.Cleanup(func() {
			_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = $1", code2)
		})

		err := repo.Write(context.Background(), testURL(code2, url.Original))
		var dupErr *DuplicateURLError
		require.ErrorAs(t, err, &dupErr)
		assert.ErrorIs(t, err, ErrURLAlreadyExists)
		assert.Equal(t, code, dupErr.URL.Code)
		assert.Equal(t, url.Original, dupErr.URL.Original)
	})

	t.Run("unknown code returns ErrNotFound", func(t *testing.T) {
		_, err := repo.Get(context.Background(), "missing")
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

	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, migrations.Up(db))

	repo := NewPostgres(db)

	deleteByOriginals(t, db,
		"https://one.example",
		"https://two.example",
		"https://three.example",
		"https://conflict.example",
	)

	code1 := fmt.Sprintf("pgb%d", time.Now().UnixNano())
	code2 := fmt.Sprintf("pgb%d", time.Now().UnixNano()+1)

	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = ANY($1)", []string{code1, code2})
	})

	u1 := testURL(code1, "https://one.example")
	u2 := testURL(code2, "https://two.example")

	t.Run("write batch and get", func(t *testing.T) {
		require.NoError(t, repo.WriteBatch(context.Background(), []model.URL{u1, u2}))

		got1, err := repo.Get(context.Background(), code1)
		require.NoError(t, err)
		assert.Equal(t, u1.Original, got1.Original)

		got2, err := repo.Get(context.Background(), code2)
		require.NoError(t, err)
		assert.Equal(t, u2.Original, got2.Original)
	})

	t.Run("batch conflict writes nothing", func(t *testing.T) {
		code3 := fmt.Sprintf("pgb%d", time.Now().UnixNano()+2)
		t.Cleanup(func() {
			_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = $1", code3)
		})

		err := repo.WriteBatch(context.Background(), []model.URL{
			testURL(code3, "https://three.example"),
			testURL(code1, "https://conflict.example"),
		})
		assert.ErrorIs(t, err, ErrConflict)

		_, err = repo.Get(context.Background(), code3)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("batch duplicate original returns DuplicateURLError", func(t *testing.T) {
		dupCode := fmt.Sprintf("pgb%d", time.Now().UnixNano()+5)
		t.Cleanup(func() {
			_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = $1", dupCode)
		})

		err := repo.Write(context.Background(), testURL(dupCode, "https://dup.example"))
		require.NoError(t, err)

		newCode := fmt.Sprintf("pgb%d", time.Now().UnixNano()+6)
		err = repo.WriteBatch(context.Background(), []model.URL{
			testURL(newCode, "https://dup.example"),
		})
		var dupErr *DuplicateURLError
		require.ErrorAs(t, err, &dupErr)
		assert.ErrorIs(t, err, ErrURLAlreadyExists)
		assert.Equal(t, dupCode, dupErr.URL.Code)

		_, err = repo.Get(context.Background(), newCode)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
