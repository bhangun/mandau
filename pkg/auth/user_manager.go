// Package auth provides JWT authentication middleware for the REST API
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user account
type User struct {
	Username  string   `json:"username"`
	Password  string   `json:"password"` // bcrypt hashed
	Roles     []string `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

// UserManager manages user accounts stored on disk
type UserManager struct {
	mu       sync.RWMutex
	users    map[string]*User
	store    *UserStore
}

// UserStore is the on-disk storage for users
type UserStore struct {
	Users    []*User `json:"users"`
	FilePath string  `json:"-"`
}

// NewUserManager creates a new user manager
func NewUserManager(dataDir string) (*UserManager, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	storePath := filepath.Join(dataDir, "users.json")
	store := &UserStore{
		FilePath: storePath,
		Users:    make([]*User, 0),
	}

	// Load existing users
	if data, err := os.ReadFile(storePath); err == nil {
		if err := json.Unmarshal(data, &store.Users); err != nil {
			return nil, fmt.Errorf("load users from %s: %w", storePath, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read users file: %w", err)
	}

	// Build users map
	users := make(map[string]*User)
	for _, u := range store.Users {
		users[u.Username] = u
	}

	return &UserManager{
		users: users,
		store: store,
	}, nil
}

// AddUser adds a new user
func (um *UserManager) AddUser(username, password, role string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Check if user already exists
	if _, exists := um.users[username]; exists {
		return fmt.Errorf("user '%s' already exists", username)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Create user
	user := &User{
		Username:  username,
		Password:  string(hashedPassword),
		Roles:     []string{role},
		CreatedAt: time.Now(),
	}

	um.users[username] = user
	um.store.Users = append(um.store.Users, user)

	// Persist to disk
	return um.save()
}

// DeleteUser deletes a user
func (um *UserManager) DeleteUser(username string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	if _, exists := um.users[username]; !exists {
		return fmt.Errorf("user '%s' not found", username)
	}

	// Remove from map
	delete(um.users, username)

	// Remove from store
	for i, u := range um.store.Users {
		if u.Username == username {
			um.store.Users = append(um.store.Users[:i], um.store.Users[i+1:]...)
			break
		}
	}

	// Persist to disk
	return um.save()
}

// ListUsers returns all users (without passwords)
func (um *UserManager) ListUsers() []map[string]interface{} {
	um.mu.RLock()
	defer um.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(um.users))
	for _, u := range um.users {
		result = append(result, map[string]interface{}{
			"username":   u.Username,
			"roles":      u.Roles,
			"created_at": u.CreatedAt,
			"last_login": u.LastLogin,
		})
	}

	return result
}

// Authenticate validates username/password
func (um *UserManager) Authenticate(username, password string) (*User, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, exists := um.users[username]
	if !exists {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login time
	user.LastLogin = func() *time.Time { t := time.Now(); return &t }()

	// Persist last login
	go func() {
		um.mu.Lock()
		um.save()
		um.mu.Unlock()
	}()

	return user, nil
}

// UserExists checks if a user exists
func (um *UserManager) UserExists(username string) bool {
	um.mu.RLock()
	defer um.mu.RUnlock()
	_, exists := um.users[username]
	return exists
}

// GetDefaultAdminPassword generates a random password for initial admin setup
func GetDefaultAdminPassword() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// save persists users to disk via the store
func (um *UserManager) save() error {
	return um.store.save()
}

// save persists users to disk
func (s *UserStore) save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal users: %w", err)
	}

	if err := os.WriteFile(s.FilePath, data, 0600); err != nil {
		return fmt.Errorf("write users file: %w", err)
	}

	return nil
}
