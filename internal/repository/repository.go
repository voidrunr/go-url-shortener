package repository

import (
	"context"
	"errors"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

var (
	ErrNotFound         = errors.New("url not found")
	ErrConflict         = errors.New("code already exists")
	ErrURLAlreadyExists = errors.New("url already exists")
	ErrGone             = errors.New("url is deleted")
)

type DuplicateURLError struct {
	URL model.URL
}

func (e *DuplicateURLError) Error() string {
	return ErrURLAlreadyExists.Error()
}

func (e *DuplicateURLError) Unwrap() error {
	return ErrURLAlreadyExists
}

type Repository interface {
	Write(ctx context.Context, url model.URL) error
	WriteBatch(ctx context.Context, urls []model.URL) error
	Get(ctx context.Context, code string) (model.URL, error)
	GetByUser(string) ([]model.URL, error)
	DeleteBatch(userID string, codes []string) error
}
