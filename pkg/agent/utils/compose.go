package utils

import (
	"context"
	"os/exec"
	"sync"
)

var (
	composeCmd []string
	composeMu  sync.RWMutex
)

// GetComposeCommand returns the available compose command for the system.
// It detects if 'docker compose' (plugin) or 'docker-compose' (standalone) is available.
func GetComposeCommand(ctx context.Context) []string {
	composeMu.RLock()
	if composeCmd != nil {
		defer composeMu.RUnlock()
		return composeCmd
	}
	composeMu.RUnlock()

	composeMu.Lock()
	defer composeMu.Unlock()

	// Double check after lock
	if composeCmd != nil {
		return composeCmd
	}

	// 1. Try 'docker compose'
	cmd := exec.CommandContext(ctx, "docker", "compose", "version")
	if err := cmd.Run(); err == nil {
		composeCmd = []string{"docker", "compose"}
		return composeCmd
	}

	// 2. Try 'docker-compose'
	cmd = exec.CommandContext(ctx, "docker-compose", "version")
	if err := cmd.Run(); err == nil {
		composeCmd = []string{"docker-compose"}
		return composeCmd
	}

	// Default to 'docker compose' if nothing found (standard V2)
	composeCmd = []string{"docker", "compose"}
	return composeCmd
}
