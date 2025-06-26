package extractor

import (
	"net/url"
	"regexp"
	"strings"

	"sitemap-go/pkg/logger"
)

var (
	// Common stop words to filter out
	stopWords = map[string]bool{
		"the": true, "and": true, "or": true, "in": true, "on": true,
		"at": true, "to": true, "for": true, "of": true, "with": true,
		"by": true, "from": true, "a": true, "an": true, "as": true,
		"is": true, "was": true, "are": true, "were": true, "been": true,
		// Gaming-specific stop words
		"game": true, "games": true, "play": true, "online": true, "free": true,
		"html5": true, "flash": true, "web": true, "browser": true, "mobile": true,
		"app": true, "download": true, "install": true, "full": true, "version": true,
	}
	
	// Gaming keywords that should be preserved
	gameKeywords = map[string]bool{
		"action": true, "adventure": true, "arcade": true, "puzzle": true, "racing": true,
		"strategy": true, "shooter": true, "platform": true, "rpg": true, "simulation": true,
		"sports": true, "fighting": true, "horror": true, "multiplayer": true, "singleplayer": true,
		"3d": true, "2d": true, "retro": true, "classic": true, "indie": true,
		"mario": true, "sonic": true, "minecraft": true, "pokemon": true, "zelda": true,
		"geometry": true, "dash": true, "wordle": true, "tetris": true, "snake": true,
		"pacman": true, "chess": true, "solitaire": true, "mahjong": true, "sudoku": true,
	}
	
	// Regex patterns for cleaning
	nonAlphaNumeric = regexp.MustCompile(`[^a-zA-Z0-9\s\-_]`)
	multiSpace      = regexp.MustCompile(`\s+`)
	multiDash       = regexp.MustCompile(`-+`)
)

type URLKeywordExtractor struct {
	filters        []Filter
	minWordLength  int
	maxWordLength  int
	log            *logger.Logger
}

func NewURLKeywordExtractor() *URLKeywordExtractor {
	return &URLKeywordExtractor{
		filters:       make([]Filter, 0),
		minWordLength: 3,
		maxWordLength: 50,
		log:           logger.GetLogger().WithField("component", "keyword_extractor"),
	}
}

func (e *URLKeywordExtractor) Extract(urlStr string) ([]string, error) {
	e.log.Debug("Extracting keywords from URL")
	
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		e.log.WithError(err).Error("Failed to parse URL")
		return nil, err
	}

	// Extract keywords from path
	pathKeywords := e.extractFromPath(parsedURL.Path)
	
	// Extract keywords from query parameters
	queryKeywords := e.extractFromQuery(parsedURL.Query())
	
	// Combine and deduplicate
	keywordMap := make(map[string]bool)
	for _, kw := range pathKeywords {
		keywordMap[kw] = true
	}
	for _, kw := range queryKeywords {
		keywordMap[kw] = true
	}
	
	// Convert map to slice
	keywords := make([]string, 0, len(keywordMap))
	for kw := range keywordMap {
		keywords = append(keywords, kw)
	}
	
	// Apply filters
	for _, filter := range e.filters {
		keywords = filter.Apply(keywords)
	}
	
	e.log.WithFields(map[string]interface{}{
		"url":           urlStr,
		"keywords_count": len(keywords),
	}).Debug("Keywords extracted successfully")
	
	return keywords, nil
}

func (e *URLKeywordExtractor) SetFilters(filters []Filter) {
	e.filters = filters
}

func (e *URLKeywordExtractor) Normalize(keyword string) string {
	// Convert to lowercase
	normalized := strings.ToLower(keyword)
	
	// Remove non-alphanumeric characters (except spaces and dashes)
	normalized = nonAlphaNumeric.ReplaceAllString(normalized, " ")
	
	// Replace multiple spaces with single space
	normalized = multiSpace.ReplaceAllString(normalized, " ")
	
	// Replace multiple dashes with single dash
	normalized = multiDash.ReplaceAllString(normalized, "-")
	
	// Trim spaces and dashes
	normalized = strings.Trim(normalized, " -")
	
	return normalized
}

