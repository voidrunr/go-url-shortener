package service

import (
	"crypto/rand"
	"fmt"
	"time"
	"encoding/base64"

	"github.com/voidrunr/go-url-shortener/internal/model"
	"github.com/voidrunr/go-url-shortener/internal/repository"
)

type UrlService struct {
	repo	*repository.UrlRepository
	baseUrl	string
}

func New (repo *repository.UrlRepository, baseUrl string) *UrlService {
	return &UrlService{
		repo:		repo,
		baseUrl:	baseUrl,
	}
}

func generateCode (len int) (string, error) {
	b := make([]byte, len)

	_, err := rand.Read(b);
	if  err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b)[:len], nil
}

func (srv UrlService) Shorten (originalUrl string) (string, error) {
	code, err := generateCode(6)
	if err != nil {
		return "", err
	}

	now := time.Now()

	url := model.Url{
		Original:	originalUrl,
		Code:		code,
		CreatedAt:	now,
		UpdatedAt:	now,
	}

	srv.repo.Write(url)

	return fmt.Sprintf("%s/%s", srv.baseUrl, code), nil
}

func (srv UrlService) Resolve (code string) (string, error) {
	url, err := srv.repo.Get(code)
	if err != nil {
		return "", err
	}

	return url.Original, err
}
