package extractor

type Keyword struct {
	Word         string  `json:"word"`
	SearchVolume int     `json:"search_volume"`
	Competition  float64 `json:"competition"`
	CPC          float64 `json:"cpc"`
	UpdatedAt    string  `json:"updated_at"`
}

type KeywordExtractor interface {
	Extract(url string) ([]string, error)
	SetFilters(filters []Filter)
	Normalize(keyword string) string
}

type Filter interface {
	Apply(keywords []string) []string
	Name() string
}