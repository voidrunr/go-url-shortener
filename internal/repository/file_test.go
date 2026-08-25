package repository

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

func TestFileRepository_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")

	repo, err := NewFile(path)
	require.NoError(t, err)

	url := testURL("abc123", "https://example.com")
	require.NoError(t, repo.Write(context.Background(), url))

	reopened, err := NewFile(path)
	require.NoError(t, err)

	got, err := reopened.Get(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, url.Original, got.Original)
	assert.Equal(t, url.Code, got.Code)
}

func TestFileRepository_NotFound(t *testing.T) {
	repo, err := NewFile(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, err)

	_, err = repo.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileRepository_Conflict(t *testing.T) {
	repo, err := NewFile(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, err)

	require.NoError(t, repo.Write(context.Background(), testURL("abc123", "https://example.com")))

	err = repo.Write(context.Background(), testURL("abc123", "https://other.com"))
	assert.ErrorIs(t, err, ErrConflict)
}

func TestFileRepository_DuplicateOriginal(t *testing.T) {
	repo, err := NewFile(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, err)

	require.NoError(t, repo.Write(context.Background(), testURL("abc123", "https://example.com")))

	err = repo.Write(context.Background(), testURL("xyz789", "https://example.com"))
	var dupErr *DuplicateURLError
	require.ErrorAs(t, err, &dupErr)
	assert.ErrorIs(t, err, ErrURLAlreadyExists)
	assert.Equal(t, "abc123", dupErr.URL.Code)
}

func TestFileRepository_MissingFileIsNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope", "data.json")

	_, err := NewFile(path)
	assert.NoError(t, err)
}

func TestFileRepository_WriteBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")

	repo, err := NewFile(path)
	require.NoError(t, err)

	u1 := testURL("b1", "https://one.example")
	u2 := testURL("b2", "https://two.example")
	require.NoError(t, repo.WriteBatch(context.Background(), []model.URL{u1, u2}))

	reopened, err := NewFile(path)
	require.NoError(t, err)

	got1, err := reopened.Get(context.Background(), "b1")
	require.NoError(t, err)
	assert.Equal(t, u1.Original, got1.Original)

	got2, err := reopened.Get(context.Background(), "b2")
	require.NoError(t, err)
	assert.Equal(t, u2.Original, got2.Original)
}

func TestFileRepository_WriteBatchConflict(t *testing.T) {
	repo, err := NewFile(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, err)

	require.NoError(t, repo.Write(context.Background(), testURL("b1", "https://one.example")))

	err = repo.WriteBatch(context.Background(), []model.URL{
		testURL("b2", "https://two.example"),
		testURL("b1", "https://conflict.example"),
	})
	assert.ErrorIs(t, err, ErrConflict)

	_, err = repo.Get(context.Background(), "b2")
	assert.ErrorIs(t, err, ErrNotFound)
}
