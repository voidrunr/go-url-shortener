package repository

import (
	"encoding/json"
	"log"
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

func New(filePath string) *URLRepository {
	repo := &URLRepository{
		store:    make(map[string]model.URL),
		filePath: filePath,
	}
	repo.loadFromFile()
	return repo
}

func (repo *URLRepository) Write(url model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	if _, ok := repo.store[url.Code]; ok {
		return ErrConflict
	}

	repo.store[url.Code] = url
	repo.saveToFile()

	return nil
}

func (repo *URLRepository) Delete(url model.URL) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	delete(repo.store, url.Code)
	repo.saveToFile()
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

func (repo *URLRepository) saveToFile() {
	urls := make([]model.URL, 0, len(repo.store))
	for _, url := range repo.store {
		urls = append(urls, url)
	}

	data, err := json.MarshalIndent(urls, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal URLs: %v", err)
		return
	}

	if err := os.WriteFile(repo.filePath, data, 0644); err != nil {
		log.Printf("Failed to write URLs to file: %v", err)
	}
}

func (repo *URLRepository) loadFromFile() {
	data, err := os.ReadFile(repo.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("Failed to read URLs file: %v", err)
		return
	}

	var urls []model.URL
	if err := json.Unmarshal(data, &urls); err != nil {
		log.Printf("Failed to unmarshal URLs: %v", err)
		return
	}

	for _, url := range urls {
		repo.store[url.Code] = url
	}
}
