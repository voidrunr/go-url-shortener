package model

import "time"

type Url struct {
	Original	string
	Code		string
	CreatedAt	time.Time
	UpdatedAt	time.Time
	ExpiresAt	time.Time
}
