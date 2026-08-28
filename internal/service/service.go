package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

type Repository interface {
	Write(ctx context.Context, url model.URL) error
	WriteBatch(ctx context.Context, urls []model.URL) error
	Get(ctx context.Context, code string) (model.URL, error)
	GetByUser(string) ([]model.URL, error)
	DeleteBatch(userID string, codes []string) error
}

type URLService struct {
	repo             Repository
	baseURL          string
	collisionRetries int
	deleter          *deleter
}

const (
	defaultFlushInterval = 100 * time.Millisecond
	defaultQueueSize     = 256
)

func New(repo Repository, baseURL string, collisionRetries int) *URLService {
	return NewWithDeleter(repo, baseURL, collisionRetries, defaultFlushInterval, defaultQueueSize)
}

func NewWithDeleter(repo Repository, baseURL string, collisionRetries int, flushInterval time.Duration, queueSize int) *URLService {
	dl := newDeleter(repo, flushInterval, queueSize)
	dl.start()
	return &URLService{
		repo:             repo,
		baseURL:          baseURL,
		collisionRetries: collisionRetries,
		deleter:          dl,
	}
}

func generateCode(n int) (string, error) {
	b := make([]byte, n)

	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}

func generateUUID() (string, error) {
	b := make([]byte, 16)

	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func (srv URLService) Shorten(originalURL string, userID string) (string, error) {
	unlimited := srv.collisionRetries <= 0

	for i := 0; unlimited || i < srv.collisionRetries; i++ {
		code, err := generateCode(6)
		if err != nil {
			return "", err
		}

		now := time.Now()

		uuid, err := generateUUID()
		if err != nil {
			return "", err
		}

		u := model.URL{
			UUID:      uuid,
			Original:  originalURL,
			Code:      code,
			UserID:    userID,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := srv.repo.Write(context.TODO(), u); err != nil {
			var dupErr *repository.DuplicateURLError
			if errors.As(err, &dupErr) {
				shortURL, joinErr := url.JoinPath(srv.baseURL, dupErr.URL.Code)
				if joinErr != nil {
					return "", joinErr
				}
				return shortURL, repository.ErrURLAlreadyExists
			}
			if errors.Is(err, repository.ErrConflict) {
				continue
			}
			return "", err
		}

		return url.JoinPath(srv.baseURL, code)
	}

	return "", fmt.Errorf("failed to generate unique code after %d attempts", srv.collisionRetries)
}

func (srv URLService) ShortenBatch(items []model.BatchItem, userID string) ([]model.BatchItem, error) {
	unlimited := srv.collisionRetries <= 0

	for i := 0; unlimited || i < srv.collisionRetries; i++ {
		now := time.Now()

		urls := make([]model.URL, 0, len(items))
		result := make([]model.BatchItem, 0, len(items))

		for _, item := range items {
			code, err := generateCode(6)
			if err != nil {
				return nil, err
			}

			uuid, err := generateUUID()
			if err != nil {
				return nil, err
			}

			urls = append(urls, model.URL{
				UUID:      uuid,
				Original:  item.OriginalURL,
				Code:      code,
				UserID:    userID,
				CreatedAt: now,
				UpdatedAt: now,
			})

			shortURL, err := url.JoinPath(srv.baseURL, code)
			if err != nil {
				return nil, err
			}

			result = append(result, model.BatchItem{
				CorrelationID: item.CorrelationID,
				ShortURL:      shortURL,
			})
		}

		if err := srv.repo.WriteBatch(context.TODO(), urls); err != nil {
			if errors.Is(err, repository.ErrConflict) {
				continue
			}
			return nil, err
		}

		return result, nil
	}

	return nil, fmt.Errorf("failed to generate unique codes after %d attempts", srv.collisionRetries)
}

func (srv URLService) Resolve(code string) (string, error) {
	url, err := srv.repo.Get(context.TODO(), code)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", repository.ErrNotFound
		}
		return "", err
	}

	if url.Deleted {
		return "", repository.ErrGone
	}

	return url.Original, err
}

func (srv URLService) Delete(userID string, codes []string) error {
	if len(codes) == 0 {
		return nil
	}
	srv.deleter.enqueue(deleteRequest{userID: userID, codes: codes})
	return nil
}

func (srv URLService) ListByUser(userID string) ([]model.URL, error) {
	return srv.repo.GetByUser(userID)
}
