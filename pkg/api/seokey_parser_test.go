package api

import (
	"testing"
)

func TestSEOKeyParser_ParseResponse_Success(t *testing.T) {
	parser := NewSEOKeyParser()
	
	// Test successful response
	responseBody := `{
		"status": "success",
		"data": [
			{
				"keyword": "test keyword",
				"metrics": {
					"avg_monthly_searches": 1000,
					"competition": "LOW",
					"latest_searches": 800
				}
			},
			{
				"keyword": "another keyword",
				"metrics": {
					"avg_monthly_searches": 2000,
					"competition": "HIGH",
					"latest_searches": 1500
				}
			}
		]
	}`
	
	result, err := parser.ParseResponse([]byte(responseBody))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if result.Status != "success" {
		t.Errorf("Expected status 'success', got: %s", result.Status)
	}
	
	if len(result.Keywords) != 2 {
		t.Errorf("Expected 2 keywords, got: %d", len(result.Keywords))
	}
	
	// Test first keyword
	if result.Keywords[0].Word != "test keyword" {
		t.Errorf("Expected 'test keyword', got: %s", result.Keywords[0].Word)
	}
	if result.Keywords[0].SearchVolume != 1000 {
		t.Errorf("Expected search volume 1000, got: %d", result.Keywords[0].SearchVolume)
	}
	if result.Keywords[0].Competition != 0.3 {
		t.Errorf("Expected competition 0.3 (LOW), got: %f", result.Keywords[0].Competition)
	}
	
	// Test second keyword
	if result.Keywords[1].Competition != 0.8 {
		t.Errorf("Expected competition 0.8 (HIGH), got: %f", result.Keywords[1].Competition)
	}
}

func TestSEOKeyParser_ParseResponse_EmptyData(t *testing.T) {
	parser := NewSEOKeyParser()
	
	responseBody := `{
		"status": "success",
		"data": []
	}`
	
	result, err := parser.ParseResponse([]byte(responseBody))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if result.Status != "success" {
		t.Errorf("Expected status 'success', got: %s", result.Status)
	}
	
	if len(result.Keywords) != 0 {
		t.Errorf("Expected 0 keywords, got: %d", len(result.Keywords))
	}
}

func TestSEOKeyParser_ParseResponse_ErrorStatus(t *testing.T) {
	parser := NewSEOKeyParser()
	
	responseBody := `{
		"status": "error",
		"data": []
	}`
	
	result, err := parser.ParseResponse([]byte(responseBody))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	
	if result.Status != "error" {
		t.Errorf("Expected status 'error', got: %s", result.Status)
	}
}

func TestSEOKeyParser_ParseResponse_InvalidJSON(t *testing.T) {
	parser := NewSEOKeyParser()
	
	responseBody := `invalid json`
	
	_, err := parser.ParseResponse([]byte(responseBody))
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestSEOKeyParser_ParseResponse_EmptyBody(t *testing.T) {
	parser := NewSEOKeyParser()
	
	_, err := parser.ParseResponse([]byte{})
	if err == nil {
		t.Fatal("Expected error for empty body, got nil")
	}
}

func TestSEOKeyParser_MapCompetitionValue(t *testing.T) {
	parser := NewSEOKeyParser()
	
	tests := []struct {
		input    string
		expected float64
	}{
		{"LOW", 0.3},
		{"HIGH", 0.8},
		{"MEDIUM", 0.5},
		{"unknown", 0.5},
		{"", 0.5},
	}
	
	for _, test := range tests {
		result := parser.mapCompetitionValue(test.input)
		if result != test.expected {
			t.Errorf("For input '%s', expected %f, got %f", test.input, test.expected, result)
		}
	}
}
