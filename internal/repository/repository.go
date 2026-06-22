package repository

import (
	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/service"
	"sync"
)

type URLRepository struct {
	mutex sync.RWMutex
	store map[string]model.URL
}

func New() *URLRepository {
	return &URLRepository{
		store: make(map[string]model.URL),
	}
}

func (repo *URLRepository) Write(url model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	if _, ok := repo.store[url.Code]; ok {
		return service.ErrConflict
	}

	repo.store[url.Code] = url
	return nil
}

func (repo *URLRepository) Delete(url model.URL) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	delete(repo.store, url.Code)
}

func (repo *URLRepository) Get(code string) (model.URL, error) {
	repo.mutex.RLock()
	defer repo.mutex.RUnlock()

	url, ok := repo.store[code]

	if !ok {
		return model.URL{}, service.ErrNotFound
	}

	return url, nil
}
