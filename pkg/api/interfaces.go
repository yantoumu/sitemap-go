package api

import "context"

// APIResponse represents Google Trends API response
type APIResponse struct {
	Keywords []Keyword `json:"keywords"`
	Status   string    `json:"status"`
	Message  string    `json:"message"`
}

// Keyword represents trend data for a single keyword
type Keyword struct {
	Word         string  `json:"word"`
	SearchVolume int     `json:"search_volume"`
	Competition  float64 `json:"competition"`
	CPC          float64 `json:"cpc"`
}

// APIClient interface for Google Trends API
type APIClient interface {
	Query(ctx context.Context, keywords []string) (*APIResponse, error)
}