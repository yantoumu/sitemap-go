package api

import (
	"encoding/json"
	"fmt"
)

// SEOKeyResponse represents the raw response structure from SEOKey API
// Follows Single Responsibility Principle - only handles SEOKey API response format
type SEOKeyResponse struct {
	Status string `json:"status"`
	Data   []struct {
		Keyword string `json:"keyword"`
		Metrics struct {
			AvgMonthlySearches int    `json:"avg_monthly_searches"`
			Competition        string `json:"competition"`
			LatestSearches     int    `json:"latest_searches"`
		} `json:"metrics"`
	} `json:"data"`
}

// SEOKeyParser provides unified parsing logic for SEOKey API responses
// Follows DRY principle by centralizing duplicate parsing logic
type SEOKeyParser struct{}

// NewSEOKeyParser creates a new SEOKey response parser
func NewSEOKeyParser() *SEOKeyParser {
	return &SEOKeyParser{}
}

// ParseResponse converts SEOKey API response to standardized APIResponse format
// Centralizes the competition value mapping and response transformation logic
func (p *SEOKeyParser) ParseResponse(body []byte) (*APIResponse, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty response body from SEOKey API")
	}

	var seokeyResp SEOKeyResponse

	if err := json.Unmarshal(body, &seokeyResp); err != nil {
		return nil, fmt.Errorf("failed to decode SEOKey response: %w (response: %s)", err, string(body[:min(len(body), 200)]))
	}

	// Enhanced error handling for different API response states
	if seokeyResp.Status != "success" {
		return &APIResponse{
			Status:  "error",
			Message: fmt.Sprintf("SEOKey API returned status: %s", seokeyResp.Status),
		}, nil
	}

	if len(seokeyResp.Data) == 0 {
		return &APIResponse{
			Status:  "success",
			Message: "No keyword data available",
			Keywords: []Keyword{},
		}, nil
	}

	// Convert to standardized APIResponse format
	apiResp := APIResponse{
		Status:   "success",
		Keywords: make([]Keyword, 0, len(seokeyResp.Data)),
	}

	for _, data := range seokeyResp.Data {
		// Validate keyword data before processing
		if data.Keyword == "" {
			continue // Skip empty keywords
		}

		keyword := Keyword{
			Word:         data.Keyword,
			SearchVolume: data.Metrics.AvgMonthlySearches,
			Competition:  p.mapCompetitionValue(data.Metrics.Competition),
			CPC:          0, // SEOKey API doesn't provide CPC data
		}
		apiResp.Keywords = append(apiResp.Keywords, keyword)
	}

	return &apiResp, nil
}

// min returns the minimum of two integers (helper function)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// mapCompetitionValue converts SEOKey competition strings to numeric values
// Centralizes competition mapping logic to ensure consistency
func (p *SEOKeyParser) mapCompetitionValue(competition string) float64 {
	switch competition {
	case "LOW":
		return 0.3
	case "HIGH":
		return 0.8
	case "MEDIUM":
		return 0.5
	default:
		return 0.5 // Default to medium competition
	}
}

// ValidateResponse checks if the SEOKey response is valid and contains data
func (p *SEOKeyParser) ValidateResponse(body []byte) error {
	var seokeyResp SEOKeyResponse
	
	if err := json.Unmarshal(body, &seokeyResp); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}
	
	if seokeyResp.Status != "success" {
		return fmt.Errorf("API returned non-success status: %s", seokeyResp.Status)
	}
	
	if len(seokeyResp.Data) == 0 {
		return fmt.Errorf("API returned empty data array")
	}
	
	return nil
}

// GetKeywordCount returns the number of keywords in the response
func (p *SEOKeyParser) GetKeywordCount(body []byte) (int, error) {
	var seokeyResp SEOKeyResponse
	
	if err := json.Unmarshal(body, &seokeyResp); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return len(seokeyResp.Data), nil
}
