package repository

import (
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

func (e *DuplicateURLError) Is(target error) bool {
	return target == ErrURLAlreadyExists
}

type Repository interface {
	Write(model.URL) error
	WriteBatch([]model.URL) error
	Get(string) (model.URL, error)
	GetByUser(string) ([]model.URL, error)
	DeleteBatch(userID string, codes []string) error
}
