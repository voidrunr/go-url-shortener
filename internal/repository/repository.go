package repository

import (
	"encoding/json"
	"os"
	"sync"

	"errors"
	"github.com/voidrunr/go-url-shortener/internal/model"
)

var (
	ErrNotFound = errors.New("url not found")
	ErrConflict = errors.New("code already exists")
)

type URLRepository struct {
	mutex    sync.RWMutex
	store    map[string]model.URL
	filePath string
}

func New(filePath string) (*URLRepository, error) {
	repo := &URLRepository{
		store:    make(map[string]model.URL),
		filePath: filePath,
	}
	if err := repo.loadFromFile(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (repo *URLRepository) Write(url model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	if _, ok := repo.store[url.Code]; ok {
		return ErrConflict
	}

	repo.store[url.Code] = url
	return repo.saveToFile()
}

func (repo *URLRepository) Delete(url model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	delete(repo.store, url.Code)
	return repo.saveToFile()
}

func (repo *URLRepository) Get(code string) (model.URL, error) {
	repo.mutex.RLock()
	defer repo.mutex.RUnlock()

	url, ok := repo.store[code]

	if !ok {
		return model.URL{}, ErrNotFound
	}

	return url, nil
}

func (repo *URLRepository) saveToFile() error {
	urls := make([]model.URL, 0, len(repo.store))
	for _, url := range repo.store {
		urls = append(urls, url)
	}

	data, err := json.MarshalIndent(urls, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(repo.filePath, data, 0644)
}

func (repo *URLRepository) loadFromFile() error {
	data, err := os.ReadFile(repo.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var urls []model.URL
	if err := json.Unmarshal(data, &urls); err != nil {
		return err
	}

	for _, url := range urls {
		repo.store[url.Code] = url
	}
	return nil
}
