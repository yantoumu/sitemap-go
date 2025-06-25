package storage

import (
	"container/list"
	"sync"
	"time"
)

// cacheItem represents an item in the cache
type cacheItem struct {
	key       string
	value     interface{}
	timestamp time.Time
	element   *list.Element
}

// MemoryCache implements an LRU cache with TTL support
type MemoryCache struct {
	maxSize   int
	items     map[string]*cacheItem
	lruList   *list.List
	mu        sync.RWMutex
	ttl       time.Duration
}

// NewMemoryCache creates a new in-memory cache with specified size
func NewMemoryCache(maxSize int) *MemoryCache {
	return NewMemoryCacheWithTTL(maxSize, 0) // No TTL by default
}

// NewMemoryCacheWithTTL creates a new in-memory cache with TTL
func NewMemoryCacheWithTTL(maxSize int, ttl time.Duration) *MemoryCache {
	cache := &MemoryCache{
		maxSize: maxSize,
		items:   make(map[string]*cacheItem),
		lruList: list.New(),
		ttl:     ttl,
	}
	
	// Start cleanup routine if TTL is enabled
	if ttl > 0 {
		go cache.cleanupRoutine()
	}
	
	return cache
}

// Set adds or updates an item in the cache
func (mc *MemoryCache) Set(key string, value interface{}) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	now := time.Now()
	
	// Check if item already exists
	if item, exists := mc.items[key]; exists {
		// Update existing item
		item.value = value
		item.timestamp = now
		mc.lruList.MoveToFront(item.element)
		return nil
	}
	
	// Create new item
	item := &cacheItem{
		key:       key,
		value:     value,
		timestamp: now,
	}
	
	// Add to front of LRU list
	element := mc.lruList.PushFront(item)
	item.element = element
	mc.items[key] = item
	
	// Evict oldest items if cache is full
	if len(mc.items) > mc.maxSize {
		mc.evictOldest()
	}
	
	return nil
}

// Get retrieves an item from the cache
func (mc *MemoryCache) Get(key string) (interface{}, bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	item, exists := mc.items[key]
	if !exists {
		return nil, false
	}
	
	// Check TTL if enabled
	if mc.ttl > 0 && time.Since(item.timestamp) > mc.ttl {
		mc.deleteItem(item)
		return nil, false
	}
	
	// Move to front (mark as recently used)
	mc.lruList.MoveToFront(item.element)
	
	return item.value, true
}

// Delete removes an item from the cache
func (mc *MemoryCache) Delete(key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if item, exists := mc.items[key]; exists {
		mc.deleteItem(item)
	}
	
	return nil
}

// Clear removes all items from the cache
func (mc *MemoryCache) Clear() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.items = make(map[string]*cacheItem)
	mc.lruList = list.New()
	
	return nil
}

// Size returns the current number of items in the cache
func (mc *MemoryCache) Size() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.items)
}

// Keys returns all keys in the cache
func (mc *MemoryCache) Keys() []string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	keys := make([]string, 0, len(mc.items))
	for key := range mc.items {
		keys = append(keys, key)
	}
	
	return keys
}

// Stats returns cache statistics
func (mc *MemoryCache) Stats() CacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	return CacheStats{
		Size:    len(mc.items),
		MaxSize: mc.maxSize,
		TTL:     mc.ttl,
	}
}

// evictOldest removes the least recently used item
func (mc *MemoryCache) evictOldest() {
	element := mc.lruList.Back()
	if element != nil {
		item := element.Value.(*cacheItem)
		mc.deleteItem(item)
	}
}

// deleteItem removes an item from both map and list
func (mc *MemoryCache) deleteItem(item *cacheItem) {
	delete(mc.items, item.key)
	mc.lruList.Remove(item.element)
}

// cleanupRoutine periodically removes expired items
func (mc *MemoryCache) cleanupRoutine() {
	ticker := time.NewTicker(mc.ttl / 2) // Cleanup every half TTL
	defer ticker.Stop()
	
	for range ticker.C {
		mc.cleanupExpired()
	}
}

// cleanupExpired removes all expired items
func (mc *MemoryCache) cleanupExpired() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if mc.ttl == 0 {
		return
	}
	
	now := time.Now()
	var expiredItems []*cacheItem
	
	// Find expired items
	for _, item := range mc.items {
		if now.Sub(item.timestamp) > mc.ttl {
			expiredItems = append(expiredItems, item)
		}
	}
	
	// Remove expired items
	for _, item := range expiredItems {
		mc.deleteItem(item)
	}
}

// CacheStats represents cache statistics
type CacheStats struct {
	Size    int           `json:"size"`
	MaxSize int           `json:"max_size"`
	TTL     time.Duration `json:"ttl"`
}