package storage

// extractDomainFromURL extracts domain from sitemap URL
func extractDomainFromURL(sitemapURL string) string {
	// Simple domain extraction
	if len(sitemapURL) > 8 {
		if sitemapURL[:8] == "https://" {
			sitemapURL = sitemapURL[8:]
		} else if sitemapURL[:7] == "http://" {
			sitemapURL = sitemapURL[7:]
		}
	}

	// Find first slash
	if idx := len(sitemapURL); idx > 0 {
		for i, ch := range sitemapURL {
			if ch == '/' {
				idx = i
				break
			}
		}
		return sitemapURL[:idx]
	}

	return "unknown"
}