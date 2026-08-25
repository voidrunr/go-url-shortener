package model

import "time"

type URL struct {
	UUID      string     `json:"uuid"`
	Original  string     `json:"original_url"`
	Code      string     `json:"short_url"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	ExpiresAt *time.Time `json:"-"`
}

type BatchItem struct {
	CorrelationID string
	OriginalURL   string
	ShortURL      string
}
