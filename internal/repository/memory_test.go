package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	t.Run("unknown code returns ErrNotFound", func(t *testing.T) {
		_, err := repo.Get("missing")
		assert.ErrorIs(t, err, ErrNotFound)
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
