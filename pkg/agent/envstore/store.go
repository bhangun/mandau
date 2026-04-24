package envstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	keySize    = 32 // AES-256
	saltSize   = 32
	ivSize     = 12 // GCM standard IV size
	iterations = 100000
)

// SecureStore manages encrypted environment variables
type SecureStore struct {
	storePath string
	masterKey []byte
	data      *StoreData
}

// StoreData represents the encrypted store structure
type StoreData struct {
	Entries  map[string]string    `json:"entries"`  // Encrypted values
	Metadata map[string]EntryMeta `json:"metadata"` // Non-sensitive metadata
}

// EntryMeta contains metadata about an env entry
type EntryMeta struct {
	Source   string `json:"source"`             // "manual", "imported", "file"
	File     string `json:"file,omitempty"`     // Source file if imported
	Imported string `json:"imported,omitempty"` // Import timestamp
}

// NewSecureStore creates a new secure environment store
func NewSecureStore(storePath string, masterPassword string) (*SecureStore, error) {
	// Derive key from master password
	masterKey := deriveKey(masterPassword, []byte(storePath))

	store := &SecureStore{
		storePath: storePath,
		masterKey: masterKey,
		data:      &StoreData{},
	}

	// Load existing data if available
	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

// deriveKey derives a key from password and salt using PBKDF2
func deriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, iterations, keySize, sha256.New)
}

// getSalt returns the salt for key derivation
func (s *SecureStore) getSalt() []byte {
	// Use store path as salt base (deterministic)
	hash := sha256.Sum256([]byte(s.storePath))
	return hash[:saltSize]
}

// encrypt encrypts plaintext using AES-GCM
func (s *SecureStore) encrypt(plaintext []byte) (string, error) {
	salt := s.getSalt()
	key := deriveKey(string(s.masterKey), salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, ivSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts ciphertext using AES-GCM
func (s *SecureStore) decrypt(ciphertext string) (string, error) {
	salt := s.getSalt()
	key := deriveKey(string(s.masterKey), salt)

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := ivSize
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// load loads the store from disk
func (s *SecureStore) load() error {
	if _, err := os.Stat(s.storePath); os.IsNotExist(err) {
		s.data = &StoreData{
			Entries:  make(map[string]string),
			Metadata: make(map[string]EntryMeta),
		}
		return nil
	}

	data, err := os.ReadFile(s.storePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, s.data)
}

// save saves the store to disk
func (s *SecureStore) save() error {
	data, err := json.Marshal(s.data)
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.storePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(s.storePath, data, 0600)
}

// Set sets an environment variable
func (s *SecureStore) Set(key, value string, source string) error {
	encrypted, err := s.encrypt([]byte(value))
	if err != nil {
		return fmt.Errorf("encrypt value: %w", err)
	}

	s.data.Entries[key] = encrypted
	s.data.Metadata[key] = EntryMeta{
		Source: source,
	}

	return s.save()
}

// Get gets an environment variable (decrypted)
func (s *SecureStore) Get(key string) (string, error) {
	encrypted, ok := s.data.Entries[key]
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}

	return s.decrypt(encrypted)
}

// Delete removes an environment variable
func (s *SecureStore) Delete(key string) error {
	if _, ok := s.data.Entries[key]; !ok {
		return fmt.Errorf("key not found: %s", key)
	}

	delete(s.data.Entries, key)
	delete(s.data.Metadata, key)

	return s.save()
}

// List returns all keys (without values for security)
func (s *SecureStore) List() map[string]EntryMeta {
	return s.data.Metadata
}

// GetAll returns all decrypted key-value pairs
func (s *SecureStore) GetAll() (map[string]string, error) {
	result := make(map[string]string)
	for key, encrypted := range s.data.Entries {
		value, err := s.decrypt(encrypted)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

// ImportFromFile imports environment variables from a .env file
func (s *SecureStore) ImportFromFile(filePath string) (int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("read file: %w", err)
	}

	return s.ImportFromContent(string(data), filePath)
}

// ImportFromContent imports from .env content string
func (s *SecureStore) ImportFromContent(content, source string) (int, error) {
	lines := strings.Split(content, "\n")
	count := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, "\"'")

		if key != "" {
			if err := s.Set(key, value, "imported"); err != nil {
				return count, err
			}
			// Update metadata with file source
			if meta, ok := s.data.Metadata[key]; ok {
				meta.File = source
				s.data.Metadata[key] = meta
			}
			count++
		}
	}

	return count, s.save()
}

// ExportToFile exports all variables to a .env file
func (s *SecureStore) ExportToFile(filePath string) error {
	all, err := s.GetAll()
	if err != nil {
		return err
	}

	var lines []string
	for key, value := range all {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	content := strings.Join(lines, "\n")
	return os.WriteFile(filePath, []byte(content), 0644)
}

// ExportToEnvVars returns exportable environment variables
func (s *SecureStore) ExportToEnvVars() (map[string]string, error) {
	return s.GetAll()
}

// DefaultStorePath returns the default store path
func DefaultStorePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mandau", "envstore.enc")
}
