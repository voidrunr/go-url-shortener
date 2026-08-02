package repository

import (
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

func (repo *MemoryRepository) Write(url model.URL) error {
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

func (repo *MemoryRepository) WriteBatch(urls []model.URL) error {
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

func (repo *MemoryRepository) Get(code string) (model.URL, error) {
	repo.mutex.RLock()
	defer repo.mutex.RUnlock()

	url, ok := repo.store[code]

	if !ok {
		return model.URL{}, ErrNotFound
	}

	return url, nil
}

func (repo *MemoryRepository) findByOriginal(original string) (model.URL, bool) {
	for _, url := range repo.store {
		if url.Original == original {
			return url, true
		}
	}

	return model.URL{}, false
}
