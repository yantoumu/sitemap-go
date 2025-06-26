package detector

import (
	"context"
	"time"

	"sitemap-go/pkg/parser"
)

// ChangeType represents the type of change detected
type ChangeType string

const (
	ChangeTypeAdded    ChangeType = "added"
	ChangeTypeRemoved  ChangeType = "removed"
	ChangeTypeModified ChangeType = "modified"
)

// URLChange represents a detected change in URLs
type URLChange struct {
	URL        parser.URL    `json:"url"`
	Type       ChangeType    `json:"type"`
	Timestamp  time.Time     `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ChangeSet represents a collection of changes for a domain
type ChangeSet struct {
	Domain     string       `json:"domain"`
	Changes    []URLChange  `json:"changes"`
	Timestamp  time.Time    `json:"timestamp"`
	TotalAdded int          `json:"total_added"`
	TotalRemoved int        `json:"total_removed"`
	TotalModified int       `json:"total_modified"`
}

// ChangeDetector interface for URL change detection
type ChangeDetector interface {
	// DetectChanges compares old and new URL sets and returns detected changes
	DetectChanges(ctx context.Context, oldURLs, newURLs []parser.URL) (*ChangeSet, error)
	
	// GetChangeHistory retrieves change history for a domain
	GetChangeHistory(ctx context.Context, domain string, limit int) ([]*ChangeSet, error)
}

// HistoryManager interface for managing historical data
type HistoryManager interface {
	// SaveSnapshot saves a URL snapshot for a domain
	SaveSnapshot(ctx context.Context, domain string, urls []parser.URL) error
	
	// GetLatestSnapshot retrieves the most recent snapshot for a domain
	GetLatestSnapshot(ctx context.Context, domain string) ([]parser.URL, error)
	
	// GetSnapshotHistory retrieves snapshot history
	GetSnapshotHistory(ctx context.Context, domain string, limit int) ([]SnapshotMetadata, error)
}

// SnapshotMetadata contains metadata about a saved snapshot
type SnapshotMetadata struct {
	Domain    string    `json:"domain"`
	Timestamp time.Time `json:"timestamp"`
	URLCount  int       `json:"url_count"`
	Checksum  string    `json:"checksum"`
}