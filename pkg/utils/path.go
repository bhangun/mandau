package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ExpandPath expands user home directory references in paths.
// On Unix-like systems, expands ~ to user's home directory.
// On Windows, expands %USERPROFILE% references to user's home directory.
func ExpandPath(path string) string {
	if path == "" {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	// Unix-style: ~/something
	if strings.HasPrefix(path, "~/") || path == "~" {
		if path == "~" {
			return homeDir
		}
		return filepath.Join(homeDir, path[2:])
	}

	// Windows-style: %USERPROFILE%\something
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(path, "%USERPROFILE%") {
			if path == "%USERPROFILE%" {
				return homeDir
			}
			return filepath.Join(homeDir, strings.TrimPrefix(path, "%USERPROFILE%")[1:])
		}
		// Also handle Windows backslash tilde: ~\something
		if strings.HasPrefix(path, "~\\") {
			return filepath.Join(homeDir, path[2:])
		}
	}

	return path
}
