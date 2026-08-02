package repository

import (
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

	t.Run("duplicate original returns DuplicateURLError", func(t *testing.T) {
		code2 := fmt.Sprintf("pgd%d", time.Now().UnixNano())
		t.Cleanup(func() {
			_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = $1", code2)
		})

		err := repo.Write(testURL(code2, url.Original))
		var dupErr *DuplicateURLError
		require.ErrorAs(t, err, &dupErr)
		assert.ErrorIs(t, err, ErrURLAlreadyExists)
		assert.Equal(t, code, dupErr.URL.Code)
		assert.Equal(t, url.Original, dupErr.URL.Original)
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

func TestPostgresRepository_GetByUser(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping postgres integration test")
	}

	db, err := database.Open(dsn)
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, migrations.Up(db))

	repo := NewPostgres(db)

	base := time.Now().UnixNano()
	codes := []string{
		fmt.Sprintf("pgu%d", base),
		fmt.Sprintf("pgu%d", base+1),
		fmt.Sprintf("pgu%d", base+2),
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = ANY($1)", codes)
	})

	u1 := testURL(codes[0], "https://one.user.example")
	u1.UserID = "user-a"
	u2 := testURL(codes[1], "https://two.user.example")
	u2.UserID = "user-a"
	u3 := testURL(codes[2], "https://three.user.example")
	u3.UserID = "user-b"

	require.NoError(t, repo.Write(u1))
	require.NoError(t, repo.Write(u2))
	require.NoError(t, repo.Write(u3))

	got, err := repo.GetByUser("user-a")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	for _, u := range got {
		assert.Equal(t, "user-a", u.UserID)
	}

	other, err := repo.GetByUser("user-b")
	require.NoError(t, err)
	assert.Len(t, other, 1)
	assert.Equal(t, "user-b", other[0].UserID)

	none, err := repo.GetByUser("nobody")
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestPostgresRepository_DeleteBatch(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping postgres integration test")
	}

	db, err := database.Open(dsn)
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, migrations.Up(db))

	repo := NewPostgres(db)

	base := time.Now().UnixNano()
	codes := []string{
		fmt.Sprintf("pgdel%d", base),
		fmt.Sprintf("pgdel%d", base+1),
		fmt.Sprintf("pgdel%d", base+2),
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM shortener_urls WHERE code = ANY($1)", codes)
	})

	u1 := testURL(codes[0], "https://one.del.example")
	u1.UserID = "user-a"
	u2 := testURL(codes[1], "https://two.del.example")
	u2.UserID = "user-a"
	u3 := testURL(codes[2], "https://three.del.example")
	u3.UserID = "user-b"

	require.NoError(t, repo.Write(u1))
	require.NoError(t, repo.Write(u2))
	require.NoError(t, repo.Write(u3))

	require.NoError(t, repo.DeleteBatch("user-a", []string{codes[0], codes[1], "missing"}))

	got1, err := repo.Get(codes[0])
	require.NoError(t, err)
	assert.True(t, got1.Deleted)

	got2, err := repo.Get(codes[1])
	require.NoError(t, err)
	assert.True(t, got2.Deleted)

	got3, err := repo.Get(codes[2])
	require.NoError(t, err)
	assert.False(t, got3.Deleted)

	require.NoError(t, repo.DeleteBatch("nobody", []string{codes[0]}))
	got1, err = repo.Get(codes[0])
	require.NoError(t, err)
	assert.True(t, got1.Deleted)
}
