package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"time"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

var ErrNotFound = errors.New("url not found")

type Repository interface {
	Write(model.URL)
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

	var code string
	for i := 0; unlimited || i < srv.collisionRetries; i++ {
		var err error
		code, err = generateCode(6)
		if err != nil {
			return "", err
		}

		_, err = srv.repo.Get(code)
		if errors.Is(err, ErrNotFound) {
			break
		}
		if err != nil {
			return "", err
		}
	}

	now := time.Now()

	u := model.URL{
		Original:  originalURL,
		Code:      code,
		CreatedAt: now,
		UpdatedAt: now,
	}

	srv.repo.Write(u)

	return url.JoinPath(srv.baseURL, code)
}

func (srv URLService) Resolve(code string) (string, error) {
	url, err := srv.repo.Get(code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}

	return url.Original, err
}
