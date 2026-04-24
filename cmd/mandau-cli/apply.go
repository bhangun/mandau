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

Environment Variables:
  By default, reads .env file from the same directory as the compose file.
  Use --env to include variables from the secure mandau env store:
    mandau apply compose.yaml --env DATABASE_URL,API_KEY
  Use --env-file to include additional .env files:
    mandau apply compose.yaml --env-file prod.env

Actions:
  The second argument determines the action to perform:
  up      - Create and start containers (default if no action specified)
  down    - Stop and remove containers, networks, images
  start   - Start existing containers
  stop    - Stop running containers without removing them
  restart - Restart containers
  pause   - Pause all processes within containers
  unpause - Unpause all processes within containers
  ps      - List containers from the compose file
  logs    - Show logs from running services
  pull    - Pull service images
  build   - Build or rebuild services
  create  - Create services without starting them
  kill    - Force stop containers

Examples:
  mandau apply docker-compose.yaml
  mandau apply docker-compose.yaml up -d
  mandau apply docker-compose.yaml --env DB_PASSWORD,API_KEY
  mandau apply docker-compose.yaml --env-file prod.env
  mandau -a remote-agent apply compose.yaml up -d`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.runApply(cmd, args)
	},
}

func (c *CLI) runApply(cmd *cobra.Command, args []string) error {
	// Check for help flag manually since we disabled flag parsing
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return cmd.Help()
		}
	}

	// Parse custom flags before args processing
	envKeys, envFiles, remainingArgs := parseCustomFlags(args)

	filePath := remainingArgs[0]

	// 1. Resolve agent
	agentID, err := c.resolveAgent(cmd)
	if err != nil {
		return err
	}

	// 2. Resolve stack name
	stackName := deriveStackName(filePath)

	// 3. Parse arguments to extract action and flags
	// Args structure: [file] [action] [flags...]
	var action string
	var customArgs []string
	if len(remainingArgs) > 1 {
		action = remainingArgs[1]
		customArgs = remainingArgs[1:] // Include action in customArgs
	} else {
		// Default action is "up"
		action = "up"
		customArgs = []string{"up"}
	}

	// Validate action before reading file
	validActions := map[string]bool{
		"up":      true,
		"down":    true,
		"start":   true,
		"stop":    true,
		"restart": true,
		"pause":   true,
		"unpause": true,
		"ps":      true,
		"logs":    true,
		"pull":    true,
		"build":   true,
		"create":  true,
		"kill":    true,
	}

	if !validActions[action] {
		return fmt.Errorf("invalid action '%s'. Valid actions: up, down, start, stop, restart, pause, unpause, ps, logs, pull, build, create, kill", action)
	}

	// 4. Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read compose file: %w", err)
	}

	// Collect environment variables from multiple sources
	envContent := ""

	// a) Read optional .env file locally adjacent to the compose file
	envPath := filepath.Join(filepath.Dir(filePath), ".env")
	if data, err := os.ReadFile(envPath); err == nil {
		envContent = string(data)
	}

	// b) Include additional --env-file files
	for _, ef := range envFiles {
		efResolved := ef
		if strings.HasPrefix(ef, "~") {
			home, _ := os.UserHomeDir()
			efResolved = home + ef[1:]
		}
		if data, err := os.ReadFile(efResolved); err == nil {
			if envContent != "" {
				envContent += "\n"
			}
			envContent += "# From " + ef + "\n"
			envContent += string(data)
		}
	}

	// c) Include variables from secure mandau env store
	if len(envKeys) > 0 {
		store, err := getEnvStore()
		if err == nil {
			allEnvs, err := store.GetAll()
			if err == nil {
				var storeEnvs []string
				for _, key := range envKeys {
					if value, ok := allEnvs[key]; ok {
						storeEnvs = append(storeEnvs, key+"="+value)
					}
				}
				if len(storeEnvs) > 0 {
					if envContent != "" {
						envContent += "\n"
					}
					envContent += "# From mandau env store\n"
					envContent += strings.Join(storeEnvs, "\n")
				}
			}
		}
	}

	// 5. Build action message
	actionMessage := getActionMessage(action, stackName)

	// 6. Call ApplyStack
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

	fmt.Printf("%s '%s' on agent '%s'...\n", actionMessage, stackName, agentID)

	hasError := false
	for {
		event, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			// Check if we already received an error in the stream
			if !hasError {
				return fmt.Errorf("stream error: %w", err)
			}
			break
		}

		if event.Message != "" {
			fmt.Printf("  → %s\n", event.Message)
		}
		if event.Progress > 0 {
			fmt.Printf("  [%d%%]\n", event.Progress)
		}
		if event.Error != "" {
			fmt.Printf("  ✗ Error: %s\n", event.Error)
			hasError = true
		}
	}

	if hasError {
		return fmt.Errorf("action '%s' completed with errors", action)
	}

	fmt.Printf("✅ '%s' %s completed successfully\n", stackName, action)
	return nil
}

// getActionMessage returns a user-friendly message for the action being performed
func getActionMessage(action, stackName string) string {
	switch action {
	case "up":
		return fmt.Sprintf("🚀 Deploying stack")
	case "down":
		return fmt.Sprintf("⏬ Removing stack")
	case "start":
		return fmt.Sprintf("▶️  Starting stack")
	case "stop":
		return fmt.Sprintf("⏹️  Stopping stack")
	case "restart":
		return fmt.Sprintf("🔄 Restarting stack")
	case "pause":
		return fmt.Sprintf("⏸️  Pausing stack")
	case "unpause":
		return fmt.Sprintf("▶️  Unpausing stack")
	case "ps":
		return fmt.Sprintf("📋 Listing stack containers")
	case "logs":
		return fmt.Sprintf("📄 Fetching stack logs")
	case "pull":
		return fmt.Sprintf("⬇️  Pulling images for stack")
	case "build":
		return fmt.Sprintf("🔨 Building images for stack")
	case "create":
		return fmt.Sprintf("📦 Creating services for stack")
	case "kill":
		return fmt.Sprintf("💀 Killing containers in stack")
	default:
		return fmt.Sprintf("Processing stack")
	}
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

// parseCustomFlags extracts --env and --env-file flags from args
// Returns: envKeys, envFiles, remainingArgs
func parseCustomFlags(args []string) ([]string, []string, []string) {
	var envKeys []string
	var envFiles []string
	var remaining []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--env" && i+1 < len(args) {
			// --env KEY1,KEY2 or --env KEY1 --env KEY2
			next := args[i+1]
			if strings.Contains(next, ",") {
				// Comma-separated: --env KEY1,KEY2,KEY3
				envKeys = append(envKeys, strings.Split(next, ",")...)
				i++ // skip the next arg
			} else if !strings.HasPrefix(next, "-") {
				// Single key, check if next is also a key (no - prefix)
				envKeys = append(envKeys, next)
				i++ // skip the next arg
			} else {
				// No value after --env, will be handled by next iteration
				remaining = append(remaining, arg)
			}
		} else if arg == "--env-file" && i+1 < len(args) {
			// --env-file path/to/file.env
			envFiles = append(envFiles, args[i+1])
			i++ // skip the next arg
		} else if strings.HasPrefix(arg, "--env=") {
			// --env=KEY1,KEY2
			keys := strings.TrimPrefix(arg, "--env=")
			envKeys = append(envKeys, strings.Split(keys, ",")...)
		} else if strings.HasPrefix(arg, "--env-file=") {
			// --env-file=path/to/file.env
			file := strings.TrimPrefix(arg, "--env-file=")
			envFiles = append(envFiles, file)
		} else {
			remaining = append(remaining, arg)
		}
	}

	return envKeys, envFiles, remaining
}
