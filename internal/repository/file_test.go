package repository

import (
	"context"
	"os"
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

func testUserURL(code, original, userID string) model.URL {
	u := testURL(code, original)
	u.UserID = userID
	return u
}

func TestFileRepository_Delete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")

	repo, err := NewFile(path)
	require.NoError(t, err)

	require.NoError(t, repo.Write(context.Background(), testURL("abc123", "https://example.com")))
	require.NoError(t, repo.Write(context.Background(), testURL("xyz789", "https://other.com")))

	require.NoError(t, repo.Delete(context.Background(), testURL("abc123", "https://example.com")))

	_, err = repo.Get(context.Background(), "abc123")
	assert.ErrorIs(t, err, ErrNotFound)

	got, err := repo.Get(context.Background(), "xyz789")
	require.NoError(t, err)
	assert.Equal(t, "https://other.com", got.Original)
}

func TestFileRepository_DeletePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")

	repo, err := NewFile(path)
	require.NoError(t, err)
	require.NoError(t, repo.Write(context.Background(), testURL("abc123", "https://example.com")))
	require.NoError(t, repo.Delete(context.Background(), testURL("abc123", "https://example.com")))

	reopened, err := NewFile(path)
	require.NoError(t, err)

	_, err = reopened.Get(context.Background(), "abc123")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileRepository_DeleteKeepsOtherOrigins(t *testing.T) {
	repo, err := NewFile(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, err)

	require.NoError(t, repo.Write(context.Background(), testURL("first", "https://same.example")))
	require.NoError(t, repo.Write(context.Background(), testURL("second", "https://diff.example")))

	require.NoError(t, repo.Delete(context.Background(), testURL("first", "https://same.example")))

	got, err := repo.Get(context.Background(), "second")
	require.NoError(t, err)
	assert.Equal(t, "https://diff.example", got.Original)

	_, err = repo.Get(context.Background(), "first")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileRepository_GetByUser(t *testing.T) {
	repo, err := NewFile(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, err)

	require.NoError(t, repo.Write(context.Background(), testUserURL("u1", "https://one.example", "alice")))
	require.NoError(t, repo.Write(context.Background(), testUserURL("u2", "https://two.example", "alice")))
	require.NoError(t, repo.Write(context.Background(), testUserURL("b1", "https://bob.example", "bob")))

	urls, err := repo.GetByUser(context.Background(), "alice")
	require.NoError(t, err)
	require.Len(t, urls, 2)
	for _, url := range urls {
		assert.Equal(t, "alice", url.UserID)
	}

	empty, err := repo.GetByUser(context.Background(), "nobody")
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestFileRepository_DeleteBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")

	repo, err := NewFile(path)
	require.NoError(t, err)

	require.NoError(t, repo.Write(context.Background(), testUserURL("d1", "https://d1.example", "alice")))
	require.NoError(t, repo.Write(context.Background(), testUserURL("keep", "https://keep.example", "alice")))
	require.NoError(t, repo.Write(context.Background(), testUserURL("bob", "https://bob.example", "bob")))

	require.NoError(t, repo.DeleteBatch(context.Background(), "alice", []string{"d1", "bob"}))

	reopened, err := NewFile(path)
	require.NoError(t, err)

	got, err := reopened.Get(context.Background(), "d1")
	require.NoError(t, err)
	assert.True(t, got.Deleted, "d1 must be tombstoned after reload")

	gotKeep, err := reopened.Get(context.Background(), "keep")
	require.NoError(t, err)
	assert.False(t, gotKeep.Deleted)

	gotBob, err := reopened.Get(context.Background(), "bob")
	require.NoError(t, err)
	assert.False(t, gotBob.Deleted, "bob belongs to another user and must survive")
}

func TestFileRepository_DeleteBatchNothing(t *testing.T) {
	repo, err := NewFile(filepath.Join(t.TempDir(), "data.json"))
	require.NoError(t, err)

	require.NoError(t, repo.Write(context.Background(), testUserURL("d1", "https://d1.example", "alice")))

	require.NoError(t, repo.DeleteBatch(context.Background(), "nobody", []string{"d1"}))

	got, err := repo.Get(context.Background(), "d1")
	require.NoError(t, err)
	assert.False(t, got.Deleted)
}

func TestFileRepository_LegacyArrayFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	legacy := `[
  {"uuid":"legacy-1","original_url":"https://legacy.example","short_url":"leg1","user_id":"","is_deleted":false},
  {"uuid":"legacy-2","original_url":"https://legacy2.example","short_url":"leg2","user_id":"bob","is_deleted":false}
]`
	require.NoError(t, os.WriteFile(path, []byte(legacy), 0o644))

	repo, err := NewFile(path)
	require.NoError(t, err)

	got1, err := repo.Get(context.Background(), "leg1")
	require.NoError(t, err)
	assert.Equal(t, "https://legacy.example", got1.Original)

	got2, err := repo.Get(context.Background(), "leg2")
	require.NoError(t, err)
	assert.Equal(t, "bob", got2.UserID)
}