func (e *URLKeywordExtractor) extractFromPath(path string) []string {
	// Remove file extension
	if idx := strings.LastIndex(path, "."); idx > 0 {
		path = path[:idx]
	}
	
	// Split by path separators first
	pathParts := strings.Split(path, "/")
	keywords := make([]string, 0)
	
	// Process each path segment
	for _, segment := range pathParts {
		if segment == "" {
			continue
		}
		
		// Check if this segment looks like a game name with hyphens
		if e.isHyphenatedGameName(segment) {
			// Convert hyphens to spaces and clean up
			gameName := e.convertHyphensToSpaces(segment)
			if gameName != "" {
				keywords = append(keywords, gameName)
			}
		} else {
			// Process as individual words (original logic)
			parts := strings.FieldsFunc(segment, func(r rune) bool {
				return r == '-' || r == '_' || r == '.'
			})
			
			for _, part := range parts {
				// Normalize the part
				normalized := e.Normalize(part)
				
				// Skip empty, too short, or too long
				if normalized == "" || 
				   len(normalized) < e.minWordLength || 
				   len(normalized) > e.maxWordLength {
					continue
				}
				
				// Check if it's a game keyword (preserve these even if they're stop words)
				if gameKeywords[normalized] {
					keywords = append(keywords, normalized)
					continue
				}
				
				// Skip regular stop words
				if stopWords[normalized] {
					continue
				}
				
				// Handle numbers specially for games (like "2048", "3d", etc.)
				if e.isGameNumber(normalized) {
					keywords = append(keywords, normalized)
					continue
				}
				
				// Split camelCase and PascalCase
				subWords := e.splitCamelCase(normalized)
				for _, word := range subWords {
					if len(word) >= e.minWordLength {
						if gameKeywords[word] || (!stopWords[word] && !e.isCommonWord(word)) {
							keywords = append(keywords, word)
						}
					}
				}
			}
		}
	}
	
	return keywords
}

func (e *URLKeywordExtractor) extractFromQuery(params url.Values) []string {
	keywords := make([]string, 0)
	
	// Common parameter names that might contain keywords
	keywordParams := []string{"q", "query", "search", "keyword", "tag", "category", "title"}
	
	for _, param := range keywordParams {
		if values, exists := params[param]; exists {
			for _, value := range values {
				// Split by common separators
				words := strings.FieldsFunc(value, func(r rune) bool {
					return r == ' ' || r == ',' || r == '+' || r == '|'
				})
				
				for _, word := range words {
					normalized := e.Normalize(word)
					if normalized != "" && 
					   len(normalized) >= e.minWordLength && 
					   len(normalized) <= e.maxWordLength &&
					   !stopWords[normalized] {
						keywords = append(keywords, normalized)
					}
				}
			}
		}
	}
	
	return keywords
}

