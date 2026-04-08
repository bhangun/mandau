package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	certDir        string
	caDir          string
	coreHostname   string
	coreIP         string
	agentHostname  string
	agentIP        string
	caDays         int
	certDays        int
	keySize        int
	profile        string
)

func init() {
	certCmd := &cobra.Command{
		Use:   "cert",
		Short: "Certificate management",
		Long:  `Generate, verify, and manage mTLS certificates for Mandau components.`,
	}

	// Generate certificates
	genCmd := &cobra.Command{
		Use:   "gen [component]",
		Short: "Generate certificates",
		Long: `Generate mTLS certificates for Mandau components.

Component can be:
  - (empty)    : Generate all certificates (CA + core + agent + client)
  - ca         : Generate only CA certificate
  - core       : Generate core server certificate (requires existing CA)
  - agent      : Generate agent certificate (requires existing CA)
  - client     : Generate CLI client certificate (requires existing CA)

Examples:
  # Generate all certificates for development
  mandau cert gen

  # Generate all certificates with custom hostnames
  mandau cert gen --core-hostname mandau-core.example.com --core-ip 192.168.1.100

  # Generate only agent certificate for remote server
  mandau cert gen agent --agent-hostname agent-us-east-1 --agent-ip 192.168.1.101

  # Generate CA only on secure admin machine
  mandau cert gen ca --ca-dir ~/.mandau/ca
`,
		Args: cobra.MaximumNArgs(1),
		RunE: runCertGen,
	}

	genCmd.Flags().StringVar(&certDir, "cert-dir", "", "Certificate output directory (default: ~/.mandau/certs)")
	genCmd.Flags().StringVar(&caDir, "ca-dir", "", "CA directory (contains ca.key and ca.crt)")
	genCmd.Flags().StringVar(&coreHostname, "core-hostname", "localhost", "Core server hostname")
	genCmd.Flags().StringVar(&coreIP, "core-ip", "127.0.0.1", "Core server IP address")
	genCmd.Flags().StringVar(&agentHostname, "agent-hostname", "localhost", "Agent server hostname")
	genCmd.Flags().StringVar(&agentIP, "agent-ip", "127.0.0.1", "Agent server IP address")
	genCmd.Flags().IntVar(&caDays, "ca-days", 3650, "CA certificate validity in days")
	genCmd.Flags().IntVar(&certDays, "cert-days", 365, "Server/client certificate validity in days")
	genCmd.Flags().IntVar(&keySize, "key-size", 4096, "RSA key size in bits")
	genCmd.Flags().StringVarP(&profile, "profile", "p", "", "Configuration profile (dev, prod, etc.)")

	certCmd.AddCommand(genCmd)

	// Certificate status
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show certificate status",
		Long:  `Display certificate information including expiry dates and SANs.`,
		RunE:  runCertStatus,
	}

	statusCmd.Flags().StringVar(&certDir, "cert-dir", "", "Certificate directory (default: ~/.mandau/certs)")

	certCmd.AddCommand(statusCmd)

	// Verify certificates
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify certificates",
		Long:  `Verify that certificates are properly signed by the CA and valid.`,
		RunE:  runCertVerify,
	}

	verifyCmd.Flags().StringVar(&certDir, "cert-dir", "", "Certificate directory (default: ~/.mandau/certs)")

	certCmd.AddCommand(verifyCmd)

	// Rotate certificates
	rotateCmd := &cobra.Command{
		Use:   "rotate [component]",
		Short: "Rotate certificates",
		Long: `Rotate certificates by generating new ones with the same CA.

Component can be:
  - (empty) : Rotate all certificates
  - core    : Rotate only core server certificate
  - agent   : Rotate only agent certificate
  - client  : Rotate only CLI client certificate

Examples:
  # Rotate all certificates
  mandau cert rotate

  # Rotate only agent certificate
  mandau cert rotate agent
`,
		Args: cobra.MaximumNArgs(1),
		RunE: runCertRotate,
	}

	rotateCmd.Flags().StringVar(&certDir, "cert-dir", "", "Certificate directory (default: ~/.mandau/certs)")
	rotateCmd.Flags().StringVar(&caDir, "ca-dir", "", "CA directory (default: ~/.mandau/ca)")
	rotateCmd.Flags().StringVar(&coreHostname, "core-hostname", "localhost", "Core server hostname")
	rotateCmd.Flags().StringVar(&coreIP, "core-ip", "127.0.0.1", "Core server IP address")
	rotateCmd.Flags().StringVar(&agentHostname, "agent-hostname", "localhost", "Agent server hostname")
	rotateCmd.Flags().StringVar(&agentIP, "agent-ip", "127.0.0.1", "Agent server IP address")
	rotateCmd.Flags().IntVar(&certDays, "cert-days", 365, "New certificate validity in days")
	rotateCmd.Flags().IntVar(&keySize, "key-size", 4096, "RSA key size in bits")

	certCmd.AddCommand(rotateCmd)

	// Migrate certificates from old locations
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate certificates from old locations to ~/.mandau/",
		Long: `Migrate certificates from legacy locations to the standard ~/.mandau/ directory.

This command searches for certificates in common locations and moves them to ~/.mandau/certs/.

Legacy locations:
  - ./certs/ (relative to current directory)
  - ~/mandau-certs/
  - /etc/mandau/certs/

Examples:
  # Migrate from default legacy locations
  mandau cert migrate

  # Migrate from custom location
  mandau cert migrate --from /custom/cert/path
`,
		RunE: runCertMigrate,
	}

	migrateCmd.Flags().StringVar(&certDir, "from", "", "Source directory to migrate from")
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be migrated without moving files")

	certCmd.AddCommand(migrateCmd)

	rootCmd.AddCommand(certCmd)
}

