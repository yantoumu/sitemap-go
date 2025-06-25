package parser

import (
	"fmt"
	"sync"
)

type parserFactory struct {
	parsers map[string]SitemapParser
	mu      sync.RWMutex
}

var (
	factory     *parserFactory
	factoryOnce sync.Once
)

// GetParserFactory returns the singleton parser factory instance
func GetParserFactory() ParserFactory {
	factoryOnce.Do(func() {
		factory = &parserFactory{
			parsers: make(map[string]SitemapParser),
		}
		// Register default parsers
		factory.RegisterParser("xml", NewXMLParser())
		factory.RegisterParser("xml.gz", NewXMLParser())
		factory.RegisterParser("rss", NewRSSParser())
		factory.RegisterParser("feed", NewRSSParser())
		factory.RegisterParser("txt", NewTXTParser())
		factory.RegisterParser("text", NewTXTParser())
	})
	return factory
}

func (f *parserFactory) GetParser(format string) SitemapParser {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	parser, exists := f.parsers[format]
	if !exists {
		return nil
	}
	return parser
}

func (f *parserFactory) RegisterParser(format string, parser SitemapParser) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	if parser == nil {
		panic(fmt.Sprintf("cannot register nil parser for format %s", format))
	}
	
	f.parsers[format] = parser
}