package model

import "time"

type URL struct {
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	ExpiresAt *time.Time `json:"-"`
	UUID      string     `json:"uuid"`
	Original  string     `json:"original_url"`
	Code      string     `json:"short_url"`
	UserID    string     `json:"-"`
	Deleted   bool       `json:"-"`
}

type BatchItem struct {
	CorrelationID string
	OriginalURL   string
	ShortURL      string
}
