package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/bhangun/mandau/pkg/config"
	"github.com/bhangun/mandau/pkg/core"
)


func main() {
	// Configuration file path
	configPath := flag.String("config", "config/core/config.yaml", "Configuration file path")

	// Command-line flags (for backward compatibility, but config file takes precedence)
	certPath := flag.String("cert", "", "Certificate path (overrides config file)")
	keyPath := flag.String("key", "", "Key path (overrides config file)")
	caPath := flag.String("ca", "", "CA certificate path (overrides config file)")
	listenAddr := flag.String("listen", "", "Listen address (overrides config file)")
	pluginDir := flag.String("plugin-dir", "", "Plugin directory (overrides config file)")

	flag.Parse()

	// Load configuration from file first
	coreConfig := &core.CoreConfig{}

	// Load from config file
	cfg, err := config.LoadCoreConfig(*configPath)
	if err != nil {
		log.Printf("Config file not found at %s, using defaults: %v", *configPath, err)
		cfg = config.CreateDefaultCoreConfig()
	} else {
		log.Printf("Loaded configuration from %s", *configPath)
	}

	// Apply config file values as defaults
	coreConfig.ListenAddr = cfg.Server.ListenAddr
	coreConfig.CertPath = cfg.Server.TLS.CertPath
	coreConfig.KeyPath = cfg.Server.TLS.KeyPath
	coreConfig.CAPath = cfg.Server.TLS.CAPath
	coreConfig.PluginDir = cfg.PluginDir
	coreConfig.FullConfig = cfg

	// Override with command-line flags if provided
	if *listenAddr != "" {
		coreConfig.ListenAddr = *listenAddr
	}
	if *certPath != "" {
		coreConfig.CertPath = *certPath
	}
	if *keyPath != "" {
		coreConfig.KeyPath = *keyPath
	}
	if *caPath != "" {
		coreConfig.CAPath = *caPath
	}
	if *pluginDir != "" {
		coreConfig.PluginDir = *pluginDir
	}

	// Validate required paths exist
	if _, err := os.Stat(coreConfig.CertPath); os.IsNotExist(err) {
		log.Fatalf("Certificate file does not exist: %s", coreConfig.CertPath)
	}
	if _, err := os.Stat(coreConfig.KeyPath); os.IsNotExist(err) {
		log.Fatalf("Key file does not exist: %s", coreConfig.KeyPath)
	}
	if _, err := os.Stat(coreConfig.CAPath); os.IsNotExist(err) {
		log.Fatalf("CA certificate file does not exist: %s", coreConfig.CAPath)
	}

	fmt.Printf("Starting Mandau Core on %s...\n", coreConfig.ListenAddr)

	// Create and configure the Core service
	mandauCore, err := core.NewCore(coreConfig)
	if err != nil {
		log.Fatalf("failed to create core: %v", err)
	}

	// Start the Core service (this handles gRPC server setup internally)
	if err := mandauCore.Serve(); err != nil {
		log.Fatalf("failed to serve core: %v", err)
	}
}
