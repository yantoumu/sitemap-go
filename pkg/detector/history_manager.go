package detector

import (
	"context"
	"crypto/md5"
	"fmt"
	"sort"
	"time"

	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/parser"
	"sitemap-go/pkg/storage"
)

// URLHistoryManager implements HistoryManager for URL snapshots
type URLHistoryManager struct {
	storage storage.Storage
	log     *logger.Logger
}

// NewURLHistoryManager creates a new URL history manager
func NewURLHistoryManager(storage storage.Storage) *URLHistoryManager {
	return &URLHistoryManager{
		storage: storage,
		log:     logger.GetLogger().WithField("component", "url_history_manager"),
	}
}

// SaveSnapshot saves a URL snapshot for a domain
func (h *URLHistoryManager) SaveSnapshot(ctx context.Context, domain string, urls []parser.URL) error {
	timestamp := time.Now()
	checksum := h.calculateChecksum(urls)
	
	// Create snapshot metadata
	metadata := SnapshotMetadata{
		Domain:    domain,
		Timestamp: timestamp,
		URLCount:  len(urls),
		Checksum:  checksum,
	}

	// Save snapshot data
	snapshotKey := fmt.Sprintf("snapshot:%s:%d", domain, timestamp.Unix())
	if err := h.storage.Save(ctx, snapshotKey, urls); err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	// Save metadata
	metadataKey := fmt.Sprintf("snapshot_meta:%s:%d", domain, timestamp.Unix())
	if err := h.storage.Save(ctx, metadataKey, metadata); err != nil {
		return fmt.Errorf("failed to save snapshot metadata: %w", err)
	}

	// Update latest snapshot reference
	latestKey := fmt.Sprintf("latest_snapshot:%s", domain)
	if err := h.storage.Save(ctx, latestKey, snapshotKey); err != nil {
		return fmt.Errorf("failed to update latest snapshot reference: %w", err)
	}

	// Update snapshot history index
	if err := h.updateSnapshotIndex(ctx, domain, metadata); err != nil {
		h.log.WithError(err).WithField("domain", domain).Warn("Failed to update snapshot index")
	}

	h.log.WithFields(map[string]interface{}{
		"domain":    domain,
		"url_count": len(urls),
		"checksum":  checksum,
	}).Info("Snapshot saved successfully")

	return nil
}

// GetLatestSnapshot retrieves the most recent snapshot for a domain
func (h *URLHistoryManager) GetLatestSnapshot(ctx context.Context, domain string) ([]parser.URL, error) {
	latestKey := fmt.Sprintf("latest_snapshot:%s", domain)
	
	var snapshotKey string
	if err := h.storage.Load(ctx, latestKey, &snapshotKey); err != nil {
		return nil, fmt.Errorf("no latest snapshot found for domain %s: %w", domain, err)
	}

	var urls []parser.URL
	if err := h.storage.Load(ctx, snapshotKey, &urls); err != nil {
		return nil, fmt.Errorf("failed to load snapshot data: %w", err)
	}

	h.log.WithFields(map[string]interface{}{
		"domain":    domain,
		"url_count": len(urls),
	}).Debug("Latest snapshot retrieved")

	return urls, nil
}

// GetSnapshotHistory retrieves snapshot history for a domain
func (h *URLHistoryManager) GetSnapshotHistory(ctx context.Context, domain string, limit int) ([]SnapshotMetadata, error) {
	indexKey := fmt.Sprintf("snapshot_index:%s", domain)
	
	var history []SnapshotMetadata
	if err := h.storage.Load(ctx, indexKey, &history); err != nil {
		// If no history exists, return empty slice
		return []SnapshotMetadata{}, nil
	}

	// Sort by timestamp (newest first)
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.After(history[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && len(history) > limit {
		history = history[:limit]
	}

	return history, nil
}

// updateSnapshotIndex updates the snapshot index for a domain
func (h *URLHistoryManager) updateSnapshotIndex(ctx context.Context, domain string, metadata SnapshotMetadata) error {
	indexKey := fmt.Sprintf("snapshot_index:%s", domain)
	
	var history []SnapshotMetadata
	
	// Load existing history (ignore error if doesn't exist)
	_ = h.storage.Load(ctx, indexKey, &history)
	
	// Add new metadata
	history = append(history, metadata)
	
	// Keep only recent snapshots (limit to 100 entries)
	const maxHistorySize = 100
	if len(history) > maxHistorySize {
		sort.Slice(history, func(i, j int) bool {
			return history[i].Timestamp.After(history[j].Timestamp)
		})
		history = history[:maxHistorySize]
	}
	
	// Save updated history
	return h.storage.Save(ctx, indexKey, history)
}

// calculateChecksum generates a checksum for URL set
func (h *URLHistoryManager) calculateChecksum(urls []parser.URL) string {
	// Create a deterministic string representation
	var urlStrings []string
	for _, url := range urls {
		urlStrings = append(urlStrings, url.Address)
	}
	
	// Sort for consistent checksum
	sort.Strings(urlStrings)
	
	// Calculate MD5 hash
	data := fmt.Sprintf("%v", urlStrings)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// GetSnapshotByTime retrieves a snapshot closest to the specified time
func (h *URLHistoryManager) GetSnapshotByTime(ctx context.Context, domain string, targetTime time.Time) ([]parser.URL, error) {
	history, err := h.GetSnapshotHistory(ctx, domain, 0) // Get all history
	if err != nil {
		return nil, err
	}

	if len(history) == 0 {
		return nil, fmt.Errorf("no snapshots found for domain %s", domain)
	}

	// Find closest snapshot
	var closest SnapshotMetadata
	minDiff := time.Duration(1<<63 - 1) // Max duration

	for _, snapshot := range history {
		diff := targetTime.Sub(snapshot.Timestamp)
		if diff < 0 {
			diff = -diff
		}
		
		if diff < minDiff {
			minDiff = diff
			closest = snapshot
		}
	}

	// Load the closest snapshot
	snapshotKey := fmt.Sprintf("snapshot:%s:%d", domain, closest.Timestamp.Unix())
	var urls []parser.URL
	if err := h.storage.Load(ctx, snapshotKey, &urls); err != nil {
		return nil, fmt.Errorf("failed to load snapshot: %w", err)
	}

	return urls, nil
}