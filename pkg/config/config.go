package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// CoreConfig represents the configuration for the core server
type CoreConfig struct {
	Server           ServerConfig           `yaml:"server"`
	Plugins          PluginConfig           `yaml:"plugins"`
	AgentManagement  AgentManagementConfig  `yaml:"agent_management"`
	PluginDir        string                 `yaml:"plugin_dir"`
}

// AgentConfig represents the configuration for the agent
type AgentConfig struct {
	Agent            AgentInfoConfig        `yaml:"agent"`
	Server           ServerConfig           `yaml:"server"`
	ServerConnection ServerConnectionConfig `yaml:"server_connection"`
	Docker           DockerConfig           `yaml:"docker"`
	Stacks           StacksConfig           `yaml:"stacks"`
	Plugins          PluginConfig           `yaml:"plugins"`
	Security         SecurityConfig         `yaml:"security"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	ListenAddr string    `yaml:"listen_addr"`
	TLS        TLSConfig `yaml:"tls"`
}

// ServerConnectionConfig contains connection configuration to the core server
type ServerConnectionConfig struct {
	CoreAddr string    `yaml:"core_addr"`
	TLS      TLSConfig `yaml:"tls"`
}

// TLSConfig contains TLS-related configuration
type TLSConfig struct {
	CertPath   string `yaml:"cert_path"`
	KeyPath    string `yaml:"key_path"`
	CAPath     string `yaml:"ca_path"`
	MinVersion string `yaml:"min_version"`
	ServerName string `yaml:"server_name"`
}

// AgentInfoConfig contains agent identification configuration
type AgentInfoConfig struct {
	ID       string            `yaml:"id"`
	Hostname string            `yaml:"hostname"`
	Labels   map[string]string `yaml:"labels"`
}

// DockerConfig contains Docker-related configuration
type DockerConfig struct {
	Socket     string `yaml:"socket"`
	APIVersion string `yaml:"api_version"`
}

// StacksConfig contains stack-related configuration
type StacksConfig struct {
	RootDir                  string `yaml:"root_dir"`
	MaxConcurrentOperations  int    `yaml:"max_concurrent_operations"`
}

// PluginConfig contains plugin-related configuration
type PluginConfig struct {
	Enabled map[string]bool                `yaml:"enabled"`
	Configs map[string]map[string]interface{} `yaml:"configs,omitempty"`
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	ExecTimeout         string `yaml:"exec_timeout"`
	LogRetention        string `yaml:"log_retention"`
	TerminalRecording   bool   `yaml:"terminal_recording"`
}

// AgentManagementConfig contains agent management configuration
type AgentManagementConfig struct {
	HeartbeatInterval string `yaml:"heartbeat_interval"`
	OfflineTimeout    string `yaml:"offline_timeout"`
	AutoDeregister    bool   `yaml:"auto_deregister"`
}

// LoadCoreConfig loads the core server configuration from a YAML file
func LoadCoreConfig(configPath string) (*CoreConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config CoreConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Set defaults if not provided
	if config.PluginDir == "" {
		config.PluginDir = "/usr/lib/mandau/plugins"
	}

	return &config, nil
}

// LoadAgentConfig loads the agent configuration from a YAML file
func LoadAgentConfig(configPath string) (*AgentConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config AgentConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ParseDuration is a helper function to parse duration strings
func ParseDuration(durationStr string) (time.Duration, error) {
	return time.ParseDuration(durationStr)
}

// GetConfigPath returns the appropriate config path based on environment variables or defaults
func GetConfigPath(defaultPath string) string {
	configPath := os.Getenv("MANDAU_CONFIG_PATH")
	if configPath != "" {
		return configPath
	}
	return defaultPath
}

// CreateDefaultCoreConfig creates a default core configuration
func CreateDefaultCoreConfig() *CoreConfig {
	// Create default RBAC configuration
	rbacConfig := map[string]interface{}{
		"roles": `roles:
  - name: admin
    permissions:
      - resource: "*"
        actions: ["*"]
  - name: operator
    permissions:
      - resource: "stack:*"
        actions: ["read", "write", "delete"]
      - resource: "container:*"
        actions: ["read", "exec", "logs"]
      - resource: "image:*"
        actions: ["read", "pull"]
      - resource: "file:*"
        actions: ["read", "write"]
  - name: viewer
    permissions:
      - resource: "*"
        actions: ["read", "logs"]
users:
  - id: "admin@example.com"
    name: "Administrator"
    roles: ["admin"]
  - id: "ops@example.com"
    name: "Operations Team"
    roles: ["operator"]
  - id: "mandau-agent"
    name: "Agent User"
    roles: ["admin"]  # Agent needs admin rights to manage stacks on its host
  - id: "mandau-cli"
    name: "CLI User"
    roles: ["admin"]`,
	}

	return &CoreConfig{
		Server: ServerConfig{
			ListenAddr: ":8443",
			TLS: TLSConfig{
				CertPath:   "certs/core.crt",
				KeyPath:    "certs/core.key",
				CAPath:     "certs/ca.crt",
				MinVersion: "TLS1.3",
				ServerName: "mandau-core",
			},
		},
		PluginDir: "/usr/lib/mandau/plugins",
		AgentManagement: AgentManagementConfig{
			HeartbeatInterval: "30s",
			OfflineTimeout:    "90s",
			AutoDeregister:    false,
		},
		Plugins: PluginConfig{
			Enabled: map[string]bool{
				"rbac-auth": true,
			},
			Configs: map[string]map[string]interface{}{
				"rbac-auth": rbacConfig,
			},
		},
	}
}

// CreateDefaultAgentConfig creates a default agent configuration
func CreateDefaultAgentConfig() *AgentConfig {
	// Create default RBAC configuration for agent
	rbacConfig := map[string]interface{}{
		"roles": `roles:
  - name: admin
    permissions:
      - resource: "*"
        actions: ["*"]
  - name: operator
    permissions:
      - resource: "stack:*"
        actions: ["read", "write", "delete"]
      - resource: "container:*"
        actions: ["read", "exec", "logs"]
      - resource: "image:*"
        actions: ["read", "pull"]
      - resource: "file:*"
        actions: ["read", "write"]
  - name: viewer
    permissions:
      - resource: "*"
        actions: ["read", "logs"]
users:
  - id: "mandau-core"
    name: "Core Server"
    roles: ["admin"]  # Core server has admin privileges to forward requests
  - id: "mandau-cli"
    name: "CLI User"
    roles: ["admin"]
  - id: "admin@example.com"
    name: "Administrator"
    roles: ["admin"]
  - id: "ops@example.com"
    name: "Operations Team"
    roles: ["operator"]`,
	}

	return &AgentConfig{
		Server: ServerConfig{
			ListenAddr: ":8444", // Default agent listen port
			TLS: TLSConfig{
				CertPath:   "certs/agent.crt",
				KeyPath:    "certs/agent.key",
				CAPath:     "certs/ca.crt",
				MinVersion: "TLS1.3",
				ServerName: "mandau-agent",
			},
		},
		ServerConnection: ServerConnectionConfig{
			CoreAddr: "localhost:8443",
			TLS: TLSConfig{
				CertPath:   "certs/agent.crt",
				KeyPath:    "certs/agent.key",
				CAPath:     "certs/ca.crt",
				MinVersion: "TLS1.3",
				ServerName: "mandau-core",
			},
		},
		Docker: DockerConfig{
			Socket:     "/var/run/docker.sock",
			APIVersion: "1.41",
		},
		Stacks: StacksConfig{
			RootDir:                 "/var/lib/mandau/stacks",
			MaxConcurrentOperations: 5,
		},
		Security: SecurityConfig{
			ExecTimeout:       "1h",
			LogRetention:      "30d",
			TerminalRecording: true,
		},
		Plugins: PluginConfig{
			Enabled: map[string]bool{
				"rbac-auth": true,
			},
			Configs: map[string]map[string]interface{}{
				"rbac-auth": rbacConfig,
			},
		},
	}
}