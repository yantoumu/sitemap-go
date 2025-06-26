package storage

import (
	"context"
	"fmt"
	"sort"
	"time"

	"sitemap-go/pkg/logger"
)

// MonitorResult represents the result of monitoring a sitemap
type MonitorResult struct {
	SitemapURL string                 `json:"sitemap_url"`
	Keywords   []string               `json:"keywords"`
	TrendData  interface{}            `json:"trend_data,omitempty"` // Use interface{} to avoid cycle
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// MonitoringSession represents a monitoring session
type MonitoringSession struct {
	ID           string                    `json:"id"`
	StartTime    time.Time                 `json:"start_time"`
	EndTime      time.Time                 `json:"end_time"`
	TotalSites   int                       `json:"total_sites"`
	SuccessCount int                       `json:"success_count"`
	FailureCount int                       `json:"failure_count"`
	Results      []*MonitorResult         `json:"results"`
	Status       string                    `json:"status"` // "running", "completed", "failed"
}

// ResultSummary provides aggregated results
type ResultSummary struct {
	TotalSessions  int                       `json:"total_sessions"`
	LastSession    *MonitoringSession        `json:"last_session,omitempty"`
	TotalSites     int                       `json:"total_sites"`
	TotalURLs      int                       `json:"total_urls"`
	TotalKeywords  int                       `json:"total_keywords"`
	SuccessRate    float64                   `json:"success_rate"`
	LastUpdated    time.Time                 `json:"last_updated"`
	TopSites       []SiteResultSummary       `json:"top_sites"`
}

// SiteResultSummary represents individual site summary
type SiteResultSummary struct {
	SitemapURL   string    `json:"sitemap_url"`
	Domain       string    `json:"domain"`
	URLCount     int       `json:"url_count"`
	KeywordCount int       `json:"keyword_count"`
	LastSuccess  time.Time `json:"last_success"`
	SuccessRate  float64   `json:"success_rate"`
}

// ResultManager manages monitoring results and sessions
type ResultManager struct {
	storage Storage
	log     *logger.Logger
}

// NewResultManager creates a new result manager
func NewResultManager(storage Storage) *ResultManager {
	return &ResultManager{
		storage: storage,
		log:     logger.GetLogger().WithField("component", "result_manager"),
	}
}

// SaveMonitoringSession saves a complete monitoring session
func (rm *ResultManager) SaveMonitoringSession(ctx context.Context, session *MonitoringSession) error {
	// Save individual session
	sessionKey := fmt.Sprintf("session:%s", session.ID)
	if err := rm.storage.Save(ctx, sessionKey, session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Update session index
	if err := rm.updateSessionIndex(ctx, session); err != nil {
		rm.log.WithError(err).Warn("Failed to update session index")
	}

	// Update result summary
	if err := rm.updateResultSummary(ctx, session); err != nil {
		rm.log.WithError(err).Warn("Failed to update result summary")
	}

	rm.log.WithFields(map[string]interface{}{
		"session_id":     session.ID,
		"total_sites":    session.TotalSites,
		"success_count":  session.SuccessCount,
		"failure_count":  session.FailureCount,
	}).Info("Monitoring session saved")

	return nil
}

// GetLatestSession retrieves the most recent monitoring session
func (rm *ResultManager) GetLatestSession(ctx context.Context) (*MonitoringSession, error) {
	summary, err := rm.GetResultSummary(ctx)
	if err != nil {
		return nil, err
	}

	if summary.LastSession == nil {
		return nil, fmt.Errorf("no sessions found")
	}

	return summary.LastSession, nil
}

// GetSessionHistory retrieves session history
func (rm *ResultManager) GetSessionHistory(ctx context.Context, limit int) ([]*MonitoringSession, error) {
	indexKey := "session_index"

	var sessionIDs []string
	if err := rm.storage.Load(ctx, indexKey, &sessionIDs); err != nil {
		return []*MonitoringSession{}, nil // Return empty if no history
	}

	// Apply limit
	if limit > 0 && len(sessionIDs) > limit {
		sessionIDs = sessionIDs[:limit]
	}

	// Load sessions
	sessions := make([]*MonitoringSession, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		sessionKey := fmt.Sprintf("session:%s", id)
		var session MonitoringSession
		if err := rm.storage.Load(ctx, sessionKey, &session); err != nil {
			rm.log.WithError(err).WithField("session_id", id).Warn("Failed to load session")
			continue
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// GetResultSummary retrieves aggregated results summary
func (rm *ResultManager) GetResultSummary(ctx context.Context) (*ResultSummary, error) {
	summaryKey := "result_summary"

	var summary ResultSummary
	if err := rm.storage.Load(ctx, summaryKey, &summary); err != nil {
		// Return default summary if none exists
		return &ResultSummary{
			TotalSessions: 0,
			TotalSites:    0,
			TotalURLs:     0,
			TotalKeywords: 0,
			SuccessRate:   0.0,
			LastUpdated:   time.Now(),
			TopSites:      []SiteResultSummary{},
		}, nil
	}

	return &summary, nil
}

// GetSiteResults retrieves results for a specific sitemap
func (rm *ResultManager) GetSiteResults(ctx context.Context, sitemapURL string, limit int) ([]*MonitorResult, error) {
	sessions, err := rm.GetSessionHistory(ctx, 0) // Get all sessions
	if err != nil {
		return nil, err
	}

	var siteResults []*MonitorResult
	
	// Collect results for the specific sitemap
	for _, session := range sessions {
		for _, result := range session.Results {
			if result.SitemapURL == sitemapURL {
				siteResults = append(siteResults, result)
			}
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(siteResults, func(i, j int) bool {
		return siteResults[i].Timestamp.After(siteResults[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && len(siteResults) > limit {
		siteResults = siteResults[:limit]
	}

	return siteResults, nil
}

// updateSessionIndex maintains a list of session IDs
func (rm *ResultManager) updateSessionIndex(ctx context.Context, session *MonitoringSession) error {
	indexKey := "session_index"

	var sessionIDs []string
	_ = rm.storage.Load(ctx, indexKey, &sessionIDs) // Ignore error if doesn't exist

	// Add new session ID to the beginning
	sessionIDs = append([]string{session.ID}, sessionIDs...)

	// Keep only recent sessions (limit to 50)
	const maxSessions = 50
	if len(sessionIDs) > maxSessions {
		sessionIDs = sessionIDs[:maxSessions]
	}

	return rm.storage.Save(ctx, indexKey, sessionIDs)
}

// updateResultSummary updates the aggregated results summary
func (rm *ResultManager) updateResultSummary(ctx context.Context, session *MonitoringSession) error {
	summaryKey := "result_summary"

	// Get existing summary
	summary, _ := rm.GetResultSummary(ctx)

	// Update totals
	summary.TotalSessions++
	summary.LastSession = session
	summary.TotalSites = session.TotalSites
	summary.LastUpdated = time.Now()

	// Calculate totals from last session
	totalURLs := 0
	totalKeywords := 0
	successCount := 0

	for _, result := range session.Results {
		if result.Success {
			successCount++
			totalKeywords += len(result.Keywords)
		}
		// Estimate URLs (this could be improved with actual URL count)
		totalURLs += len(result.Keywords) // Rough estimate
	}

	summary.TotalURLs = totalURLs
	summary.TotalKeywords = totalKeywords
	
	if session.TotalSites > 0 {
		summary.SuccessRate = float64(successCount) / float64(session.TotalSites) * 100
	}

	// Update top sites summary
	summary.TopSites = rm.buildTopSites(session)

	return rm.storage.Save(ctx, summaryKey, summary)
}

// buildTopSites creates top sites summary from session results
func (rm *ResultManager) buildTopSites(session *MonitoringSession) []SiteResultSummary {
	siteMap := make(map[string]SiteResultSummary)

	for _, result := range session.Results {
		domain := extractDomainFromURL(result.SitemapURL)
		
		site, exists := siteMap[domain]
		if !exists {
			site = SiteResultSummary{
				SitemapURL: result.SitemapURL,
				Domain:     domain,
			}
		}

		if result.Success {
			site.KeywordCount += len(result.Keywords)
			site.URLCount += len(result.Keywords) // Rough estimate
			site.LastSuccess = result.Timestamp
			site.SuccessRate = 100.0 // Single session success
		}

		siteMap[domain] = site
	}

	// Convert to slice and sort by keyword count
	sites := make([]SiteResultSummary, 0, len(siteMap))
	for _, site := range siteMap {
		sites = append(sites, site)
	}

	sort.Slice(sites, func(i, j int) bool {
		return sites[i].KeywordCount > sites[j].KeywordCount
	})

	// Return top 10 sites
	if len(sites) > 10 {
		sites = sites[:10]
	}

	return sites
}

