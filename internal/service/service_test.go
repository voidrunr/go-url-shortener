package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

type stubRepo struct {
	urls            map[string]model.URL
	writeCalls      int
	batchWriteCalls int
	conflicts       int
	conflictOnWrite int
	duplicate       bool
}

func newStubRepo() *stubRepo {
	return &stubRepo{urls: make(map[string]model.URL)}
}

func (r *stubRepo) Write(ctx context.Context, url model.URL) error {
	r.writeCalls++
	if r.conflictOnWrite > 0 {
		r.conflictOnWrite--
		return repository.ErrConflict
	}
	for _, existing := range r.urls {
		if existing.Original == url.Original {
			return &repository.DuplicateURLError{URL: existing}
		}
	}
	if _, ok := r.urls[url.Code]; ok {
		return repository.ErrConflict
	}
	r.urls[url.Code] = url
	return nil
}

func (r *stubRepo) WriteBatch(ctx context.Context, urls []model.URL) error {
	r.batchWriteCalls++
	if r.duplicate {
		return &repository.DuplicateURLError{URL: model.URL{Code: "existing", Original: urls[0].Original}}
	}
	if r.conflicts > 0 {
		r.conflicts--
		return repository.ErrConflict
	}
	for _, u := range urls {
		if err := r.Write(ctx, u); err != nil {
			return err
		}
	}
	return nil
}

func (r *stubRepo) Get(ctx context.Context, code string) (model.URL, error) {
	u, ok := r.urls[code]
	if !ok {
		return model.URL{}, repository.ErrNotFound
	}
	return u, nil
}

func (r *stubRepo) GetByUser(userID string) ([]model.URL, error) {
	urls := make([]model.URL, 0)
	for _, u := range r.urls {
		if u.UserID == userID {
			urls = append(urls, u)
		}
	}
	return urls, nil
}

func TestShortenBatch(t *testing.T) {
	repo := newStubRepo()
	svc := New(repo, "http://localhost:8080", 5)

	items := []model.BatchItem{
		{CorrelationID: "1", OriginalURL: "https://one.example"},
		{CorrelationID: "2", OriginalURL: "https://two.example"},
	}

	results, err := svc.ShortenBatch(items, "user-1")
	require.NoError(t, err)
	require.Len(t, results, 2)

	for i, res := range results {
		assert.Equal(t, items[i].CorrelationID, res.CorrelationID)
		assert.True(t, strings.HasPrefix(res.ShortURL, "http://localhost:8080/"), res.ShortURL)

		code := strings.TrimPrefix(res.ShortURL, "http://localhost:8080/")
		got, err := repo.Get(context.Background(), code)
		require.NoError(t, err)
		assert.Equal(t, items[i].OriginalURL, got.Original)
		assert.Equal(t, "user-1", got.UserID)
	}
}

func TestShortenBatch_RetriesOnConflict(t *testing.T) {
	repo := newStubRepo()
	repo.conflicts = 2
	svc := New(repo, "http://localhost:8080", 5)

	items := []model.BatchItem{
		{CorrelationID: "1", OriginalURL: "https://new.example"},
	}

	results, err := svc.ShortenBatch(items, "user-1")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "1", results[0].CorrelationID)
	assert.Equal(t, 3, repo.batchWriteCalls)

	code := strings.TrimPrefix(results[0].ShortURL, "http://localhost:8080/")
	got, err := repo.Get(context.Background(), code)
	require.NoError(t, err)
	assert.Equal(t, "https://new.example", got.Original)
}

func TestShortenBatch_DuplicateURLDoesNotRetry(t *testing.T) {
	repo := newStubRepo()
	repo.duplicate = true
	svc := New(repo, "http://localhost:8080", 5)

	items := []model.BatchItem{
		{CorrelationID: "1", OriginalURL: "https://dup.example"},
	}

	results, err := svc.ShortenBatch(items, "user-1")
	require.Nil(t, results)

	var dupErr *repository.DuplicateURLError
	require.ErrorAs(t, err, &dupErr)
	assert.Equal(t, 1, repo.batchWriteCalls)
}

func TestShorten_ExistingURLReturnsExistingShortURL(t *testing.T) {
	repo := newStubRepo()
	svc := New(repo, "http://localhost:8080", 5)

	first, err := svc.Shorten("https://duplicate.example", "user-1")
	require.NoError(t, err)

	second, err := svc.Shorten("https://duplicate.example", "user-2")
	require.ErrorIs(t, err, repository.ErrURLAlreadyExists)
	assert.Equal(t, first, second)

	got, err := repo.Get(context.Background(), strings.TrimPrefix(second, "http://localhost:8080/"))
	require.NoError(t, err)
	assert.Equal(t, "https://duplicate.example", got.Original)
}

func TestListByUser(t *testing.T) {
	repo := newStubRepo()
	svc := New(repo, "http://localhost:8080", 5)

	_, err := svc.Shorten("https://one.example", "user-a")
	require.NoError(t, err)
	_, err = svc.Shorten("https://two.example", "user-b")
	require.NoError(t, err)

	urls, err := svc.ListByUser("user-a")
	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Equal(t, "https://one.example", urls[0].Original)

	empty, err := svc.ListByUser("nobody")
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestShorten_RetriesOnConflict(t *testing.T) {
	repo := newStubRepo()
	repo.conflictOnWrite = 1
	svc := New(repo, "http://localhost:8080", 5)

	shortURL, err := svc.Shorten("https://retry.example", "user-1")
	require.NoError(t, err)
	assert.Equal(t, 2, repo.writeCalls)

	code := strings.TrimPrefix(shortURL, "http://localhost:8080/")
	got, err := repo.Get(context.Background(), code)
	require.NoError(t, err)
	assert.Equal(t, "https://retry.example", got.Original)
}
