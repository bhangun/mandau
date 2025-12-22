package filesystem

import "os"

// FileInfo represents file information
type FileInfo struct {
	Path string
	Size int64
}

// Manager manages filesystem operations
type Manager struct{}

// NewManager creates a new filesystem manager
func NewManager() *Manager {
	return &Manager{}
}

// ReadFile reads a file
func (m *Manager) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