var dryRun bool

// getDefaultCertDir returns the default certificate directory (~/.mandau/certs)
func getDefaultCertDir() string {
	if certDir != "" {
		return certDir
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./certs"
	}
	return filepath.Join(homeDir, ".mandau", "certs")
}

// getDefaultCADir returns the default CA directory (~/.mandau/ca)
func getDefaultCADir() string {
	if caDir != "" {
		return caDir
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./ca"
	}
	return filepath.Join(homeDir, ".mandau", "ca")
}

// runCertGen generates certificates based on command arguments
func runCertGen(cmd *cobra.Command, args []string) error {
	component := ""
	if len(args) > 0 {
		component = args[0]
	}

	// Apply profile if specified
	if err := applyProfile(profile); err != nil {
		return err
	}

	certDir := getDefaultCertDir()
	caDir := getDefaultCADir()

	// Create directories
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}
	if err := os.MkdirAll(caDir, 0700); err != nil {
		return fmt.Errorf("create CA dir: %w", err)
	}

	switch component {
	case "":
		// Generate all certificates
		return generateAllCerts(certDir, caDir)
	case "ca":
		// Generate only CA
		return generateCA(caDir)
	case "core":
		// Generate core certificate (requires CA)
		return generateCoreCert(certDir, caDir)
	case "agent":
		// Generate agent certificate (requires CA)
		return generateAgentCert(certDir, caDir)
	case "client":
		// Generate client certificate (requires CA)
		return generateClientCert(certDir, caDir)
	default:
		return fmt.Errorf("unknown component: %s (valid: ca, core, agent, client)", component)
	}
}

// generateAllCerts generates all certificates (CA + core + agent + client)
func generateAllCerts(certDir, caDir string) error {
	fmt.Println("Generating Mandau certificates...")
	fmt.Println()

	// Step 1: Generate CA
	if err := generateCA(caDir); err != nil {
		return err
	}

	// Copy CA cert to cert dir for convenience
	caCertPath := filepath.Join(caDir, "ca.crt")
	caCertDest := filepath.Join(certDir, "ca.crt")
	if err := copyFile(caCertPath, caCertDest); err != nil {
		return fmt.Errorf("copy CA cert: %w", err)
	}

	// Step 2: Generate core certificate
	if err := generateCoreCert(certDir, caDir); err != nil {
		return err
	}

	// Step 3: Generate agent certificate
	if err := generateAgentCert(certDir, caDir); err != nil {
		return err
	}

	// Step 4: Generate client certificate
	if err := generateClientCert(certDir, caDir); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✓ All certificates generated successfully!")
	fmt.Println()
	printCertSummary(certDir, caDir)
	fmt.Println()
	fmt.Println("⚠  SECURITY NOTE: Keep ca.key secure! It can sign new certificates.")
	fmt.Println("   Location:", caDir)

	return nil
}

