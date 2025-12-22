package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/bhangun/mandau/pkg/core"
)


func main() {
	certPath := flag.String("cert", "certs/core.crt", "Certificate path")
	keyPath := flag.String("key", "certs/core.key", "Key path")
	caPath := flag.String("ca", "certs/ca.crt", "CA certificate path")
	listenAddr := flag.String("listen", ":8443", "Listen address (e.g., :8443)")
	flag.Parse()

	fmt.Printf("Starting Mandau Core on %s...\n", *listenAddr)

	// Create and configure the Core service
	coreConfig := &core.CoreConfig{
		ListenAddr: *listenAddr,
		CertPath:   *certPath,
		KeyPath:    *keyPath,
		CAPath:     *caPath,
		PluginDir:  "/usr/lib/mandau/plugins", // Default plugin directory
	}

	mandauCore, err := core.NewCore(coreConfig)
	if err != nil {
		log.Fatalf("failed to create core: %v", err)
	}

	// Start the Core service (this handles gRPC server setup internally)
	if err := mandauCore.Serve(); err != nil {
		log.Fatalf("failed to serve core: %v", err)
	}
}
