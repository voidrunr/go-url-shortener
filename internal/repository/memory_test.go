package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

func TestMemoryRepository(t *testing.T) {
	repo := NewMemory()

	url := testURL("abc123", "https://example.com")

	t.Run("write and get", func(t *testing.T) {
		require.NoError(t, repo.Write(url))

		got, err := repo.Get("abc123")
		require.NoError(t, err)
		assert.Equal(t, url.Original, got.Original)
		assert.Equal(t, url.Code, got.Code)
	})

	t.Run("duplicate code returns ErrConflict", func(t *testing.T) {
		err := repo.Write(testURL("abc123", "https://other.com"))
		assert.ErrorIs(t, err, ErrConflict)
	})

	t.Run("duplicate original returns DuplicateURLError", func(t *testing.T) {
		repo := NewMemory()
		require.NoError(t, repo.Write(testURL("abc123", "https://example.com")))

		err := repo.Write(testURL("xyz789", "https://example.com"))
		var dupErr *DuplicateURLError
		require.ErrorAs(t, err, &dupErr)
		assert.ErrorIs(t, err, ErrURLAlreadyExists)
		assert.Equal(t, "abc123", dupErr.URL.Code)
	})

	t.Run("batch write", func(t *testing.T) {
		repo := NewMemory()
		u1 := testURL("b1", "https://one.example")
		u2 := testURL("b2", "https://two.example")

		require.NoError(t, repo.WriteBatch([]model.URL{u1, u2}))

		got1, err := repo.Get("b1")
		require.NoError(t, err)
		assert.Equal(t, u1.Original, got1.Original)

		got2, err := repo.Get("b2")
		require.NoError(t, err)
		assert.Equal(t, u2.Original, got2.Original)
	})

	t.Run("batch write conflict writes nothing", func(t *testing.T) {
		repo := NewMemory()
		require.NoError(t, repo.Write(testURL("b1", "https://one.example")))

		err := repo.WriteBatch([]model.URL{
			testURL("b2", "https://two.example"),
			testURL("b1", "https://conflict.example"),
		})
		assert.ErrorIs(t, err, ErrConflict)

		_, err = repo.Get("b2")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("unknown code returns ErrNotFound", func(t *testing.T) {
		_, err := repo.Get("missing")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("get by user", func(t *testing.T) {
		repo := NewMemory()
		u1 := testURL("d1", "https://one.example")
		u1.UserID = "user-a"
		u2 := testURL("d2", "https://two.example")
		u2.UserID = "user-a"
		u3 := testURL("d3", "https://three.example")
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

		none, err := repo.GetByUser("nobody")
		require.NoError(t, err)
		assert.Empty(t, none)
	})

	t.Run("repeated write is preserved", func(t *testing.T) {
		repo := NewMemory()
		u1 := testURL("c1", "https://one.example")
		u2 := testURL("c2", "https://two.example")

		require.NoError(t, repo.Write(u1))
		require.NoError(t, repo.Write(u2))

		got1, err := repo.Get("c1")
		require.NoError(t, err)
		assert.Equal(t, u1.Original, got1.Original)

		got2, err := repo.Get("c2")
		require.NoError(t, err)
		assert.Equal(t, u2.Original, got2.Original)
	})
}

func TestMemoryRepository_Empty(t *testing.T) {
	repo := NewMemory()

	_, err := repo.Get("anything")
	assert.ErrorIs(t, err, ErrNotFound)
}
