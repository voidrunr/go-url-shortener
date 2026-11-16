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

func TestDuplicateURLError_ErrorAndUnwrap(t *testing.T) {
	err := &DuplicateURLError{URL: testURL("abc123", "https://example.com")}

	assert.Equal(t, ErrURLAlreadyExists.Error(), err.Error())
	assert.ErrorIs(t, err, ErrURLAlreadyExists)
}

func TestFileRepository_NewFileReadError(t *testing.T) {
	_, err := NewFile(t.TempDir())
	require.Error(t, err)
	assert.NotErrorIs(t, err, os.ErrNotExist)
}

func TestFileRepository_EmptyFileIsNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	require.NoError(t, os.WriteFile(path, []byte("  \n"), 0o644))

	repo, err := NewFile(path)
	require.NoError(t, err)

	_, err = repo.Get(context.Background(), "anything")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileRepository_InvalidJSONLineReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	content := `{"short_url":"ok","original_url":"https://ok.example"}
not-json`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	_, err := NewFile(path)
	require.Error(t, err)
}

func TestFileRepository_TombstoneOnlyEntryStaysDeleted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	line := `{"uuid":"t1","original_url":"https://gone.example","short_url":"gonex","user_id":"alice","is_deleted":true}`
	require.NoError(t, os.WriteFile(path, []byte(line+"\n"), 0o644))

	repo, err := NewFile(path)
	require.NoError(t, err)

	got, err := repo.Get(context.Background(), "gonex")
	require.NoError(t, err)
	assert.True(t, got.Deleted)

	urls, err := repo.GetByUser(context.Background(), "alice")
	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.NotContains(t, repo.byOriginal, "https://gone.example", "deleted entries must not be registered in byOriginal")
}

func TestFileRepository_RepeatedEntryOverwritesOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	content := `{"uuid":"1","original_url":"https://first.example","short_url":"code1","user_id":"alice"}
{"uuid":"2","original_url":"https://second.example","short_url":"code1","user_id":"alice"}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	repo, err := NewFile(path)
	require.NoError(t, err)

	got, err := repo.Get(context.Background(), "code1")
	require.NoError(t, err)
	assert.Equal(t, "https://second.example", got.Original)
	assert.Equal(t, "code1", repo.byOriginal["https://second.example"])
}

func TestFileRepository_WriteAppendOpenError(t *testing.T) {
	repo := &FileRepository{
		store:      make(map[string]model.URL),
		byOriginal: make(map[string]string),
		filePath:   t.TempDir(),
	}

	err := repo.Write(context.Background(), testURL("abc123", "https://example.com"))
	require.Error(t, err)
}
