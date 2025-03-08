package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// Errors
var (
	ErrKeyNotFound        = errors.New("encryption key not found")
	ErrInvalidKeySize     = errors.New("invalid key size")
	ErrDataEncryption     = errors.New("failed to encrypt data")
	ErrDataDecryption     = errors.New("failed to decrypt data")
	ErrInvalidData        = errors.New("invalid encrypted data")
	ErrKeyEnvVarNotSet    = errors.New("encryption key environment variable not set")
	ErrEncryptionDisabled = errors.New("encryption is disabled")
)

// EncryptionProvider defines interface for data encryption
type EncryptionProvider interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
	EncryptString(plaintext string) (string, error)
	DecryptString(ciphertext string) (string, error)
	IsEnabled() bool
}

// AESGCMEncryption implements encryption using AES-GCM
type AESGCMEncryption struct {
	key      []byte
	enabled  bool
	keyID    string
	mu       sync.RWMutex
	keyCache map[string][]byte // For key rotation support
}

// EncryptionConfig contains configuration for encryption
type EncryptionConfig struct {
	Enabled    bool
	KeyEnvVar  string
	DefaultKey string // Fallback key (for dev/testing only)
	KeyID      string
}

// NewAESGCMEncryption creates a new AES-GCM encryption provider
func NewAESGCMEncryption(config EncryptionConfig) (*AESGCMEncryption, error) {
	if !config.Enabled {
		return &AESGCMEncryption{
			enabled: false,
		}, nil
	}

	// Try to get key from environment
	var key []byte
	var err error
	
	if config.KeyEnvVar != "" {
		keyStr := os.Getenv(config.KeyEnvVar)
		if keyStr != "" {
			// If hex encoded
			key, err = hex.DecodeString(keyStr)
			if err != nil {
				// If not hex, try to decode from base64
				key, err = base64.StdEncoding.DecodeString(keyStr)
				if err != nil {
					// If not base64 either, use as raw key
					key = []byte(keyStr)
				}
			}
		}
	}

	// If no key from env, use default key (only for dev/testing)
	if len(key) == 0 && config.DefaultKey != "" {
		key = []byte(config.DefaultKey)
	}

	// Ensure we have a key
	if len(key) == 0 {
		return nil, ErrKeyNotFound
	}

	// Hash the key to ensure it's the right size for AES-256
	hashedKey := sha256.Sum256(key)
	
	keyID := config.KeyID
	if keyID == "" {
		keyID = "default"
	}

	return &AESGCMEncryption{
		key:      hashedKey[:],
		enabled:  true,
		keyID:    keyID,
		keyCache: map[string][]byte{keyID: hashedKey[:]},
	}, nil
}

// IsEnabled returns whether encryption is enabled
func (e *AESGCMEncryption) IsEnabled() bool {
	return e.enabled
}

// AddKey adds a key to the key cache for key rotation
func (e *AESGCMEncryption) AddKey(keyID string, key []byte) error {
	if !e.enabled {
		return ErrEncryptionDisabled
	}
	
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Hash the key to ensure it's the right size for AES-256
	hashedKey := sha256.Sum256(key)
	e.keyCache[keyID] = hashedKey[:]
	
	return nil
}

// SetActiveKey sets the active key for encryption
func (e *AESGCMEncryption) SetActiveKey(keyID string) error {
	if !e.enabled {
		return ErrEncryptionDisabled
	}
	
	e.mu.Lock()
	defer e.mu.Unlock()
	
	key, exists := e.keyCache[keyID]
	if !exists {
		return ErrKeyNotFound
	}
	
	e.key = key
	e.keyID = keyID
	
	return nil
}

// Encrypt encrypts data using AES-GCM
func (e *AESGCMEncryption) Encrypt(data []byte) ([]byte, error) {
	if !e.enabled {
		return data, nil
	}

	e.mu.RLock()
	key := e.key
	keyID := e.keyID
	e.mu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDataEncryption, err.Error())
	}

	// Generate a nonce (Number used ONCE)
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDataEncryption, err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDataEncryption, err.Error())
	}

	// Encrypt and authenticate
	ciphertext := aesgcm.Seal(nil, nonce, data, nil)

	// Format: keyID + nonce + ciphertext
	keyIDBytes := []byte(keyID)
	keyIDLen := byte(len(keyIDBytes))
	
	result := make([]byte, 1+keyIDLen+len(nonce)+len(ciphertext))
	result[0] = keyIDLen
	copy(result[1:1+keyIDLen], keyIDBytes)
	copy(result[1+keyIDLen:1+keyIDLen+len(nonce)], nonce)
	copy(result[1+keyIDLen+len(nonce):], ciphertext)

	return result, nil
}

