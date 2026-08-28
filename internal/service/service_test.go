package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

type stubRepo struct {
	mutex           sync.RWMutex
	urls            map[string]model.URL
	writeCalls      int
	conflicts       int
	batchWriteCalls int
	conflictOnWrite int
	duplicate       bool
}

func newStubRepo() *stubRepo {
	return &stubRepo{urls: make(map[string]model.URL)}
}

func (r *stubRepo) Write(ctx context.Context, url model.URL) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.writeCalls++
	if r.conflictOnWrite > 0 {
		r.conflictOnWrite--
		return repository.ErrConflict
	}
	_ = ctx
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
	r.mutex.Lock()
	defer r.mutex.Unlock()
	_ = ctx

	r.batchWriteCalls++
	if r.duplicate {
		return &repository.DuplicateURLError{URL: model.URL{Code: "existing", Original: urls[0].Original}}
	}
	if r.conflicts > 0 {
		r.conflicts--
		return repository.ErrConflict
	}

	for _, u := range urls {
		for _, existing := range r.urls {
			if existing.Original == u.Original {
				return &repository.DuplicateURLError{URL: existing}
			}
		}
		if _, ok := r.urls[u.Code]; ok {
			return repository.ErrConflict
		}
		if r.conflictOnWrite > 0 {
			r.conflictOnWrite--
			return repository.ErrConflict
		}
		r.writeCalls++
		r.urls[u.Code] = u
	}
	return nil
}

func (r *stubRepo) Get(ctx context.Context, code string) (model.URL, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	_ = ctx
	u, ok := r.urls[code]
	if !ok {
		return model.URL{}, repository.ErrNotFound
	}
	return u, nil
}

func (r *stubRepo) GetByUser(userID string) ([]model.URL, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	urls := make([]model.URL, 0)
	for _, u := range r.urls {
		if u.UserID == userID {
			urls = append(urls, u)
		}
	}
	return urls, nil
}

func (r *stubRepo) DeleteBatch(userID string, codes []string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	wanted := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		wanted[c] = struct{}{}
	}
	for code, u := range r.urls {
		if u.UserID != userID {
			continue
		}
		if _, ok := wanted[code]; ok {
			u.Deleted = true
			r.urls[code] = u
		}
	}
	return nil
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

func TestDelete_MarksOwnedURLsAsGone(t *testing.T) {
	repo := newStubRepo()
	svc := NewWithDeleter(repo, "http://localhost:8080", 5, 10*time.Millisecond, 16)

	urlA1, err := svc.Shorten("https://delete-one.example", "user-a")
	require.NoError(t, err)
	urlA2, err := svc.Shorten("https://delete-two.example", "user-a")
	require.NoError(t, err)
	urlB, err := svc.Shorten("https://delete-other.example", "user-b")
	require.NoError(t, err)

	codeA1 := strings.TrimPrefix(urlA1, "http://localhost:8080/")
	codeA2 := strings.TrimPrefix(urlA2, "http://localhost:8080/")
	codeB := strings.TrimPrefix(urlB, "http://localhost:8080/")

	err = svc.Delete("user-a", []string{codeA1, codeA2})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err := svc.Resolve(codeA1)
		return errors.Is(err, repository.ErrGone)
	}, 2*time.Second, 10*time.Millisecond)

	_, err = svc.Resolve(codeA2)
	require.ErrorIs(t, err, repository.ErrGone)

	_, err = svc.Resolve(codeB)
	require.NoError(t, err)
}

func TestDelete_EmptyCodesIsNoop(t *testing.T) {
	repo := newStubRepo()
	svc := NewWithDeleter(repo, "http://localhost:8080", 5, 10*time.Millisecond, 16)

	url, err := svc.Shorten("https://keep.example", "user-a")
	require.NoError(t, err)
	code := strings.TrimPrefix(url, "http://localhost:8080/")

	err = svc.Delete("user-a", nil)
	require.NoError(t, err)

	_, err = svc.Resolve(code)
	require.NoError(t, err)
}

func TestResolve_DeletedURLReturnsGone(t *testing.T) {
	repo := newStubRepo()
	svc := NewWithDeleter(repo, "http://localhost:8080", 5, 10*time.Millisecond, 16)

	url, err := svc.Shorten("https://gone.example", "user-a")
	require.NoError(t, err)
	code := strings.TrimPrefix(url, "http://localhost:8080/")

	err = svc.Delete("user-a", []string{code})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err := svc.Resolve(code)
		return errors.Is(err, repository.ErrGone)
	}, 2*time.Second, 10*time.Millisecond)
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
