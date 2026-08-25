package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

func TestMemoryRepository(t *testing.T) {
	repo := NewMemory()

	url := testURL("abc123", "https://example.com")

	t.Run("write and get", func(t *testing.T) {
		require.NoError(t, repo.Write(context.Background(), url))

		got, err := repo.Get(context.Background(), "abc123")
		require.NoError(t, err)
		assert.Equal(t, url.Original, got.Original)
		assert.Equal(t, url.Code, got.Code)
	})

	t.Run("duplicate code returns ErrConflict", func(t *testing.T) {
		err := repo.Write(context.Background(), testURL("abc123", "https://other.com"))
		assert.ErrorIs(t, err, ErrConflict)
	})

	t.Run("duplicate original returns DuplicateURLError", func(t *testing.T) {
		repo := NewMemory()
		require.NoError(t, repo.Write(context.Background(), testURL("abc123", "https://example.com")))

		err := repo.Write(context.Background(), testURL("xyz789", "https://example.com"))
		var dupErr *DuplicateURLError
		require.ErrorAs(t, err, &dupErr)
		assert.ErrorIs(t, err, ErrURLAlreadyExists)
		assert.Equal(t, "abc123", dupErr.URL.Code)
	})

	t.Run("batch write", func(t *testing.T) {
		repo := NewMemory()
		u1 := testURL("b1", "https://one.example")
		u2 := testURL("b2", "https://two.example")

		require.NoError(t, repo.WriteBatch(context.Background(), []model.URL{u1, u2}))

		got1, err := repo.Get(context.Background(), "b1")
		require.NoError(t, err)
		assert.Equal(t, u1.Original, got1.Original)

		got2, err := repo.Get(context.Background(), "b2")
		require.NoError(t, err)
		assert.Equal(t, u2.Original, got2.Original)
	})

	t.Run("batch write conflict writes nothing", func(t *testing.T) {
		repo := NewMemory()
		require.NoError(t, repo.Write(context.Background(), testURL("b1", "https://one.example")))

		err := repo.WriteBatch(context.Background(), []model.URL{
			testURL("b2", "https://two.example"),
			testURL("b1", "https://conflict.example"),
		})
		assert.ErrorIs(t, err, ErrConflict)

		_, err = repo.Get(context.Background(), "b2")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("unknown code returns ErrNotFound", func(t *testing.T) {
		_, err := repo.Get(context.Background(), "missing")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("repeated write is preserved", func(t *testing.T) {
		repo := NewMemory()
		u1 := testURL("c1", "https://one.example")
		u2 := testURL("c2", "https://two.example")

		require.NoError(t, repo.Write(context.Background(), u1))
		require.NoError(t, repo.Write(context.Background(), u2))

		got1, err := repo.Get(context.Background(), "c1")
		require.NoError(t, err)
		assert.Equal(t, u1.Original, got1.Original)

		got2, err := repo.Get(context.Background(), "c2")
		require.NoError(t, err)
		assert.Equal(t, u2.Original, got2.Original)
	})
}

func TestMemoryRepository_Empty(t *testing.T) {
	repo := NewMemory()

	_, err := repo.Get(context.Background(), "anything")
	assert.ErrorIs(t, err, ErrNotFound)
}
