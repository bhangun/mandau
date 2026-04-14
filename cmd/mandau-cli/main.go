package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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
		Use:     "mandau",
		Short:   "Mandau infrastructure control CLI",
		Version: version, // Add version flag
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// setup logging based on flags/env then connect
			if err := setupLogging(cmd); err != nil {
				fmt.Printf("Warning: failed to initialize logger: %v\n", err)
			}
			return cli.connect(cmd)
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if cli.conn != nil {
				_ = cli.conn.Close()
			}
			// Close rotating writer if open
			if RotWriter != nil {
				_ = RotWriter.Close()
			}
			return nil
		},
	}

	agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "Agent management",
	}

	stackCmd = &cobra.Command{
		Use:   "stack",
		Short: "Stack management",
	}

	AppLog    *log.Logger
	RotWriter *rotatingWriter
	LogLevel  int
)

const (
	LevelDebug = iota
	LevelInfo
	LevelWarn
	LevelError
)

func parseLogLevel(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func Log(levelStr, format string, v ...interface{}) {
	if AppLog == nil {
		return
	}
	lvl := parseLogLevel(levelStr)
	if lvl < LogLevel {
		return
	}
	prefix := strings.ToUpper(levelStr)
	AppLog.Printf("[%s] %s", prefix, fmt.Sprintf(format, v...))
}

type CLI struct {
	coreClient  v1.CoreServiceClient
	agentClient v1.AgentServiceClient
	conn        *grpc.ClientConn
	config      *config.CoreConfig // For CLI, we can reuse the core config structure
}

// rotatingWriter handles log rotation by size and/or daily.
type rotatingWriter struct {
	path        string
	mu          sync.Mutex
	file        *os.File
	mode        string // "daily", "size", "both", "none"
	maxSize     int64  // bytes
	currentDate string
}

func newRotatingWriter(path, mode string, maxSize int64) (*rotatingWriter, error) {
	rw := &rotatingWriter{path: path, mode: mode, maxSize: maxSize}
	if err := rw.openNew(); err != nil {
		return nil, err
	}
	return rw, nil
}

func (rw *rotatingWriter) openNew() error {
	f, err := os.OpenFile(rw.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	rw.file = f
	rw.currentDate = time.Now().Format("2006-01-02")
	return nil
}

func (rw *rotatingWriter) shouldRotate(n int) bool {
	if rw.mode == "none" {
		return false
	}
	now := time.Now()
	if rw.mode == "daily" || rw.mode == "both" {
		if rw.currentDate != now.Format("2006-01-02") {
			return true
		}
	}
	if rw.mode == "size" || rw.mode == "both" {
		if rw.file != nil {
			if fi, err := rw.file.Stat(); err == nil {
				if fi.Size()+int64(n) > rw.maxSize {
					return true
				}
			}
		}
	}
	return false
}

func (rw *rotatingWriter) rotate() error {
	// close current
	if rw.file != nil {
		_ = rw.file.Sync()
		_ = rw.file.Close()
		// rename
		ts := time.Now().Format("20060102-150405")
		newName := fmt.Sprintf("%s.%s", rw.path, ts)
		_ = os.Rename(rw.path, newName)
	}
	// open new
	return rw.openNew()
}

func (rw *rotatingWriter) Write(p []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.shouldRotate(len(p)) {
		if err := rw.rotate(); err != nil {
			// continue writing even if rotate fails
			fmt.Fprintf(os.Stderr, "rotate failed: %v\n", err)
		}
	}
	return rw.file.Write(p)
}

func (rw *rotatingWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.file != nil {
		_ = rw.file.Sync()
		err := rw.file.Close()
		rw.file = nil
		return err
	}
	return nil
}

func initAppLogger(mode string, sizeMB int64, retentionDays int, retainCount int) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	logsDir := filepath.Join(home, ".mandau", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(logsDir, "mandau.log")

	mode = strings.ToLower(mode)
	if mode == "" {
		mode = "daily"
	}
	maxSize := sizeMB * 1024 * 1024

	rw, err := newRotatingWriter(logPath, mode, maxSize)
	if err != nil {
		// fallback to simple file
		f, ferr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if ferr != nil {
			return ferr
		}
		AppLog = log.New(io.MultiWriter(os.Stdout, f), "mandau: ", log.LstdFlags|log.Lmicroseconds)
		AppLog.Printf("Application log started (rotation disabled, fallback)")
		return nil
	}
	RotWriter = rw
	AppLog = log.New(io.MultiWriter(os.Stdout, RotWriter), "mandau: ", log.LstdFlags|log.Lmicroseconds)
	AppLog.Printf("Application log started (rotate=%s sizeMB=%d)", mode, sizeMB)

	// Prune old logs according to retention settings
	if retentionDays > 0 || retainCount > 0 {
		pruneOldLogs(logPath, retentionDays, retainCount)
	}
	return nil
}

func pruneOldLogs(logPath string, retentionDays int, retainCount int) {
	dir := filepath.Dir(logPath)
	base := filepath.Base(logPath)
	files, err := os.ReadDir(dir)
	if err != nil {
		if AppLog != nil {
			AppLog.Printf("pruneOldLogs: read dir failed: %v", err)
		}
		return
	}
	var matches []os.DirEntry
	for _, f := range files {
		if f.Name() == base || strings.HasPrefix(f.Name(), base+".") {
			matches = append(matches, f)
		}
	}
	// sort by mod time descending
	type fi struct {
		name string
		mod  time.Time
	}
	var arr []fi
	for _, m := range matches {
		st, err := os.Stat(filepath.Join(dir, m.Name()))
		if err != nil {
			continue
		}
		arr = append(arr, fi{name: m.Name(), mod: st.ModTime()})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].mod.After(arr[j].mod) })

	// delete older than retentionDays
	if retentionDays > 0 {
		cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
		for _, it := range arr {
			if it.mod.Before(cutoff) {
				p := filepath.Join(dir, it.name)
				_ = os.Remove(p)
				if AppLog != nil {
					AppLog.Printf("pruneOldLogs: removed old log %s", p)
				}
			}
		}
	}
	// keep only retainCount most recent
	if retainCount > 0 && len(arr) > retainCount {
		for i := retainCount; i < len(arr); i++ {
			p := filepath.Join(dir, arr[i].name)
			_ = os.Remove(p)
			if AppLog != nil {
				AppLog.Printf("pruneOldLogs: removed excess log %s", p)
			}
		}
	}
}