// generateCA generates a Certificate Authority
func generateCA(caDir string) error {
	fmt.Println("→ Generating CA certificate...")

	// Generate CA private key
	caKeyPath := filepath.Join(caDir, "ca.key")
	caKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return fmt.Errorf("generate CA key: %w", err)
	}

	caKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caKey),
	})

	if err := os.WriteFile(caKeyPath, caKeyPEM, 0600); err != nil {
		return fmt.Errorf("write CA key: %w", err)
	}

	// Generate CA certificate
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Mandau CA",
			Organization: []string{"Mandau"},
			Country:      []string{"US"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, caDays),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create CA cert: %w", err)
	}

	caCertPath := filepath.Join(caDir, "ca.crt")
	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertDER,
	})

	if err := os.WriteFile(caCertPath, caCertPEM, 0644); err != nil {
		return fmt.Errorf("write CA cert: %w", err)
	}

	fmt.Println("  ✓ CA certificate generated:", caCertPath)
	return nil
}

// generateCoreCert generates a core server certificate
func generateCoreCert(certDir, caDir string) error {
	fmt.Println("→ Generating core server certificate...")
	fmt.Printf("  Hostname: %s, IP: %s\n", coreHostname, coreIP)

	return generateCertWithOpenSSL(certDir, caDir, "core", coreHostname, coreIP)
}

// generateAgentCert generates an agent certificate
func generateAgentCert(certDir, caDir string) error {
	fmt.Println("→ Generating agent certificate...")
	fmt.Printf("  Hostname: %s, IP: %s\n", agentHostname, agentIP)

	return generateCertWithOpenSSL(certDir, caDir, "agent", agentHostname, agentIP)
}

// generateClientCert generates a CLI client certificate
func generateClientCert(certDir, caDir string) error {
	fmt.Println("→ Generating CLI client certificate...")

	// Load CA
	caKey, caCert, err := loadCA(caDir)
	if err != nil {
		return fmt.Errorf("load CA: %w", err)
	}

	// Generate private key
	key, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   "mandau-cli",
			Organization: []string{"Mandau"},
			Country:      []string{"US"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, certDays),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// Sign certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create cert: %w", err)
	}

	// Write private key
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	keyPath := filepath.Join(certDir, "client.key")
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}

	// Write certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	certPath := filepath.Join(certDir, "client.crt")
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}

	fmt.Println("  ✓ Client certificate generated:", certPath)
	return nil
}

// loadCA loads the CA key and certificate from the specified directory
func loadCA(caDir string) (*rsa.PrivateKey, *x509.Certificate, error) {
	// Load CA key
	caKeyPEM, err := os.ReadFile(filepath.Join(caDir, "ca.key"))
	if err != nil {
		return nil, nil, fmt.Errorf("read CA key: %w", err)
	}

	block, _ := pem.Decode(caKeyPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("decode CA key")
	}

	caKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA key: %w", err)
	}

	// Load CA cert
	caCertPEM, err := os.ReadFile(filepath.Join(caDir, "ca.crt"))
	if err != nil {
		return nil, nil, fmt.Errorf("read CA cert: %w", err)
	}

	block, _ = pem.Decode(caCertPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("decode CA cert")
	}

	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA cert: %w", err)
	}

	return caKey, caCert, nil
}

// generateCertWithOpenSSL uses Go's crypto package to generate a certificate
func generateCertWithOpenSSL(certDir, caDir, component, hostname, ip string) error {
	// Load CA
	caKey, caCert, err := loadCA(caDir)
	if err != nil {
		return fmt.Errorf("load CA: %w", err)
	}

	// Generate private key
	key, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	// Parse IP address
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("mandau-%s", component),
			Organization: []string{"Mandau"},
			Country:      []string{"US"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, certDays),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:    []string{hostname, "localhost", fmt.Sprintf("mandau-%s", component)},
		IPAddresses: []net.IP{ipAddr, net.ParseIP("127.0.0.1")},
	}

	// Sign certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create cert: %w", err)
	}

	// Write private key
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	keyPath := filepath.Join(certDir, fmt.Sprintf("%s.key", component))
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}

	// Write certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	certPath := filepath.Join(certDir, fmt.Sprintf("%s.crt", component))
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}

	fmt.Printf("  ✓ %s certificate generated: %s\n", component, certPath)
	return nil
}

