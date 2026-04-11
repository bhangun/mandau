package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bhangun/mandau/pkg/config"
	"github.com/spf13/cobra"
)

func init() {
	agentCmd.AddCommand(&cobra.Command{
		Use:   "default [agent-id]",
		Short: "Set or show the default agent",
		Args:  cobra.MaximumNArgs(1),
		RunE:  cli.defaultAgent,
	})
}

func (c *CLI) defaultAgent(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		if c.config.DefaultAgent != "" {
			fmt.Printf("Current default agent: %s\n", c.config.DefaultAgent)
		} else {
			fmt.Println("No default agent set. Use 'mandau agent default <agent-id>' to set one.")
		}
		return nil
	}

	agentID := args[0]
	c.config.DefaultAgent = agentID

	// Save to config file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}
	configPath := filepath.Join(homeDir, ".mandau", "config.yaml")

	if err := config.SaveCoreConfig(configPath, c.config); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("✓ Default agent set to: %s\n", agentID)
	return nil
}
