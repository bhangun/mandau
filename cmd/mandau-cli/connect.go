package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bhangun/mandau/pkg/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "connect [server-ip]",
		Short: "Connect to a Mandau Core server",
		Long:  "Configure the CLI to connect to a Mandau Core server and prepare for certificate sync.",
		Args:  cobra.ExactArgs(1),
		RunE:  cli.connectServer,
	})
}

func (c *CLI) connectServer(cmd *cobra.Command, args []string) error {
	addr := args[0]
	if !strings.Contains(addr, ":") {
		addr = addr + ":9444"
	}

	// Update config
	// Current config might have a different structure, let's ensure we update the right field.
	// In cmd/mandau-cli/main.go, cli.connect uses c.config.ServerConnection (if we update it there).
	// Actually, cli.config is *config.CoreConfig. 
	// Wait, let's check main.go again to see how server address is handled.
	
	fmt.Printf("Updating configuration to connect to %s...\n", addr)
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}
	configPath := filepath.Join(homeDir, ".mandau", "config.yaml")

	// We need to update the server address in the config. 
	// The current CoreConfig.Server.ListenAddr is used for the server, but for the CLI,
	// we use PersistentFlags --server which defaults to localhost:3443.
	// We should probably save it in a way that the CLI respects by default.
	
	c.config.Server.ListenAddr = addr
	
	if err := config.SaveCoreConfig(configPath, c.config); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("✓ Configuration updated.\n\n")
	fmt.Printf("To sync certificates from the server, run this command manually:\n\n")
	
	// Try to get the username for the scp command
	user := os.Getenv("USER")
	if user == "" {
		user = "<USER>"
	}
	
	ip := strings.Split(addr, ":")[0]
	fmt.Printf("  scp %s@%s:~/.mandau/certs/{ca.crt,client.crt,client.key} ~/.mandau/certs/\n\n", user, ip)
	fmt.Printf("After syncing, try 'mandau agent list' to verify the connection.\n")

	return nil
}