// runCertStatus displays certificate information
func runCertStatus(cmd *cobra.Command, args []string) error {
	certDir := getDefaultCertDir()

	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		return fmt.Errorf("certificate directory not found: %s", certDir)
	}

	fmt.Println("Mandau Certificate Status")
	fmt.Println("=========================")
	fmt.Println()

	certFiles := map[string]string{
		"CA":     "ca.crt",
		"Core":   "core.crt",
		"Agent":  "agent.crt",
		"Client": "client.crt",
	}

	for name, certFile := range certFiles {
		certPath := filepath.Join(certDir, certFile)
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			fmt.Printf("%s: Not found\n", name)
			continue
		}

		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			fmt.Printf("%s: Error reading: %v\n", name, err)
			continue
		}

		block, _ := pem.Decode(certPEM)
		if block == nil {
			fmt.Printf("%s: Error decoding\n", name)
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			fmt.Printf("%s: Error parsing: %v\n", name, err)
			continue
		}

		// Calculate days until expiry
		daysUntilExpiry := int(time.Until(cert.NotAfter).Hours() / 24)

		fmt.Printf("%s Certificate:\n", name)
		fmt.Printf("  Subject: %s\n", cert.Subject.CommonName)
		fmt.Printf("  Issuer: %s\n", cert.Issuer.CommonName)
		fmt.Printf("  Valid From: %s\n", cert.NotBefore.Format("2006-01-02"))
		fmt.Printf("  Valid Until: %s (%d days)\n", cert.NotAfter.Format("2006-01-02"), daysUntilExpiry)

		if len(cert.DNSNames) > 0 {
			fmt.Printf("  DNS Names: %v\n", cert.DNSNames)
		}

		if daysUntilExpiry < 30 {
			fmt.Printf("  ⚠  WARNING: Certificate expires in less than 30 days!\n")
		}

		fmt.Println()
	}

	return nil
}

// runCertVerify verifies certificates
func runCertVerify(cmd *cobra.Command, args []string) error {
	certDir := getDefaultCertDir()
	caDir := getDefaultCADir()

	fmt.Println("Verifying Mandau certificates...")
	fmt.Println()

	// Load CA
	_, caCert, err := loadCA(caDir)
	if err != nil {
		return fmt.Errorf("load CA: %w", err)
	}

	certFiles := map[string]string{
		"Core":   "core.crt",
		"Agent":  "agent.crt",
		"Client": "client.crt",
	}

	allValid := true
	for name, certFile := range certFiles {
		certPath := filepath.Join(certDir, certFile)
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			fmt.Printf("%s: Not found\n", name)
			allValid = false
			continue
		}

		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			fmt.Printf("%s: Error reading: %v\n", name, err)
			allValid = false
			continue
		}

		block, _ := pem.Decode(certPEM)
		if block == nil {
			fmt.Printf("%s: Error decoding\n", name)
			allValid = false
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			fmt.Printf("%s: Error parsing: %v\n", name, err)
			allValid = false
			continue
		}

		// Verify certificate against CA
		opts := x509.VerifyOptions{
			Roots: x509.NewCertPool(),
		}
		opts.Roots.AddCert(caCert)

		_, err = cert.Verify(opts)
		if err != nil {
			fmt.Printf("%s: ✗ Invalid - %v\n", name, err)
			allValid = false
		} else {
			fmt.Printf("%s: ✓ Valid (signed by %s)\n", name, cert.Issuer.CommonName)
		}
	}

	fmt.Println()
	if allValid {
		fmt.Println("✓ All certificates are valid")
	} else {
		fmt.Println("✗ Some certificates are invalid")
		return fmt.Errorf("certificate verification failed")
	}

	return nil
}

