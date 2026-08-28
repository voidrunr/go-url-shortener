package repository

import (
	"context"
	"sync"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

type MemoryRepository struct {
	mutex sync.RWMutex
	store map[string]model.URL
}

func NewMemory() *MemoryRepository {
	return &MemoryRepository{
		store: make(map[string]model.URL),
	}
}

func (repo *MemoryRepository) Write(ctx context.Context, url model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	if existing, ok := repo.findByOriginal(url.Original); ok {
		return &DuplicateURLError{URL: existing}
	}

	if _, ok := repo.store[url.Code]; ok {
		return ErrConflict
	}

	repo.store[url.Code] = url
	return nil
}

func (repo *MemoryRepository) WriteBatch(ctx context.Context, urls []model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	for _, url := range urls {
		if _, ok := repo.store[url.Code]; ok {
			return ErrConflict
		}
	}

	for _, url := range urls {
		repo.store[url.Code] = url
	}
	return nil
}

func (repo *MemoryRepository) Get(ctx context.Context, code string) (model.URL, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	url, ok := repo.store[code]

	if !ok {
		return model.URL{}, ErrNotFound
	}

	return url, nil
}

func (repo *MemoryRepository) GetByUser(userID string) ([]model.URL, error) {
	repo.mutex.RLock()
	defer repo.mutex.RUnlock()

	urls := make([]model.URL, 0)
	for _, url := range repo.store {
		if url.UserID == userID {
			urls = append(urls, url)
		}
	}

	return urls, nil
}

func (repo *MemoryRepository) DeleteBatch(userID string, codes []string) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	wanted := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		wanted[code] = struct{}{}
	}

	for code, url := range repo.store {
		if url.UserID != userID {
			continue
		}
		if _, ok := wanted[code]; ok {
			url.Deleted = true
			repo.store[code] = url
		}
	}

	return nil
}

func (repo *MemoryRepository) findByOriginal(original string) (model.URL, bool) {
	for _, url := range repo.store {
		if url.Original == original {
			return url, true
		}
	}

	return model.URL{}, false
}