// Decrypt decrypts data using AES-GCM
func (e *AESGCMEncryption) Decrypt(data []byte) ([]byte, error) {
	if !e.enabled {
		return data, nil
	}

	// Check minimum size (1 byte for keyID length + at least 1 byte for keyID + 12 bytes for nonce + at least 1 byte for data)
	if len(data) < 15 {
		return nil, ErrInvalidData
	}

	// Extract keyID, nonce, and ciphertext
	keyIDLen := int(data[0])
	if 1+keyIDLen+12 > len(data) {
		return nil, ErrInvalidData
	}
	
	keyID := string(data[1 : 1+keyIDLen])
	nonce := data[1+keyIDLen : 1+keyIDLen+12]
	ciphertext := data[1+keyIDLen+12:]

	// Get the key
	e.mu.RLock()
	key, exists := e.keyCache[keyID]
	if !exists {
		key = e.key // Fallback to current key
	}
	e.mu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDataDecryption, err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDataDecryption, err.Error())
	}

	// Decrypt and verify
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDataDecryption, err.Error())
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns a base64-encoded result
func (e *AESGCMEncryption) EncryptString(plaintext string) (string, error) {
	if !e.enabled {
		return plaintext, nil
	}

	encrypted, err := e.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptString decrypts a base64-encoded encrypted string
func (e *AESGCMEncryption) DecryptString(ciphertext string) (string, error) {
	if !e.enabled {
		return ciphertext, nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrDataDecryption, err.Error())
	}

	decrypted, err := e.Decrypt(data)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

// GenerateKey generates a random encryption key
func GenerateKey(bits int) (string, error) {
	if bits%8 != 0 {
		return "", ErrInvalidKeySize
	}

	bytes := bits / 8
	key := make([]byte, bytes)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}

	return hex.EncodeToString(key), nil
}

// EncryptionManager manages multiple encryption providers
type EncryptionManager struct {
	providers map[string]EncryptionProvider
	default_  string
	mu        sync.RWMutex
}

// NewEncryptionManager creates a new encryption manager
func NewEncryptionManager() *EncryptionManager {
	return &EncryptionManager{
		providers: make(map[string]EncryptionProvider),
	}
}

// RegisterProvider registers an encryption provider
func (em *EncryptionManager) RegisterProvider(name string, provider EncryptionProvider) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.providers[name] = provider
	
	// If it's the first provider, make it the default
	if em.default_ == "" {
		em.default_ = name
	}
}

// SetDefaultProvider sets the default encryption provider
func (em *EncryptionManager) SetDefaultProvider(name string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if _, exists := em.providers[name]; !exists {
		return ErrKeyNotFound
	}

	em.default_ = name
	return nil
}

// GetProvider returns an encryption provider by name
func (em *EncryptionManager) GetProvider(name string) (EncryptionProvider, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	provider, exists := em.providers[name]
	if !exists {
		return nil, ErrKeyNotFound
	}

	return provider, nil
}

// GetDefaultProvider returns the default encryption provider
func (em *EncryptionManager) GetDefaultProvider() (EncryptionProvider, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if em.default_ == "" {
		return nil, ErrKeyNotFound
	}

	return em.providers[em.default_], nil
}

// Encrypt encrypts data using the default provider
func (em *EncryptionManager) Encrypt(data []byte) ([]byte, error) {
	provider, err := em.GetDefaultProvider()
	if err != nil {
		return nil, err
	}

	return provider.Encrypt(data)
}

// Decrypt decrypts data using the default provider
func (em *EncryptionManager) Decrypt(data []byte) ([]byte, error) {
	provider, err := em.GetDefaultProvider()
	if err != nil {
		return nil, err
	}

	return provider.Decrypt(data)
}

// EncryptString encrypts a string using the default provider
func (em *EncryptionManager) EncryptString(plaintext string) (string, error) {
	provider, err := em.GetDefaultProvider()
	if err != nil {
		return "", err
	}

	return provider.EncryptString(plaintext)
}

// DecryptString decrypts a string using the default provider
func (em *EncryptionManager) DecryptString(ciphertext string) (string, error) {
	provider, err := em.GetDefaultProvider()
	if err != nil {
		return "", err
	}

	return provider.DecryptString(ciphertext)
} 