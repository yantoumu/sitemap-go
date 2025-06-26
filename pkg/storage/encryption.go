package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"sitemap-go/pkg/logger"
)

// EncryptionConfig holds configuration for AES-256 encryption
type EncryptionConfig struct {
	KeyDerivationSalt []byte `json:"key_derivation_salt"`
	KeySize           int    `json:"key_size"` // 32 bytes for AES-256
}

// DefaultEncryptionConfig returns default encryption configuration
func DefaultEncryptionConfig() EncryptionConfig {
	// Use deterministic salt for consistent key derivation across restarts
	// This ensures the same passphrase always generates the same encryption key
	salt := []byte("sitemap-go-default-salt-32-bytes!")
	
	return EncryptionConfig{
		KeyDerivationSalt: salt,
		KeySize:           32, // AES-256
	}
}

// DeterministicEncryptionConfig creates config with salt derived from passphrase
// This ensures consistent encryption/decryption across program restarts
func DeterministicEncryptionConfig(passphrase string) EncryptionConfig {
	// Create deterministic salt from passphrase using SHA-256
	// This ensures same passphrase always produces same salt
	hash := sha256.Sum256([]byte(passphrase + "-sitemap-go-salt-v1"))
	salt := hash[:]
	
	return EncryptionConfig{
		KeyDerivationSalt: salt,
		KeySize:           32, // AES-256
	}
}

// AESEncryptor provides AES-256-GCM encryption/decryption
type AESEncryptor struct {
	config EncryptionConfig
	key    []byte
	log    *logger.Logger
}

// NewAESEncryptor creates a new AES encryptor with the given passphrase
func NewAESEncryptor(passphrase string, config EncryptionConfig) (*AESEncryptor, error) {
	if len(passphrase) == 0 {
		return nil, fmt.Errorf("passphrase cannot be empty")
	}
	
	// Derive key from passphrase using SHA-256
	key := deriveKey(passphrase, config.KeyDerivationSalt, config.KeySize)
	
	return &AESEncryptor{
		config: config,
		key:    key,
		log:    logger.GetLogger().WithField("component", "aes_encryptor"),
	}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
func (e *AESEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("plaintext cannot be empty")
	}
	
	// Create AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		e.log.WithError(err).Error("Failed to create AES cipher")
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		e.log.WithError(err).Error("Failed to create GCM")
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		e.log.WithError(err).Error("Failed to generate nonce")
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	
	e.log.WithField("size", len(ciphertext)).Debug("Data encrypted successfully")
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (e *AESEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("ciphertext cannot be empty")
	}
	
	// Create AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		e.log.WithError(err).Error("Failed to create AES cipher")
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		e.log.WithError(err).Error("Failed to create GCM")
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Check minimum length
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	
	// Extract nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	
	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		e.log.WithError(err).Error("Failed to decrypt data")
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	
	e.log.WithField("size", len(plaintext)).Debug("Data decrypted successfully")
	return plaintext, nil
}

// EncryptString encrypts a string and returns hex-encoded result
func (e *AESEncryptor) EncryptString(plaintext string) (string, error) {
	encrypted, err := e.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(encrypted), nil
}

// DecryptString decrypts hex-encoded string
func (e *AESEncryptor) DecryptString(ciphertext string) (string, error) {
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("invalid hex encoding: %w", err)
	}
	
	decrypted, err := e.Decrypt(data)
	if err != nil {
		return "", err
	}
	
	return string(decrypted), nil
}

// GetKeyFingerprint returns a fingerprint of the encryption key for verification
func (e *AESEncryptor) GetKeyFingerprint() string {
	hash := sha256.Sum256(e.key)
	return hex.EncodeToString(hash[:8]) // First 8 bytes as fingerprint
}

// RotateKey generates a new key from a new passphrase
func (e *AESEncryptor) RotateKey(newPassphrase string) error {
	if len(newPassphrase) == 0 {
		return fmt.Errorf("new passphrase cannot be empty")
	}
	
	// Generate new salt for key rotation
	newSalt := make([]byte, 32)
	if _, err := rand.Read(newSalt); err != nil {
		return fmt.Errorf("failed to generate new salt: %w", err)
	}
	
	// Derive new key
	newKey := deriveKey(newPassphrase, newSalt, e.config.KeySize)
	
	// Update configuration and key
	e.config.KeyDerivationSalt = newSalt
	e.key = newKey
	
	e.log.Info("Encryption key rotated successfully")
	return nil
}

// deriveKey derives an encryption key from passphrase and salt using SHA-256
func deriveKey(passphrase string, salt []byte, keySize int) []byte {
	// Simple key derivation using SHA-256 (in production, consider using PBKDF2 or Argon2)
	hash := sha256.New()
	hash.Write([]byte(passphrase))
	hash.Write(salt)
	key := hash.Sum(nil)
	
	// If we need exactly keySize bytes, truncate or extend
	if len(key) > keySize {
		return key[:keySize]
	}
	
	// If key is shorter than required, extend it
	for len(key) < keySize {
		hash.Reset()
		hash.Write(key)
		hash.Write(salt)
		additional := hash.Sum(nil)
		key = append(key, additional...)
	}
	
	return key[:keySize]
}

// SecureZero securely zeros out sensitive data in memory
func SecureZero(data []byte) {
	for i := range data {
		data[i] = 0
	}
}