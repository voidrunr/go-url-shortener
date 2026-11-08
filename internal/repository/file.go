package repository

import (
	"bufio"
	"bytes"
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
	mutex      sync.RWMutex
	store      map[string]model.URL
	byOriginal map[string]string
	filePath   string
}

func NewFile(filePath string) (*FileRepository, error) {
	repo := &FileRepository{
		store:      make(map[string]model.URL),
		byOriginal: make(map[string]string),
		filePath:   filePath,
	}
	if err := repo.loadFromFile(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (repo *FileRepository) Write(ctx context.Context, url model.URL) error {
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
	return repo.appendEntries([]fileEntry{toFileEntry(url)})
}

func (repo *FileRepository) WriteBatch(ctx context.Context, urls []model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	for _, url := range urls {
		if _, ok := repo.store[url.Code]; ok {
			return ErrConflict
		}
	}

	entries := make([]fileEntry, 0, len(urls))
	for _, url := range urls {
		repo.store[url.Code] = url
		repo.byOriginal[url.Original] = url.Code
		entries = append(entries, toFileEntry(url))
	}
	return repo.appendEntries(entries)
}

func (repo *FileRepository) Delete(ctx context.Context, url model.URL) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	delete(repo.store, url.Code)
	if code, ok := repo.byOriginal[url.Original]; ok && code == url.Code {
		delete(repo.byOriginal, url.Original)
	}
	return repo.rewrite()
}

func (repo *FileRepository) Get(ctx context.Context, code string) (model.URL, error) {
	repo.mutex.RLock()
	defer repo.mutex.RUnlock()

	url, ok := repo.store[code]

	if !ok {
		return model.URL{}, ErrNotFound
	}

	return url, nil
}

func (repo *FileRepository) GetByUser(ctx context.Context, userID string) ([]model.URL, error) {
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

func (repo *FileRepository) DeleteBatch(ctx context.Context, userID string, codes []string) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()

	wanted := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		wanted[code] = struct{}{}
	}

	var tombstones []fileEntry
	for code, url := range repo.store {
		if url.UserID != userID {
			continue
		}
		if _, ok := wanted[code]; ok {
			url.Deleted = true
			repo.store[code] = url
			tombstones = append(tombstones, fileEntry{Code: code, Deleted: true})
		}
	}

	if len(tombstones) == 0 {
		return nil
	}
	return repo.appendEntries(tombstones)
}

func (repo *FileRepository) appendEntries(entries []fileEntry) error {
	fh, err := os.OpenFile(repo.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fh.Close()

	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if _, err := fh.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func (repo *FileRepository) rewrite() error {
	entries := make([]fileEntry, 0, len(repo.store))
	for _, url := range repo.store {
		entries = append(entries, toFileEntry(url))
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(repo.filePath, data, 0o644)
}

func (repo *FileRepository) applyEntry(e fileEntry) {
	if existing, ok := repo.store[e.Code]; ok && e.Deleted {
		existing.Deleted = true
		repo.store[e.Code] = existing
		return
	}

	url := fromFileEntry(e)
	if !e.Deleted {
		repo.byOriginal[url.Original] = url.Code
	}
	repo.store[url.Code] = url
}

func (repo *FileRepository) loadFromFile() error {
	data, err := os.ReadFile(repo.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil
	}

	if trimmed[0] == '[' {
		var entries []fileEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			return err
		}
		for _, e := range entries {
			repo.applyEntry(e)
		}
		return nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var e fileEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return err
		}
		repo.applyEntry(e)
	}
	return scanner.Err()
}
