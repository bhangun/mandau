package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	v1 "github.com/bhangun/mandau/api/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Manager manages filesystem operations
type Manager struct {
	stackRoot string
}

// NewManager creates a new filesystem manager
func NewManager(stackRoot string) *Manager {
	return &Manager{
		stackRoot: stackRoot,
	}
}

// resolvePath resolves a path relative to a stack or as an absolute path
func (m *Manager) resolvePath(stackName, path string) (string, error) {
	if stackName == "" {
		// Treat as absolute path
		return path, nil
	}

	// Resolve relative to stack root
	stackPath := filepath.Join(m.stackRoot, stackName)
	resolved := filepath.Join(stackPath, path)

	// Prevent path traversal
	if !filepath.HasPrefix(resolved, stackPath) {
		return "", fmt.Errorf("path traversal attempt: %s", path)
	}

	return resolved, nil
}

// ListFiles returns information about files in a directory
func (m *Manager) ListFiles(ctx context.Context, stackName, path string) ([]*v1.FileInfo, error) {
	resolvedPath, err := m.resolvePath(stackName, path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		return nil, err
	}

	result := make([]*v1.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		result = append(result, &v1.FileInfo{
			Name:     entry.Name(),
			Path:     filepath.Join(path, entry.Name()),
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			Modified: timestamppb.New(info.ModTime()),
			Mode:     uint32(info.Mode()),
		})
	}

	return result, nil
}

// ReadFile reads the content of a file
func (m *Manager) ReadFile(ctx context.Context, stackName, path string) ([]byte, *v1.FileInfo, error) {
	resolvedPath, err := m.resolvePath(stackName, path)
	if err != nil {
		return nil, nil, err
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, nil, err
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, nil, err
	}

	return data, &v1.FileInfo{
		Name:     info.Name(),
		Path:     path,
		IsDir:    info.IsDir(),
		Size:     info.Size(),
		Modified: timestamppb.New(info.ModTime()),
		Mode:     uint32(info.Mode()),
	}, nil
}

// WriteFile writes content to a file
func (m *Manager) WriteFile(ctx context.Context, stackName, path string, content []byte, mode uint32) error {
	resolvedPath, err := m.resolvePath(stackName, path)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if mode == 0 {
		mode = 0644
	}

	return os.WriteFile(resolvedPath, content, os.FileMode(mode))
}

// DeleteFile deletes a file or directory
func (m *Manager) DeleteFile(ctx context.Context, stackName, path string, recursive bool) error {
	resolvedPath, err := m.resolvePath(stackName, path)
	if err != nil {
		return err
	}

	if recursive {
		return os.RemoveAll(resolvedPath)
	}
	return os.Remove(resolvedPath)
}

// MoveFile moves or renames a file or directory
func (m *Manager) MoveFile(ctx context.Context, stackName, src, dest string) error {
	resolvedSrc, err := m.resolvePath(stackName, src)
	if err != nil {
		return err
	}

	resolvedDest, err := m.resolvePath(stackName, dest)
	if err != nil {
		return err
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(resolvedDest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	return os.Rename(resolvedSrc, resolvedDest)
}

// CopyFile copies a file or directory (host-side only)
func (m *Manager) CopyFile(ctx context.Context, stackName, src, dest string) error {
	resolvedSrc, err := m.resolvePath(stackName, src)
	if err != nil {
		return err
	}

	resolvedDest, err := m.resolvePath(stackName, dest)
	if err != nil {
		return err
	}

	// Simplified copy implementation (for files only)
	return copyFile(resolvedSrc, resolvedDest)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}

// CreateDirectory creates a directory
func (m *Manager) CreateDirectory(ctx context.Context, stackName, path string) error {
	resolvedPath, err := m.resolvePath(stackName, path)
	if err != nil {
		return err
	}

	return os.MkdirAll(resolvedPath, 0755)
}
