package api

import "context"

type APIResponse struct {
	Keywords []Keyword `json:"keywords"`
	Status   string    `json:"status"`
	Message  string    `json:"message"`
}

type Keyword struct {
	Word         string  `json:"word"`
	SearchVolume int     `json:"search_volume"`
	Competition  float64 `json:"competition"`
	CPC          float64 `json:"cpc"`
}

type APIMetrics struct {
	RequestCount  int64   `json:"request_count"`
	ErrorCount    int64   `json:"error_count"`
	AvgLatency    float64 `json:"avg_latency"`
	SuccessRate   float64 `json:"success_rate"`
}

type HealthStatus struct {
	Healthy   bool   `json:"healthy"`
	LastCheck string `json:"last_check"`
	Message   string `json:"message"`
}

type APIClient interface {
	Query(ctx context.Context, keywords []string) (*APIResponse, error)
	HealthCheck(ctx context.Context) error
	GetMetrics() *APIMetrics
}

type APIPool interface {
	GetClient() (APIClient, error)
	ReturnClient(client APIClient)
	HealthStatus() map[string]HealthStatus
}