func (e *URLKeywordExtractor) splitCamelCase(word string) []string {
	var result []string
	var current strings.Builder
	
	for i, r := range word {
		if i > 0 && 'A' <= r && r <= 'Z' {
			if current.Len() > 0 {
				result = append(result, strings.ToLower(current.String()))
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	
	if current.Len() > 0 {
		result = append(result, strings.ToLower(current.String()))
	}
	
	return result
}

// isGameNumber checks if a string is a gaming-relevant number
func (e *URLKeywordExtractor) isGameNumber(s string) bool {
	gameNumbers := map[string]bool{
		"2d": true, "3d": true, "2048": true, "1001": true, "24": true,
		"10": true, "100": true, "1000": true, "2": true, "3": true,
	}
	return gameNumbers[s]
}

// isCommonWord checks if a word is too common to be useful as a keyword
func (e *URLKeywordExtractor) isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"com": true, "www": true, "http": true, "https": true, "html": true,
		"page": true, "site": true, "home": true, "index": true, "main": true,
		"new": true, "old": true, "big": true, "small": true, "good": true,
		"bad": true, "best": true, "top": true, "hot": true, "cool": true,
	}
	return commonWords[word]
}

// extractCompoundGameNames identifies compound game names from URL parts
func (e *URLKeywordExtractor) extractCompoundGameNames(parts []string) []string {
	if len(parts) < 2 {
		return []string{}
	}
	
	compounds := []string{}
	
	// Check for common game name patterns
	for i := 0; i < len(parts)-1; i++ {
		part1 := e.Normalize(parts[i])
		part2 := e.Normalize(parts[i+1])
		
		// Skip if either part is too short or a stop word
		if len(part1) < 2 || len(part2) < 2 || 
		   stopWords[part1] || stopWords[part2] {
			continue
		}
		
		// Create compound if it looks like a game name
		compound := part1 + "-" + part2
		if e.looksLikeGameName(part1, part2) {
			compounds = append(compounds, compound)
		}
		
		// Check for three-word compounds (like "super-mario-bros")
		if i < len(parts)-2 {
			part3 := e.Normalize(parts[i+2])
			if len(part3) >= 2 && !stopWords[part3] {
				threeCompound := part1 + "-" + part2 + "-" + part3
				if e.looksLikeGameName(part1, part2) || e.looksLikeGameName(part2, part3) {
					compounds = append(compounds, threeCompound)
				}
			}
		}
	}
	
	return compounds
}

// looksLikeGameName checks if two words together form a likely game name
func (e *URLKeywordExtractor) looksLikeGameName(word1, word2 string) bool {
	// Check if either word is a known game keyword
	if gameKeywords[word1] || gameKeywords[word2] {
		return true
	}
	
	// Check for common game name patterns
	gamePatterns := map[string]bool{
		"super": true, "mega": true, "ultra": true, "mini": true, "micro": true,
		"big": true, "little": true, "tiny": true, "giant": true, "crazy": true,
		"wild": true, "mad": true, "epic": true, "legend": true, "hero": true,
		"world": true, "land": true, "city": true, "wars": true, "quest": true,
		"adventure": true, "rush": true, "run": true, "jump": true, "dash": true,
	}
	
	return gamePatterns[word1] || gamePatterns[word2]
}

// isHyphenatedGameName checks if a segment looks like a hyphenated game name
func (e *URLKeywordExtractor) isHyphenatedGameName(segment string) bool {
	// Must contain hyphens
	if !strings.Contains(segment, "-") {
		return false
	}
	
	// Split by hyphens and check if it looks like a game name
	parts := strings.Split(segment, "-")
	
	// Must have at least 2 parts
	if len(parts) < 2 {
		return false
	}
	
	// Check if this might be a game name by looking at patterns
	// Game names typically have meaningful words, not random single characters
	meaningfulParts := 0
	for _, part := range parts {
		// Skip empty parts
		if part == "" {
			continue
		}
		
		// Check if part looks meaningful (more than 1 char, or is a number)
		if len(part) > 1 || e.isGameNumber(part) {
			meaningfulParts++
		}
	}
	
	// Consider it a game name if most parts are meaningful
	return meaningfulParts >= 2
}

// convertHyphensToSpaces converts hyphened game names to space-separated keywords
func (e *URLKeywordExtractor) convertHyphensToSpaces(segment string) string {
	// Replace hyphens with spaces
	spaced := strings.ReplaceAll(segment, "-", " ")
	
	// Clean up multiple spaces
	spaced = strings.TrimSpace(spaced)
	spaced = multiSpace.ReplaceAllString(spaced, " ")
	
	// Additional cleanup - remove any non-alphanumeric except spaces and numbers
	cleaned := nonAlphaNumeric.ReplaceAllStringFunc(spaced, func(s string) string {
		if s == " " {
			return s
		}
		return ""
	})
	
	// Final cleanup
	cleaned = strings.TrimSpace(cleaned)
	
	// Only return if it's meaningful (more than just single characters)
	if len(cleaned) > 2 {
		return cleaned
	}
	
	return ""
}