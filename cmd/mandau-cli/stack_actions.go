package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/spf13/cobra"
)

func init() {
	// Stack up command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "up [agent-id] [stack-name] [compose-file] [flags...]",
		Short: "Create and start containers from a compose file",
		Long: `Create and start containers from a compose file on a remote agent.

Examples:
  mandau stack up agent-001 mystack ./docker-compose.yaml
  mandau stack up agent-001 mystack ./docker-compose.yaml -d`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle help manually since flag parsing is disabled
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "up")
		},
	})

	// Stack down command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "down [agent-id] [stack-name] [compose-file] [flags...]",
		Short: "Stop and remove containers, networks, images",
		Long: `Stop and remove containers, networks, and images created by a stack.

Examples:
  mandau stack down agent-001 mystack ./docker-compose.yaml
  mandau stack down agent-001 mystack ./docker-compose.yaml --volumes`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "down")
		},
	})

	// Stack start command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "start [agent-id] [stack-name] [compose-file]",
		Short: "Start existing containers",
		Long: `Start existing containers without recreating them.

Examples:
  mandau stack start agent-001 mystack ./docker-compose.yaml`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "start")
		},
	})

	// Stack stop command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "stop [agent-id] [stack-name] [compose-file]",
		Short: "Stop running containers without removing them",
		Long: `Stop running containers without removing them.

Examples:
  mandau stack stop agent-001 mystack ./docker-compose.yaml`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "stop")
		},
	})

	// Stack restart command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "restart [agent-id] [stack-name] [compose-file]",
		Short: "Restart containers",
		Long: `Restart containers in the stack.

Examples:
  mandau stack restart agent-001 mystack ./docker-compose.yaml`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "restart")
		},
	})

	// Stack ps command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "ps [agent-id] [stack-name] [compose-file]",
		Short: "List containers from the compose file",
		Long: `List containers from the compose file.

Examples:
  mandau stack ps agent-001 mystack ./docker-compose.yaml`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "ps")
		},
	})

	// Stack logs command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "logs [agent-id] [stack-name] [compose-file] [flags...]",
		Short: "Show logs from running services",
		Long: `Show logs from running services in the stack.

Examples:
  mandau stack logs agent-001 mystack ./docker-compose.yaml
  mandau stack logs agent-001 mystack ./docker-compose.yaml -f
  mandau stack logs agent-001 mystack ./docker-compose.yaml --tail=100`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "logs")
		},
	})

	// Stack pull command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "pull [agent-id] [stack-name] [compose-file]",
		Short: "Pull service images",
		Long: `Pull service images without starting containers.

Examples:
  mandau stack pull agent-001 mystack ./docker-compose.yaml`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "pull")
		},
	})

	// Stack build command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "build [agent-id] [stack-name] [compose-file]",
		Short: "Build or rebuild services",
		Long: `Build or rebuild services in the stack.

Examples:
  mandau stack build agent-001 mystack ./docker-compose.yaml`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "build")
		},
	})

	// Stack create command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "create [agent-id] [stack-name] [compose-file]",
		Short: "Create services without starting them",
		Long: `Create services without starting them.

Examples:
  mandau stack create agent-001 mystack ./docker-compose.yaml`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "create")
		},
	})

	// Stack kill command
	stackCmd.AddCommand(&cobra.Command{
		Use:   "kill [agent-id] [stack-name] [compose-file]",
		Short: "Force stop containers",
		Long: `Force stop containers in the stack.

Examples:
  mandau stack kill agent-001 mystack ./docker-compose.yaml`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg == "--help" || arg == "-h" {
					return cmd.Help()
				}
			}
			if len(args) < 3 {
				return fmt.Errorf("requires at least 3 arguments: [agent-id] [stack-name] [compose-file]")
			}
			return cli.stackAction(cmd, args, "kill")
		},
	})
}

// stackAction is a generic handler for all stack actions
func (c *CLI) stackAction(cmd *cobra.Command, args []string, action string) error {
	agentID := args[0]
	stackName := args[1]
	composeFile := args[2]

	// Read compose file
	content, err := os.ReadFile(composeFile)
	if err != nil {
		return fmt.Errorf("read compose file: %w", err)
	}

	// Read optional .env file
	envPath := filepath.Join(filepath.Dir(composeFile), ".env")
	envContent := ""
	if data, err := os.ReadFile(envPath); err == nil {
		envContent = string(data)
	}

	// Collect any trailing arguments (flags)
	var customArgs []string
	if len(args) > 3 {
		customArgs = append([]string{action}, args[3:]...)
	} else {
		customArgs = []string{action}
	}

	// Call ApplyStack
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

	actionMsg := getActionMessage(action, stackName)
	fmt.Printf("%s '%s' on agent '%s'...\n", actionMsg, stackName, agentID)

	hasError := false
	for {
		event, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
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
