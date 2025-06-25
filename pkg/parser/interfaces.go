package parser

import (
	"context"
	"net/url"
)

type URL struct {
	ID          string            `json:"id"`
	Address     string            `json:"address"`
	Keywords    []string          `json:"keywords"`
	LastUpdated string            `json:"last_updated"`
	Metadata    map[string]string `json:"metadata"`
}

type SitemapParser interface {
	Parse(ctx context.Context, url string) ([]URL, error)
	SupportedFormats() []string
	Validate(url string) error
}

type ParserFactory interface {
	GetParser(format string) SitemapParser
	RegisterParser(format string, parser SitemapParser)
}

type Filter interface {
	ShouldExclude(url *url.URL) bool
	Name() string
}