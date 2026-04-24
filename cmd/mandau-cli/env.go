package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bhangun/mandau/pkg/agent/envstore"
	"github.com/spf13/cobra"
)

var (
	envStore    *envstore.SecureStore
	envStoreErr error
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage secure environment variables",
	Long: `Manage secure environment variables for remote deployments.

The env system provides secure storage and transmission of environment 
variables to remote agents. Variables can be set manually, imported from 
.env files, or loaded from the system.

Usage:
  mandau env set KEY=VALUE           Set a variable
  mandau env get KEY                 Get a variable value
  mandau env list                    List all variables (keys only)
  mandau env delete KEY              Delete a variable
  mandau env import FILE            Import from .env file
  mandau env export FILE            Export to .env file
  mandau env clear                   Clear all variables

Examples:
  mandau env set DATABASE_URL=postgres://localhost:5432/mydb
  mandau env import .env
  mandau env import ~/projects/myapp/prod.env
  mandau env list
  mandau env export prod-vars.env`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return cmd.Help()
		}
		return nil
	},
}

var envSetCmd = &cobra.Command{
	Use:   "set [KEY=VALUE]",
	Short: "Set an environment variable",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getEnvStore()
		if err != nil {
			return err
		}

		parts := strings.SplitN(args[0], "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid format: use KEY=VALUE")
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return fmt.Errorf("key cannot be empty")
		}

		if err := store.Set(key, value, "manual"); err != nil {
			return fmt.Errorf("failed to set: %w", err)
		}

		fmt.Printf("✅ Set %s\n", key)
		return nil
	},
}

var envGetCmd = &cobra.Command{
	Use:   "get [KEY]",
	Short: "Get an environment variable value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getEnvStore()
		if err != nil {
			return err
		}

		key := args[0]
		value, err := store.Get(key)
		if err != nil {
			return fmt.Errorf("key not found: %s", key)
		}

		fmt.Println(value)
		return nil
	},
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all environment variables (keys only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getEnvStore()
		if err != nil {
			return err
		}

		metadata := store.List()
		if len(metadata) == 0 {
			fmt.Println("No environment variables stored.")
			return nil
		}

		fmt.Println("Stored environment variables:")
		fmt.Println("─────────────────────────────")
		for key, meta := range metadata {
			source := meta.Source
			if meta.File != "" {
				source = fmt.Sprintf("%s (from %s)", meta.Source, meta.File)
			}
			fmt.Printf("  %-30s [%s]\n", key, source)
		}
		return nil
	},
}

var envDeleteCmd = &cobra.Command{
	Use:   "delete [KEY]",
	Short: "Delete an environment variable",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getEnvStore()
		if err != nil {
			return err
		}

		key := args[0]
		if err := store.Delete(key); err != nil {
			return fmt.Errorf("failed to delete: %w", err)
		}

		fmt.Printf("✅ Deleted %s\n", key)
		return nil
	},
}

var envImportCmd = &cobra.Command{
	Use:   "import [FILE]",
	Short: "Import environment variables from a .env file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getEnvStore()
		if err != nil {
			return err
		}

		filePath := args[0]
		// Resolve tilde
		if strings.HasPrefix(filePath, "~") {
			home, _ := os.UserHomeDir()
			filePath = home + filePath[1:]
		}

		count, err := store.ImportFromFile(filePath)
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		fmt.Printf("✅ Imported %d variables from %s\n", count, filePath)
		return nil
	},
}

var envExportCmd = &cobra.Command{
	Use:   "export [FILE]",
	Short: "Export environment variables to a .env file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getEnvStore()
		if err != nil {
			return err
		}

		filePath := args[0]
		// Resolve tilde
		if strings.HasPrefix(filePath, "~") {
			home, _ := os.UserHomeDir()
			filePath = home + filePath[1:]
		}

		if err := store.ExportToFile(filePath); err != nil {
			return fmt.Errorf("export failed: %w", err)
		}

		fmt.Printf("✅ Exported to %s\n", filePath)
		return nil
	},
}

var envClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all environment variables",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getEnvStore()
		if err != nil {
			return err
		}

		all, err := store.GetAll()
		if err != nil {
			return err
		}

		count := 0
		for key := range all {
			if err := store.Delete(key); err != nil {
				return err
			}
			count++
		}

		fmt.Printf("✅ Cleared %d variables\n", count)
		return nil
	},
}

func getEnvStore() (*envstore.SecureStore, error) {
	if envStore != nil {
		return envStore, nil
	}
	if envStoreErr != nil {
		return nil, envStoreErr
	}

	// Get master password from env or prompt
	masterPassword := os.Getenv("MANDAU_ENV_MASTER_PASSWORD")
	if masterPassword == "" {
		// Use a default derived from machine ID
		home, _ := os.UserHomeDir()
		masterPassword = filepath.Join(home, ".mandau")
	}

	storePath := envstore.DefaultStorePath()
	store, err := envstore.NewSecureStore(storePath, masterPassword)
	if err != nil {
		envStoreErr = err
		return nil, err
	}

	envStore = store
	return envStore, nil
}

func init() {
	// Add env command
	rootCmd.AddCommand(envCmd)

	// Add subcommands
	envCmd.AddCommand(envSetCmd)
	envCmd.AddCommand(envGetCmd)
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envDeleteCmd)
	envCmd.AddCommand(envImportCmd)
	envCmd.AddCommand(envExportCmd)
	envCmd.AddCommand(envClearCmd)
}
