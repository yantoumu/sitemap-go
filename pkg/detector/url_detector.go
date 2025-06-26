package detector

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/parser"
	"sitemap-go/pkg/storage"
)

// URLChangeDetector implements ChangeDetector for URL changes
type URLChangeDetector struct {
	storage storage.Storage
	log     *logger.Logger
}

// NewURLChangeDetector creates a new URL change detector
func NewURLChangeDetector(storage storage.Storage) *URLChangeDetector {
	return &URLChangeDetector{
		storage: storage,
		log:     logger.GetLogger().WithField("component", "url_change_detector"),
	}
}

// DetectChanges compares old and new URL sets and returns detected changes
func (d *URLChangeDetector) DetectChanges(ctx context.Context, oldURLs, newURLs []parser.URL) (*ChangeSet, error) {
	d.log.WithFields(map[string]interface{}{
		"old_count": len(oldURLs),
		"new_count": len(newURLs),
	}).Debug("Starting change detection")

	// Create maps for efficient lookup
	oldURLMap := make(map[string]parser.URL)
	newURLMap := make(map[string]parser.URL)
	
	for _, url := range oldURLs {
		oldURLMap[url.Address] = url
	}
	
	for _, url := range newURLs {
		newURLMap[url.Address] = url
	}

	var changes []URLChange
	timestamp := time.Now()

	// Detect added URLs
	for address, url := range newURLMap {
		if _, exists := oldURLMap[address]; !exists {
			changes = append(changes, URLChange{
				URL:       url,
				Type:      ChangeTypeAdded,
				Timestamp: timestamp,
				Metadata: map[string]interface{}{
					"detection_method": "set_comparison",
				},
			})
		}
	}

	// Detect removed URLs
	for address, url := range oldURLMap {
		if _, exists := newURLMap[address]; !exists {
			changes = append(changes, URLChange{
				URL:       url,
				Type:      ChangeTypeRemoved,
				Timestamp: timestamp,
				Metadata: map[string]interface{}{
					"detection_method": "set_comparison",
				},
			})
		}
	}

	// Detect modified URLs (same address but different content)
	for address, newURL := range newURLMap {
		if oldURL, exists := oldURLMap[address]; exists {
			if d.hasURLChanged(oldURL, newURL) {
				changes = append(changes, URLChange{
					URL:       newURL,
					Type:      ChangeTypeModified,
					Timestamp: timestamp,
					Metadata: map[string]interface{}{
						"detection_method": "content_comparison",
						"changes":          d.getURLDifferences(oldURL, newURL),
					},
				})
			}
		}
	}

	// Create change set
	changeSet := &ChangeSet{
		Domain:    d.extractDomain(oldURLs, newURLs),
		Changes:   changes,
		Timestamp: timestamp,
	}

	// Count changes by type
	d.categorizeChanges(changeSet)

	d.log.WithFields(map[string]interface{}{
		"domain":         changeSet.Domain,
		"total_changes":  len(changes),
		"added":          changeSet.TotalAdded,
		"removed":        changeSet.TotalRemoved,
		"modified":       changeSet.TotalModified,
	}).Info("Change detection completed")

	return changeSet, nil
}

// GetChangeHistory retrieves change history for a domain
func (d *URLChangeDetector) GetChangeHistory(ctx context.Context, domain string, limit int) ([]*ChangeSet, error) {
	key := fmt.Sprintf("changes:%s", domain)
	
	var history []*ChangeSet
	err := d.storage.Load(ctx, key, &history)
	if err != nil {
		// If no history exists, return empty slice
		return []*ChangeSet{}, nil
	}

	// Sort by timestamp (newest first) and limit results
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.After(history[j].Timestamp)
	})

	if limit > 0 && len(history) > limit {
		history = history[:limit]
	}

	return history, nil
}

// hasURLChanged checks if a URL has been modified
func (d *URLChangeDetector) hasURLChanged(oldURL, newURL parser.URL) bool {
	// Compare keywords
	if !d.equalStringSlices(oldURL.Keywords, newURL.Keywords) {
		return true
	}

	// Compare metadata
	if !d.equalMetadata(oldURL.Metadata, newURL.Metadata) {
		return true
	}

	return false
}

// getURLDifferences returns a description of what changed
func (d *URLChangeDetector) getURLDifferences(oldURL, newURL parser.URL) map[string]interface{} {
	differences := make(map[string]interface{})

	// Check keyword changes
	if !d.equalStringSlices(oldURL.Keywords, newURL.Keywords) {
		differences["keywords"] = map[string]interface{}{
			"old": oldURL.Keywords,
			"new": newURL.Keywords,
		}
	}

	// Check metadata changes
	if !d.equalMetadata(oldURL.Metadata, newURL.Metadata) {
		differences["metadata"] = map[string]interface{}{
			"old": oldURL.Metadata,
			"new": newURL.Metadata,
		}
	}

	return differences
}

// extractDomain extracts domain from URL sets
func (d *URLChangeDetector) extractDomain(oldURLs, newURLs []parser.URL) string {
	// Try to extract domain from new URLs first
	if len(newURLs) > 0 {
		return d.getDomainFromURL(newURLs[0].Address)
	}
	
	// Fallback to old URLs
	if len(oldURLs) > 0 {
		return d.getDomainFromURL(oldURLs[0].Address)
	}
	
	return "unknown"
}

// getDomainFromURL extracts domain from URL string
func (d *URLChangeDetector) getDomainFromURL(urlStr string) string {
	if strings.HasPrefix(urlStr, "http://") {
		urlStr = strings.TrimPrefix(urlStr, "http://")
	} else if strings.HasPrefix(urlStr, "https://") {
		urlStr = strings.TrimPrefix(urlStr, "https://")
	}
	
	parts := strings.Split(urlStr, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	
	return "unknown"
}

// categorizeChanges counts changes by type
func (d *URLChangeDetector) categorizeChanges(changeSet *ChangeSet) {
	for _, change := range changeSet.Changes {
		switch change.Type {
		case ChangeTypeAdded:
			changeSet.TotalAdded++
		case ChangeTypeRemoved:
			changeSet.TotalRemoved++
		case ChangeTypeModified:
			changeSet.TotalModified++
		}
	}
}

// equalStringSlices compares two string slices
func (d *URLChangeDetector) equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	
	// Sort both slices for comparison
	sortedA := make([]string, len(a))
	sortedB := make([]string, len(b))
	copy(sortedA, a)
	copy(sortedB, b)
	sort.Strings(sortedA)
	sort.Strings(sortedB)
	
	for i := range sortedA {
		if sortedA[i] != sortedB[i] {
			return false
		}
	}
	
	return true
}

// equalMetadata compares two metadata maps
func (d *URLChangeDetector) equalMetadata(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	
	for key, valueA := range a {
		if valueB, exists := b[key]; !exists || valueA != valueB {
			return false
		}
	}
	
	return true
}