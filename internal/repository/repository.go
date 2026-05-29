package repository

import (
	"errors"
	"sync"
	"github.com/voidrunr/go-url-shortener/internal/model"
)

type UrlRepository struct {
	mutex	sync.RWMutex
	store	map[string]model.Url
}

func New () *UrlRepository {
	return &UrlRepository{
		store: make(map[string]model.Url),
	}
}

func (repo *UrlRepository) Write (url model.Url)  {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	repo.store[url.Code] = url
}

func (repo *UrlRepository) Delete (url model.Url) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	delete(repo.store, url.Code)
}

func (repo *UrlRepository) Get (code string) (model.Url, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	url, ok := repo.store[code]

	if !ok {
		return model.Url{}, errors.New("Url not found")
	}

	return url, nil
}
