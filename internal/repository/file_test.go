package repository

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileRepository_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")

	repo, err := NewFile(path)
	require.NoError(t, err)

	url := testURL("abc123", "https://example.com")
	require.NoError(t, repo.Write(url))

	reopened, err := NewFile(path)
	require.NoError(t, err)

	got, err := reopened.Get("abc123")
	require.NoError(t, err)
	assert.Equal(t, url.Original, got.Original)
	assert.Equal(t, url.Code, got.Code)
}

func TestFileRepository_NotFound(t *testing.T) {
	repo, err := NewFile(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, err)

	_, err = repo.Get("missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileRepository_Conflict(t *testing.T) {
	repo, err := NewFile(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, err)

	require.NoError(t, repo.Write(testURL("abc123", "https://example.com")))

	err = repo.Write(testURL("abc123", "https://other.com"))
	assert.ErrorIs(t, err, ErrConflict)
}

func TestFileRepository_MissingFileIsNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope", "data.json")

	_, err := NewFile(path)
	assert.NoError(t, err)
}