// runCertRotate rotates certificates
func runCertRotate(cmd *cobra.Command, args []string) error {
	component := ""
	if len(args) > 0 {
		component = args[0]
	}

	certDir := getDefaultCertDir()
	caDir := getDefaultCADir()

	fmt.Println("Rotating certificates...")
	fmt.Println()

	// Verify CA exists
	if _, err := os.Stat(filepath.Join(caDir, "ca.key")); os.IsNotExist(err) {
		return fmt.Errorf("CA key not found in %s. Cannot rotate without CA.", caDir)
	}

	switch component {
	case "":
		// Rotate all
		if err := generateCoreCert(certDir, caDir); err != nil {
			return err
		}
		if err := generateAgentCert(certDir, caDir); err != nil {
			return err
		}
		if err := generateClientCert(certDir, caDir); err != nil {
			return err
		}
		fmt.Println()
		fmt.Println("✓ All certificates rotated successfully!")
	case "core":
		if err := generateCoreCert(certDir, caDir); err != nil {
			return err
		}
		fmt.Println("✓ Core certificate rotated!")
	case "agent":
		if err := generateAgentCert(certDir, caDir); err != nil {
			return err
		}
		fmt.Println("✓ Agent certificate rotated!")
	case "client":
		if err := generateClientCert(certDir, caDir); err != nil {
			return err
		}
		fmt.Println("✓ Client certificate rotated!")
	default:
		return fmt.Errorf("unknown component: %s", component)
	}

	return nil
}

// runCertMigrate migrates certificates from old locations
func runCertMigrate(cmd *cobra.Command, args []string) error {
	targetDir := getDefaultCertDir()

	// Determine source directory
	sourceDir := ""
	if certDir != "" {
		sourceDir = certDir
	} else {
		// Search common legacy locations
		legacyLocations := []string{
			"./certs",
			"certs",
		}

		homeDir, err := os.UserHomeDir()
		if err == nil {
			legacyLocations = append(legacyLocations, filepath.Join(homeDir, "mandau-certs"))
		}

		// Find first location that exists
		for _, loc := range legacyLocations {
			if _, err := os.Stat(filepath.Join(loc, "ca.crt")); err == nil {
				sourceDir = loc
				break
			}
		}

		if sourceDir == "" {
			return fmt.Errorf("no certificates found in legacy locations")
		}
	}

	fmt.Printf("Migrating certificates from %s to %s\n", sourceDir, targetDir)
	fmt.Println()

	// Create target directory
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	// List files to migrate
	certFiles := []string{
		"ca.crt", "ca.key",
		"core.crt", "core.key",
		"agent.crt", "agent.key",
		"client.crt", "client.key",
		"config.yaml",
	}

	if dryRun {
		fmt.Println("Dry run - would migrate:")
		for _, file := range certFiles {
			src := filepath.Join(sourceDir, file)
			if _, err := os.Stat(src); err == nil {
				fmt.Printf("  %s -> %s\n", file, targetDir)
			}
		}
		return nil
	}

	// Migrate files
	migrated := 0
	for _, file := range certFiles {
		src := filepath.Join(sourceDir, file)
		dst := filepath.Join(targetDir, file)

		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}

		if err := copyFile(src, dst); err != nil {
			fmt.Printf("  ✗ Failed to copy %s: %v\n", file, err)
			continue
		}

		fmt.Printf("  ✓ Migrated %s\n", file)
		migrated++
	}

	fmt.Println()
	if migrated > 0 {
		fmt.Printf("✓ Migrated %d files to %s\n", migrated, targetDir)
		fmt.Println()
		fmt.Println("Note: Old certificates still exist in", sourceDir)
		fmt.Println("      You can safely delete them after verifying the migration.")
	} else {
		fmt.Println("No files to migrate")
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Get file permissions
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, info.Mode())
}

// printCertSummary prints a summary of generated certificates
func printCertSummary(certDir, caDir string) {
	fmt.Println("Certificate Summary:")
	fmt.Println("  CA:", filepath.Join(caDir, "ca.crt"))
	fmt.Println("  Core:", filepath.Join(certDir, "core.crt"))
	fmt.Println("  Agent:", filepath.Join(certDir, "agent.crt"))
	fmt.Println("  Client:", filepath.Join(certDir, "client.crt"))
}

// applyProfile applies configuration profile settings
func applyProfile(profile string) error {
	switch profile {
	case "dev":
		// Development defaults
		if coreHostname == "localhost" {
			coreHostname = "localhost"
		}
		if coreIP == "127.0.0.1" {
			coreIP = "127.0.0.1"
		}
	case "prod":
		// Production - require explicit hostnames
		if coreHostname == "localhost" && coreIP == "127.0.0.1" {
			fmt.Println("⚠  WARNING: Using localhost for production profile!")
			fmt.Println("   Consider specifying --core-hostname and --core-ip")
		}
	case "":
		// No profile specified, use defaults
	default:
		return fmt.Errorf("unknown profile: %s", profile)
	}
	return nil
}
