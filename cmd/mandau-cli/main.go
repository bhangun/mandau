package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/bhangun/mandau/pkg/config"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	cli     = &CLI{}
	rootCmd = &cobra.Command{
		Use:   "mandau",
		Short: "Mandau infrastructure control CLI",
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
	rootCmd.PersistentFlags().String("server", "localhost:8443", "Core server address")
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

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func (c *CLI) connect(cmd *cobra.Command) error {
	// Try to load configuration from file first
	configPath := config.GetConfigPath("config/core/config.yaml")
	cfg, err := config.LoadCoreConfig(configPath)
	if err != nil {
		// Config file not found, proceed with command-line flags/env vars
		fmt.Printf("Config file not found at %s, using command-line flags/environment variables\n", configPath)
	} else {
		fmt.Printf("Loaded configuration from %s\n", configPath)
		c.config = cfg
	}

	serverAddr, err := c.getFlagOrEnv(cmd, "server", "MANDAU_SERVER", "localhost:8443")
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

	caFile, err := c.getFlagOrEnv(cmd, "ca", "MANDAU_CA", "./certs/ca.crt")
	if err != nil {
		return err
	}

	// If config was loaded, use values from config as defaults if not provided via CLI/env
	if c.config != nil {
		if serverAddr == "localhost:8443" { // If using default and config has a value
			serverAddr = c.config.Server.ListenAddr
		}
		if certFile == "" && c.config.Server.TLS.CertPath != "" {
			certFile = c.config.Server.TLS.CertPath
		}
		if keyFile == "" && c.config.Server.TLS.KeyPath != "" {
			keyFile = c.config.Server.TLS.KeyPath
		}
		if caFile == "./certs/ca.crt" && c.config.Server.TLS.CAPath != "" {
			caFile = c.config.Server.TLS.CAPath
		}
	}

	if certFile == "" || keyFile == "" {
		return fmt.Errorf("client certificate required (MANDAU_CERT, MANDAU_KEY)")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load cert: %w", err)
	}

	// Load CA certificate to verify server certificate
	caCert, err := ioutil.ReadFile(caFile)
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
