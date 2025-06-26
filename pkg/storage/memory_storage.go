package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// MemoryStorage provides a simple in-memory storage implementation
type MemoryStorage struct {
	data map[string][]byte
	mu   sync.RWMutex
}

// NewMemoryStorage creates a new memory storage instance
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		data: make(map[string][]byte),
	}
}

// Save stores data in memory
func (ms *MemoryStorage) Save(ctx context.Context, key string, data interface{}) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	
	ms.data[key] = jsonData
	return nil
}

// Load retrieves data from memory
func (ms *MemoryStorage) Load(ctx context.Context, key string, dest interface{}) error {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	jsonData, exists := ms.data[key]
	if !exists {
		return fmt.Errorf("key not found: %s", key)
	}
	
	if err := json.Unmarshal(jsonData, dest); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}
	
	return nil
}

// Delete removes data from memory
func (ms *MemoryStorage) Delete(ctx context.Context, key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	delete(ms.data, key)
	return nil
}

// Exists checks if a key exists in memory
func (ms *MemoryStorage) Exists(ctx context.Context, key string) (bool, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	_, exists := ms.data[key]
	return exists, nil
}