package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/bhangun/mandau/pkg/config"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	version = "0.0.16" // Will be set by build process

	cli     = &CLI{}
	rootCmd = &cobra.Command{
		Use:   "mandau",
		Short: "Mandau infrastructure control CLI",
		Version: version, // Add version flag
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cli.connect(cmd)
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if cli.conn != nil {
				return cli.conn.Close()
			}
			return nil
		},
	}
)

type CLI struct {
	coreClient  v1.CoreServiceClient
	agentClient v1.AgentServiceClient
	conn        *grpc.ClientConn
	config      *config.CoreConfig // For CLI, we can reuse the core config structure
}

func main() {

	// Global flags
	rootCmd.PersistentFlags().String("server", "localhost:9443", "Core server address")
	rootCmd.PersistentFlags().String("cert", "", "Client certificate")
	rootCmd.PersistentFlags().String("key", "", "Client key")
	rootCmd.PersistentFlags().String("ca", "", "CA certificate")

	// Agent commands
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent management",
	}

	agentCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all agents",
		RunE:  cli.listAgents,
	})

	// Stack commands
	stackCmd := &cobra.Command{
		Use:   "stack",
		Short: "Stack management",
	}

	stackCmd.AddCommand(&cobra.Command{
		Use:   "list [agent-id]",
		Short: "List stacks on agent",
		Args:  cobra.ExactArgs(1),
		RunE:  cli.listStacks,
	})

	stackCmd.AddCommand(&cobra.Command{
		Use:   "apply [agent-id] [stack-name] [compose-file]",
		Short: "Apply stack to agent",
		Args:  cobra.ExactArgs(3),
		RunE:  cli.applyStack,
	})

	stackCmd.AddCommand(&cobra.Command{
		Use:   "logs [agent-id] [stack-name]",
		Short: "Stream stack logs",
		Args:  cobra.ExactArgs(2),
		RunE:  cli.stackLogs,
	})

	rootCmd.AddCommand(agentCmd, stackCmd)

	// Auth commands (manage users via REST API)
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "User authentication management",
	}

	authCmd.AddCommand(&cobra.Command{
		Use:   "add [username] [password] [role]",
		Short: "Add a new user",
		Long:  "Add a new user to the core server. Role can be 'admin' or 'user'.",
		Args:  cobra.ExactArgs(3),
		RunE:  cli.addUser,
	})

	authCmd.AddCommand(&cobra.Command{
		Use:   "delete [username]",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		RunE:  cli.deleteUser,
	})

	authCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE:  cli.listUsers,
	})

	rootCmd.AddCommand(authCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func (c *CLI) connect(cmd *cobra.Command) error {
	// Try to load configuration from standard locations in order of preference
	var cfg *config.CoreConfig
	var err error

	// First, try the environment variable if set
	configPath := config.GetConfigPath("") // This will return env var value or empty string
	if configPath != "" {
		cfg, err = config.LoadCoreConfig(configPath)
		if err != nil {
			fmt.Printf("Config file not found at %s, trying standard locations\n", configPath)
		} else {
			fmt.Printf("Loaded configuration from %s\n", configPath)
		}
	}

	// If not found via env var or env var wasn't set, try standard locations
	if cfg == nil {
		// Try ~/.mandau/config.yaml (our new standard location)
		homeDir, errHome := os.UserHomeDir()
		if errHome == nil {
			standardConfigPath := fmt.Sprintf("%s/.mandau/config.yaml", homeDir)
			cfg, err = config.LoadCoreConfig(standardConfigPath)
			if err != nil {
				// Don't print error yet, we'll try other locations
			} else {
				fmt.Printf("Loaded configuration from %s\n", standardConfigPath)
			}
		}
	}

	// If still not found, try the old default location
	if cfg == nil {
		configPath = "config/core/config.yaml"
		cfg, err = config.LoadCoreConfig(configPath)
		if err != nil {
			// Config file not found, proceed with command-line flags/env vars
		} else {
			fmt.Printf("Loaded configuration from %s\n", configPath)
		}
	}

	if cfg != nil {
		c.config = cfg
	}

	serverAddr, err := c.getFlagOrEnv(cmd, "server", "MANDAU_SERVER", "localhost:9443")
	if err != nil {
		return err
	}

	certFile, err := c.getFlagOrEnv(cmd, "cert", "MANDAU_CERT", "")
	if err != nil {
		return err
	}

	keyFile, err := c.getFlagOrEnv(cmd, "key", "MANDAU_KEY", "")
	if err != nil {
		return err
	}

	caFile, err := c.getFlagOrEnv(cmd, "ca", "MANDAU_CA", "")
	if err != nil {
		return err
	}

	// Auto-discover certificates from ~/.mandau/certs/ if not provided
	if certFile == "" || keyFile == "" || caFile == "" {
		homeDir, errHome := os.UserHomeDir()
		if errHome == nil {
			mandauCertDir := filepath.Join(homeDir, ".mandau", "certs")
			
			// Check if auto-discovery directory exists
			if _, err := os.Stat(mandauCertDir); err == nil {
				// Use auto-discovered certificates if not explicitly provided
				if certFile == "" {
					autoCert := filepath.Join(mandauCertDir, "client.crt")
					if _, err := os.Stat(autoCert); err == nil {
						certFile = autoCert
					}
				}
				if keyFile == "" {
					autoKey := filepath.Join(mandauCertDir, "client.key")
					if _, err := os.Stat(autoKey); err == nil {
						keyFile = autoKey
					}
				}
				if caFile == "" {
					autoCA := filepath.Join(mandauCertDir, "ca.crt")
					if _, err := os.Stat(autoCA); err == nil {
						caFile = autoCA
					}
				}
				
				if certFile != "" && keyFile != "" && caFile != "" {
					fmt.Printf("Using auto-discovered certificates from %s\n", mandauCertDir)
				}
			}
		}
	}

	// If config was loaded, use values from config as defaults if not provided via CLI/env
	if c.config != nil {
		// Only use config values if command-line flags/environment variables were not explicitly set
		if !cmd.Flags().Changed("server") && os.Getenv("MANDAU_SERVER") == "" {
			if c.config.Server.ListenAddr != "" {
				// Ensure the address has a host (not just ":port")
				addr := c.config.Server.ListenAddr
				if strings.HasPrefix(addr, ":") {
					addr = "localhost" + addr
				}
				serverAddr = addr
			}
		}
		if !cmd.Flags().Changed("cert") && os.Getenv("MANDAU_CERT") == "" && certFile == "" {
			if c.config.Server.TLS.CertPath != "" {
				certFile = c.config.Server.TLS.CertPath
			}
		}
		if !cmd.Flags().Changed("key") && os.Getenv("MANDAU_KEY") == "" && keyFile == "" {
			if c.config.Server.TLS.KeyPath != "" {
				keyFile = c.config.Server.TLS.KeyPath
			}
		}
		if !cmd.Flags().Changed("ca") && os.Getenv("MANDAU_CA") == "" && caFile == "" {
			if c.config.Server.TLS.CAPath != "" {
				caFile = c.config.Server.TLS.CAPath
			}
		}
	}

	if certFile == "" || keyFile == "" {
		return fmt.Errorf("client certificate required (use 'mandau cert gen' to generate, or provide MANDAU_CERT, MANDAU_KEY)")
	}

	if caFile == "" {
		return fmt.Errorf("CA certificate required (use 'mandau cert gen' to generate, or provide MANDAU_CA)")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load cert: %w", err)
	}

	// Load CA certificate to verify server certificate
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("load CA cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("parse CA cert")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		ServerName:   "mandau-core", // Use the server name from the certificate
		MinVersion:   tls.VersionTLS13,
	}

	creds := credentials.NewTLS(tlsConfig)

	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	c.conn = conn
	// Use CoreServiceClient for core operations like ListAgents
	c.coreClient = v1.NewCoreServiceClient(conn)
	// Use AgentServiceClient for agent-specific operations
	c.agentClient = v1.NewAgentServiceClient(conn)

	return nil
}

// getFlagOrEnv gets a value from command line flag or environment variable
func (c *CLI) getFlagOrEnv(cmd *cobra.Command, flagName, envName, defaultValue string) (string, error) {
	// First check command line flag
	value, err := cmd.Flags().GetString(flagName)
	if err != nil {
		return "", fmt.Errorf("get flag %s: %w", flagName, err)
	}

	// If flag is not set, check environment variable
	if value == "" {
		value = os.Getenv(envName)
	}

	// If environment variable is not set, use default value
	if value == "" {
		value = defaultValue
	}

	return value, nil
}

func (c *CLI) listAgents(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	resp, err := c.coreClient.ListAgents(ctx, &v1.ListAgentsRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("%-20s %-30s %-10s %-20s\n", "ID", "HOSTNAME", "STATUS", "LAST SEEN")
	for _, agent := range resp.Agents {
		fmt.Printf("%-20s %-30s %-10s %-20s\n",
			agent.Id,
			agent.Hostname,
			agent.Status,
			agent.LastSeen.AsTime().Format("2006-01-02 15:04:05"),
		)
	}

	return nil
}

func (c *CLI) listStacks(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	ctx := context.Background()

	stackClient := v1.NewStackServiceClient(c.conn)

	resp, err := stackClient.ListStacks(ctx, &v1.ListStacksRequest{
		AgentId: agentID,
	})
	if err != nil {
		return err
	}

	fmt.Printf("%-20s %-15s %-10s %s\n", "NAME", "STATE", "CONTAINERS", "PATH")
	for _, stack := range resp.Stacks {
		fmt.Printf("%-20s %-15s %-10d %s\n",
			stack.Name,
			stack.State.String(),
			len(stack.Containers),
			stack.Path,
		)
	}

	return nil
}

func (c *CLI) applyStack(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	stackName := args[1]
	composeFile := args[2]

	content, err := os.ReadFile(composeFile)
	if err != nil {
		return fmt.Errorf("read compose file: %w", err)
	}

	ctx := context.Background()
	stackClient := v1.NewStackServiceClient(c.conn)

	stream, err := stackClient.ApplyStack(ctx, &v1.ApplyStackRequest{
		AgentId:        agentID,
		StackName:      stackName,
		ComposeContent: string(content),
	})
	if err != nil {
		return err
	}

	fmt.Printf("Applying stack %s to agent %s...\n", stackName, agentID)

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

	fmt.Println("✓ Stack applied successfully")
	return nil
}

func (c *CLI) stackLogs(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	stackName := args[1]

	ctx := context.Background()
	stackClient := v1.NewStackServiceClient(c.conn)

	stream, err := stackClient.GetStackLogs(ctx, &v1.GetStackLogsRequest{
		AgentId:   agentID,
		StackName: stackName,
		Follow:    true,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Streaming logs for stack %s...\n", stackName)

	for {
		entry, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		timestamp := entry.Timestamp.AsTime().Format("15:04:05")
		fmt.Printf("[%s] [%s] %s\n", timestamp, entry.ServiceName, string(entry.Content))
	}

	return nil
}

// User management stubs (auth commands)
func (c *CLI) addUser(cmd *cobra.Command, args []string) error {
	username := args[0]
	password := args[1]
	role := args[2]

	fmt.Printf("Adding user: %s (role: %s)\n", username, role)

	// Create user via REST API
	return c.createUserViaAPI(username, password, role)
}

func addUser(cmd *cobra.Command, args []string) error {
	return cli.addUser(cmd, args)
}

func (c *CLI) deleteUser(cmd *cobra.Command, args []string) error {
	username := args[0]
	fmt.Printf("Deleting user: %s\n", username)

	// Delete user via REST API
	return c.deleteUserViaAPI(username)
}

func deleteUser(cmd *cobra.Command, args []string) error {
	return cli.deleteUser(cmd, args)
}

// createUserViaAPI creates a user via the REST API
func (c *CLI) createUserViaAPI(username, password, role string) error {
	serverAddr, err := c.getFlagOrEnv(nil, "server", "MANDAU_SERVER", "localhost:9443")
	if err != nil {
		return err
	}

	// Get certificate paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	certFile := homeDir + "/.mandau/certs/client.crt"
	keyFile := homeDir + "/.mandau/certs/client.key"

	// Create HTTP client with mTLS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	// Load client certificate if available
	if _, err := os.Stat(certFile); err == nil {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("load client cert: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30,
	}

	// Call the user management API
	// This would be implemented based on your core server's user management endpoints
	_ = client
	_ = serverAddr

	fmt.Println("Note: User management API integration pending")
	return nil
}

// deleteUserViaAPI deletes a user via the REST API
func (c *CLI) deleteUserViaAPI(username string) error {
	fmt.Println("Note: User deletion API integration pending")
	return nil
}

// listUsersViaAPI lists users via the REST API
func (c *CLI) listUsersViaAPI() error {
	fmt.Println("Note: User listing API integration pending")
	return nil
}
