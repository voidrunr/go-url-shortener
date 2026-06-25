package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"time"

	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

type Repository interface {
	Write(model.URL) error
	Get(string) (model.URL, error)
}

type URLService struct {
	repo             Repository
	baseURL          string
	collisionRetries int
}

func New(repo Repository, baseURL string, collisionRetries int) *URLService {
	return &URLService{
		repo:             repo,
		baseURL:          baseURL,
		collisionRetries: collisionRetries,
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

func (srv URLService) Shorten(originalURL string) (string, error) {
	unlimited := srv.collisionRetries <= 0

	for i := 0; unlimited || i < srv.collisionRetries; i++ {
		code, err := generateCode(6)
		if err != nil {
			return "", err
		}

		now := time.Now()
		u := model.URL{
			Original:  originalURL,
			Code:      code,
			CreatedAt: now,
			UpdatedAt: now,
		}

		err = srv.repo.Write(u)
		if err == nil {
			return url.JoinPath(srv.baseURL, code)
		}
		if errors.Is(err, repository.ErrConflict) {
			continue
		}
		return "", err
	}

	return "", errors.New("failed to generate unique code after max retries")
}

func (srv URLService) Resolve(code string) (string, error) {
	url, err := srv.repo.Get(code)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", repository.ErrNotFound
		}
		return "", err
	}

	return url.Original, err
}
