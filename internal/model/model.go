package model

import "time"

type URL struct {
	Original  string
	Code      string
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt *time.Time
}
