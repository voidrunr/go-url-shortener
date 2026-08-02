package repository

import (
	"errors"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

var (
	ErrNotFound = errors.New("url not found")
	ErrConflict = errors.New("code already exists")
)

type Repository interface {
	Write(model.URL) error
	WriteBatch([]model.URL) error
	Get(string) (model.URL, error)
}
