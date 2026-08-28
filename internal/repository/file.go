package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

type fileEntry struct {
	UUID      string     `json:"uuid"`
	Original  string     `json:"original_url"`
	Code      string     `json:"short_url"`
	UserID    string     `json:"user_id"`
	Deleted   bool       `json:"is_deleted"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func toFileEntry(url model.URL) fileEntry {
	return fileEntry{
		UUID:      url.UUID,
		Original:  url.Original,
		Code:      url.Code,
		UserID:    url.UserID,
		Deleted:   url.Deleted,
		CreatedAt: url.CreatedAt,
		UpdatedAt: url.UpdatedAt,
		ExpiresAt: url.ExpiresAt,
	}
}

func fromFileEntry(e fileEntry) model.URL {
	return model.URL{
		UUID:      e.UUID,
		Original:  e.Original,
		Code:      e.Code,
		UserID:    e.UserID,
		Deleted:   e.Deleted,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
		ExpiresAt: e.ExpiresAt,
	}
}

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

func (repo *FileRepository) DeleteBatch(userID string, codes []string) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	wanted := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		wanted[code] = struct{}{}
	}

	changed := false
	for code, url := range repo.store {
		if url.UserID != userID {
			continue
		}
		if _, ok := wanted[code]; ok {
			url.Deleted = true
			repo.store[code] = url
			changed = true
		}
	}

	if !changed {
		return nil
	}
	return repo.saveToFile()
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
	entries := make([]fileEntry, 0, len(repo.store))
	for _, url := range repo.store {
		entries = append(entries, toFileEntry(url))
	}

	data, err := json.MarshalIndent(entries, "", "  ")
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

	var entries []fileEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	for _, e := range entries {
		url := fromFileEntry(e)
		repo.store[url.Code] = url
	}
	return nil
}
