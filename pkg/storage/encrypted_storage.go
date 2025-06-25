package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"sitemap-go/pkg/logger"
)

// EncryptedFileStorage provides encrypted file-based storage
type EncryptedFileStorage struct {
	dataDir   string
	encryptor *AESEncryptor
	cache     Cache
	log       *logger.Logger
	mu        sync.RWMutex
}

// NewEncryptedFileStorage creates a new encrypted file storage
func NewEncryptedFileStorage(config StorageConfig, passphrase string) (*EncryptedFileStorage, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	
	// Create encryptor
	encConfig := DefaultEncryptionConfig()
	encryptor, err := NewAESEncryptor(passphrase, encConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}
	
	// Create cache if enabled
	var cache Cache
	if config.CacheSize > 0 {
		cache = NewMemoryCache(config.CacheSize)
	}
	
	storage := &EncryptedFileStorage{
		dataDir:   config.DataDir,
		encryptor: encryptor,
		cache:     cache,
		log:       logger.GetLogger().WithField("component", "encrypted_storage"),
	}
	
	storage.log.WithFields(map[string]interface{}{
		"data_dir":        config.DataDir,
		"encryption":      config.EncryptData,
		"cache_size":      config.CacheSize,
		"key_fingerprint": encryptor.GetKeyFingerprint(),
	}).Info("Encrypted storage initialized")
	
	return storage, nil
}

// Save stores data with optional encryption
func (efs *EncryptedFileStorage) Save(ctx context.Context, key string, data interface{}) error {
	efs.mu.Lock()
	defer efs.mu.Unlock()
	
	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		efs.log.WithError(err).WithField("key", key).Error("Failed to marshal data")
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	
	// Encrypt data
	encryptedData, err := efs.encryptor.Encrypt(jsonData)
	if err != nil {
		efs.log.WithError(err).WithField("key", key).Error("Failed to encrypt data")
		return fmt.Errorf("failed to encrypt data: %w", err)
	}
	
	// Write to file
	filePath := efs.getFilePath(key)
	if err := efs.ensureDir(filepath.Dir(filePath)); err != nil {
		return err
	}
	
	if err := os.WriteFile(filePath, encryptedData, 0644); err != nil {
		efs.log.WithError(err).WithField("key", key).Error("Failed to write file")
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	// Update cache if available
	if efs.cache != nil {
		efs.cache.Set(key, data)
	}
	
	efs.log.WithFields(map[string]interface{}{
		"key":  key,
		"size": len(encryptedData),
	}).Debug("Data saved successfully")
	
	return nil
}

// Load retrieves and decrypts data
func (efs *EncryptedFileStorage) Load(ctx context.Context, key string, dest interface{}) error {
	efs.mu.RLock()
	defer efs.mu.RUnlock()
	
	// Check cache first
	if efs.cache != nil {
		if cached, found := efs.cache.Get(key); found {
			// Copy cached data to destination
			cachedJson, err := json.Marshal(cached)
			if err == nil {
				if err := json.Unmarshal(cachedJson, dest); err == nil {
					efs.log.WithField("key", key).Debug("Data loaded from cache")
					return nil
				}
			}
		}
	}
	
	// Read from file
	filePath := efs.getFilePath(key)
	encryptedData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("key not found: %s", key)
		}
		efs.log.WithError(err).WithField("key", key).Error("Failed to read file")
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	// Decrypt data
	jsonData, err := efs.encryptor.Decrypt(encryptedData)
	if err != nil {
		efs.log.WithError(err).WithField("key", key).Error("Failed to decrypt data")
		return fmt.Errorf("failed to decrypt data: %w", err)
	}
	
	// Unmarshal JSON
	if err := json.Unmarshal(jsonData, dest); err != nil {
		efs.log.WithError(err).WithField("key", key).Error("Failed to unmarshal data")
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}
	
	// Update cache if available
	if efs.cache != nil {
		efs.cache.Set(key, dest)
	}
	
	efs.log.WithField("key", key).Debug("Data loaded successfully")
	return nil
}

// Delete removes data and clears cache
func (efs *EncryptedFileStorage) Delete(ctx context.Context, key string) error {
	efs.mu.Lock()
	defer efs.mu.Unlock()
	
	filePath := efs.getFilePath(key)
	
	// Remove from file system
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		efs.log.WithError(err).WithField("key", key).Error("Failed to delete file")
		return fmt.Errorf("failed to delete file: %w", err)
	}
	
	// Remove from cache
	if efs.cache != nil {
		efs.cache.Delete(key)
	}
	
	efs.log.WithField("key", key).Debug("Data deleted successfully")
	return nil
}

// Exists checks if a key exists
func (efs *EncryptedFileStorage) Exists(ctx context.Context, key string) (bool, error) {
	efs.mu.RLock()
	defer efs.mu.RUnlock()
	
	// Check cache first
	if efs.cache != nil {
		if _, found := efs.cache.Get(key); found {
			return true, nil
		}
	}
	
	// Check file system
	filePath := efs.getFilePath(key)
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	
	return true, nil
}

// getFilePath generates file path for a given key
func (efs *EncryptedFileStorage) getFilePath(key string) string {
	// Use subdirectories to avoid too many files in one directory
	if len(key) >= 2 {
		subDir := key[:2]
		return filepath.Join(efs.dataDir, subDir, key+".enc")
	}
	return filepath.Join(efs.dataDir, key+".enc")
}

// ensureDir creates directory if it doesn't exist
func (efs *EncryptedFileStorage) ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return nil
}

// RotateEncryptionKey rotates the encryption key and re-encrypts all data
func (efs *EncryptedFileStorage) RotateEncryptionKey(newPassphrase string) error {
	efs.mu.Lock()
	defer efs.mu.Unlock()
	
	efs.log.Info("Starting encryption key rotation")
	
	// Create new encryptor
	newEncConfig := DefaultEncryptionConfig()
	newEncryptor, err := NewAESEncryptor(newPassphrase, newEncConfig)
	if err != nil {
		return fmt.Errorf("failed to create new encryptor: %w", err)
	}
	
	// Walk through all files and re-encrypt
	err = filepath.Walk(efs.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && filepath.Ext(path) == ".enc" {
			// Read and decrypt with old key
			encryptedData, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			
			jsonData, err := efs.encryptor.Decrypt(encryptedData)
			if err != nil {
				efs.log.WithError(err).WithField("file", path).Warn("Failed to decrypt file during rotation, skipping")
				return nil // Skip corrupted files
			}
			
			// Encrypt with new key
			newEncryptedData, err := newEncryptor.Encrypt(jsonData)
			if err != nil {
				return err
			}
			
			// Write back
			if err := os.WriteFile(path, newEncryptedData, info.Mode()); err != nil {
				return err
			}
		}
		
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("failed to rotate encryption key: %w", err)
	}
	
	// Update encryptor
	efs.encryptor = newEncryptor
	
	// Clear cache as all data has been re-encrypted
	if efs.cache != nil {
		efs.cache.Clear()
	}
	
	efs.log.WithField("new_key_fingerprint", newEncryptor.GetKeyFingerprint()).Info("Encryption key rotation completed")
	return nil
}