func setupLogging(cmd *cobra.Command) error {
	// Flags take precedence over env
	mode, _ := cmd.Flags().GetString("log-rotate-mode")
	if mode == "" {
		mode = os.Getenv("MANDAU_LOG_ROTATE_MODE")
	}
	if mode == "" {
		mode = "daily"
	}

	sizeMBFlag, _ := cmd.Flags().GetInt("log-rotate-size-mb")
	var sizeMB int64
	if sizeMBFlag > 0 {
		sizeMB = int64(sizeMBFlag)
	} else if s := os.Getenv("MANDAU_LOG_ROTATE_SIZE_MB"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
			sizeMB = v
		}
	}
	if sizeMB == 0 {
		sizeMB = 10
	}

	retentionDays, _ := cmd.Flags().GetInt("log-retention-days")
	if retentionDays == 0 {
		if s := os.Getenv("MANDAU_LOG_RETENTION_DAYS"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				retentionDays = v
			}
		}
	}
	retainCount, _ := cmd.Flags().GetInt("log-retain-count")
	if retainCount == 0 {
		if s := os.Getenv("MANDAU_LOG_RETAIN_COUNT"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				retainCount = v
			}
		}
	}

	// determine log level
	lvl, _ := cmd.Flags().GetString("log-level")
	if lvl == "" {
		lvl = os.Getenv("MANDAU_LOG_LEVEL")
	}
	if lvl == "" {
		lvl = "info"
	}
	LogLevel = parseLogLevel(lvl)

	return initAppLogger(mode, sizeMB, retentionDays, retainCount)
}

func main() {
	// Global flags
	rootCmd.PersistentFlags().String("server", "localhost:3443", "Core server address")
	rootCmd.PersistentFlags().String("cert", "", "Client certificate")
	rootCmd.PersistentFlags().String("key", "", "Client key")
	rootCmd.PersistentFlags().String("ca", "", "CA certificate")
	rootCmd.PersistentFlags().StringP("agent", "a", "", "Target agent ID")
	// Logging flags
	rootCmd.PersistentFlags().String("log-rotate-mode", "", "Log rotation mode: daily, size, both, none (env MANDAU_LOG_ROTATE_MODE)")
	rootCmd.PersistentFlags().Int("log-rotate-size-mb", 0, "Rotation size in MB (env MANDAU_LOG_ROTATE_SIZE_MB)")
	rootCmd.PersistentFlags().Int("log-retention-days", 0, "Prune logs older than N days (env MANDAU_LOG_RETENTION_DAYS)")
	rootCmd.PersistentFlags().Int("log-retain-count", 0, "Keep at most N rotated log files (env MANDAU_LOG_RETAIN_COUNT)")
	rootCmd.PersistentFlags().String("log-level", "", "Log level: DEBUG, INFO, WARN, ERROR (env MANDAU_LOG_LEVEL)")

	// Agent commands

	agentCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all agents",
		RunE:  cli.listAgents,
	})

	// Stack commands

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

	rootCmd.AddCommand(authCmd, applyCmd, fsCmd, shellCmd)

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

	serverAddr, err := c.getFlagOrEnv(cmd, "server", "MANDAU_SERVER", "localhost:3443")
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

	conn, err := grpc.Dial(serverAddr,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "x509") || strings.Contains(errStr, "authentication handshake failed") {
			return fmt.Errorf("TLS connection failed: %v\n\n"+
				"💡 PRO-TIP: If you recently rotated certificates using 'mandau cert gen', \n"+
				"you MUST restart the core and agent services to apply changes:\n\n"+
				"  sudo systemctl restart mandau-core mandau-agent", err)
		}
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
	serverAddr, err := c.getFlagOrEnv(nil, "server", "MANDAU_SERVER", "localhost:3443")
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

func (c *CLI) resolveAgent(cmd *cobra.Command) (string, error) {
	// 1. Check flag
	agentID, _ := cmd.Flags().GetString("agent")
	if agentID != "" {
		return agentID, nil
	}

	// 2. Check default in config
	if c.config.DefaultAgent != "" {
		return c.config.DefaultAgent, nil
	}

	// 3. Fallback: Get first agent from list
	resp, err := c.coreClient.ListAgents(context.Background(), &v1.ListAgentsRequest{})
	if err == nil && len(resp.Agents) > 0 {
		firstAgent := resp.Agents[0].Id
		fmt.Printf("ℹ No agent specified, using first available: %s\n", firstAgent)

		// Auto-save as default
		c.config.DefaultAgent = firstAgent
		homeDir, _ := os.UserHomeDir()
		configPath := filepath.Join(homeDir, ".mandau", "config.yaml")
		_ = config.SaveCoreConfig(configPath, c.config)

		return firstAgent, nil
	}

	return "", fmt.Errorf("no agent specified and no agents available. Connect an agent first.")
}
