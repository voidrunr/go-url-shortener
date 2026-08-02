package repository

import (
	"time"

	"github.com/voidrunr/go-url-shortener/internal/model"
)

func testURL(code, original string) model.URL {
	now := time.Now()
	return model.URL{
		UUID:      "test-uuid",
		Original:  original,
		Code:      code,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
