package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply [file]",
	Short: "Seamlessly deploy local compose files to a remote agent",
	Long: `Seamlessly deploy local compose files to a remote agent with intelligent defaults.
Supports standard Docker Compose syntax including trailing commands like 'up -d'.

Examples:
  mandau apply docker-compose.yaml
  mandau apply my-stack.yaml up -d
  mandau -a remote-agent apply compose.yaml`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.runApply(cmd, args)
	},
}

func (c *CLI) runApply(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// 1. Resolve agent
	agentID, err := c.resolveAgent(cmd)
	if err != nil {
		return err
	}

	// 2. Resolve stack name
	stackName := deriveStackName(filePath)

	// 3. Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Read optional .env file recursively adjacent to the compose file
	envPath := filepath.Join(filepath.Dir(filePath), ".env")
	envContent := ""
	if data, err := os.ReadFile(envPath); err == nil {
		envContent = string(data)
	}

	// Collect any trailing arguments (e.g. "up", "-d")
	var customArgs []string
	if len(args) > 1 {
		customArgs = args[1:]
	}

	// 4. Call ApplyStack
	ctx := context.Background()
	stackClient := v1.NewStackServiceClient(c.conn)

	stream, err := stackClient.ApplyStack(ctx, &v1.ApplyStackRequest{
		AgentId:        agentID,
		StackName:      stackName,
		ComposeContent: string(content),
		EnvContent:     envContent,
		CustomArgs:     customArgs,
	})
	if err != nil {
		return err
	}

	fmt.Printf("🚀 Deploying stack '%s' to agent '%s'...\n", stackName, agentID)

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		if event.Message != "" {
			fmt.Printf("  → %s\n", event.Message)
		}
		if event.Progress > 0 {
			fmt.Printf("  [%d%%]\n", event.Progress)
		}
		if event.Error != "" {
			fmt.Printf("  ✗ Error: %s\n", event.Error)
		}
	}

	fmt.Printf("✅ Deployment of '%s' completed successfully\n", stackName)
	return nil
}

func deriveStackName(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	fileName := filepath.Base(absPath)
	lowerName := strings.ToLower(fileName)

	// Check if it's a generic compose filename
	if lowerName == "docker-compose.yaml" || lowerName == "docker-compose.yml" ||
		lowerName == "compose.yaml" || lowerName == "compose.yml" {
		// Use parent directory name
		return filepath.Base(filepath.Dir(absPath))
	}

	// Otherwise use filename without extension
	ext := filepath.Ext(fileName)
	name := strings.TrimSuffix(fileName, ext)

	// Clean up common suffixes like .compose or .docker
	name = strings.TrimSuffix(name, ".compose")
	name = strings.TrimSuffix(name, ".docker")

	return name
}
