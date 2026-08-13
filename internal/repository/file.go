package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

type FileRepository struct {
	mutex    sync.Mutex
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

func (repo *FileRepository) Write(ctx context.Context, url model.URL) error {
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

func (repo *FileRepository) WriteBatch(ctx context.Context, urls []model.URL) error {
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

func (repo *FileRepository) Delete(ctx context.Context, url model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	delete(repo.store, url.Code)
	return repo.saveToFile()
}

func (repo *FileRepository) Get(ctx context.Context, code string) (model.URL, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	url, ok := repo.store[code]

	if !ok {
		return model.URL{}, ErrNotFound
	}

	return url, nil
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
