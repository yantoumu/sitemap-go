package backend

import "time"

// KeywordMetricsBatch represents the batch submission request
type KeywordMetricsBatch []KeywordMetricsData

// KeywordMetricsData represents a single keyword's metrics data
type KeywordMetricsData struct {
	Keyword string       `json:"keyword"`
	URL     string       `json:"url,omitempty"`
	Metrics MetricsData  `json:"metrics"`
}

// MetricsData contains all metrics for a keyword
type MetricsData struct {
	AvgMonthlySearches        int64                `json:"avg_monthly_searches"`
	LatestSearches           int64                `json:"latest_searches"`
	MaxMonthlySearches       int64                `json:"max_monthly_searches"`
	Competition              string               `json:"competition"`
	CompetitionIndex         int                  `json:"competition_index"`
	LowTopOfPageBidMicro     int64                `json:"low_top_of_page_bid_micro"`
	HighTopOfPageBidMicro    int64                `json:"high_top_of_page_bid_micro"`
	MonthlySearches          []MonthlySearchData  `json:"monthly_searches"`
	DataQuality              DataQualityInfo      `json:"data_quality"`
}

// MonthlySearchData represents monthly search volume data
type MonthlySearchData struct {
	Year     interface{} `json:"year"`     // Support both string and number
	Month    interface{} `json:"month"`    // Support both string and number
	Searches int64       `json:"searches"`
}

// DataQualityInfo represents data quality metrics
type DataQualityInfo struct {
	Status                  string   `json:"status"`
	Complete                bool     `json:"complete"`
	HasMissingMonths        bool     `json:"has_missing_months"`
	OnlyLastMonthHasData    bool     `json:"only_last_month_has_data"`
	TotalMonths             int      `json:"total_months"`
	AvailableMonths         int      `json:"available_months"`
	MissingMonthsCount      int      `json:"missing_months_count"`
	MissingMonths           []string `json:"missing_months"`
	Warnings                []string `json:"warnings"`
}

// BackendResponse represents the API response from backend
type BackendResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// BackendConfig holds backend API configuration
type BackendConfig struct {
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	BatchSize  int    `json:"batch_size"`   // Keywords per batch (default: 4)
	EnableGzip bool   `json:"enable_gzip"`
	Timeout    time.Duration `json:"timeout"`
}

// BackendClient interface for submitting metrics data
type BackendClient interface {
	SubmitBatch(batch KeywordMetricsBatch) (*BackendResponse, error)
	SubmitBatches(data []KeywordMetricsData) error
}