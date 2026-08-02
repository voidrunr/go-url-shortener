package repository

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

type FileRepository struct {
	mutex    sync.RWMutex
	store    map[string]model.URL
	filePath string
}

func NewFile(filePath string) (*FileRepository, error) {
	repo := &FileRepository{
		store:    make(map[string]model.URL),
		filePath: filePath,
	}
	if err := repo.loadFromFile(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (repo *FileRepository) Write(url model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	if existing, ok := repo.findByOriginal(url.Original); ok {
		return &DuplicateURLError{URL: existing}
	}

	if _, ok := repo.store[url.Code]; ok {
		return ErrConflict
	}

	repo.store[url.Code] = url
	return repo.saveToFile()
}

func (repo *FileRepository) WriteBatch(urls []model.URL) error {
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
	return repo.saveToFile()
}

func (repo *FileRepository) Delete(url model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	delete(repo.store, url.Code)
	return repo.saveToFile()
}

func (repo *FileRepository) Get(code string) (model.URL, error) {
	repo.mutex.RLock()
	defer repo.mutex.RUnlock()

	url, ok := repo.store[code]

	if !ok {
		return model.URL{}, ErrNotFound
	}

	return url, nil
}

func (repo *FileRepository) GetByUser(userID string) ([]model.URL, error) {
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

func (repo *FileRepository) findByOriginal(original string) (model.URL, bool) {
	for _, url := range repo.store {
		if url.Original == original {
			return url, true
		}
	}

	return model.URL{}, false
}

func (repo *FileRepository) saveToFile() error {
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

func (repo *FileRepository) loadFromFile() error {
	data, err := os.ReadFile(repo.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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
