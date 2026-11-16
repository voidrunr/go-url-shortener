package repository

import (
	"context"
	"sync"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

type MemoryRepository struct {
	mutex      sync.RWMutex
	store      map[string]model.URL
	byOriginal map[string]string
}

func NewMemory() *MemoryRepository {
	return &MemoryRepository{
		store:      make(map[string]model.URL),
		byOriginal: make(map[string]string),
	}
}

func (repo *MemoryRepository) Write(ctx context.Context, url model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	if code, ok := repo.byOriginal[url.Original]; ok {
		return &DuplicateURLError{URL: repo.store[code]}
	}

	if _, ok := repo.store[url.Code]; ok {
		return ErrConflict
	}

	repo.store[url.Code] = url
	repo.byOriginal[url.Original] = url.Code
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
		repo.byOriginal[url.Original] = url.Code
	}
	return nil
}

func (repo *MemoryRepository) Get(ctx context.Context, code string) (model.URL, error) {
	repo.mutex.RLock()
	defer repo.mutex.RUnlock()

	url, ok := repo.store[code]

	if !ok {
		return model.URL{}, ErrNotFound
	}

	return url, nil
}

func (repo *MemoryRepository) GetByUser(ctx context.Context, userID string) ([]model.URL, error) {
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

func (repo *MemoryRepository) DeleteBatch(ctx context.Context, userID string, codes []string) error {
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
