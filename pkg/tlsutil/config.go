// Package tlsutil provides helper functions for TLS configuration
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// Config holds TLS configuration
type Config struct {
	CertPath   string
	KeyPath    string
	CAPath     string
	MinVersion uint16
	ServerName string
	ClientAuth tls.ClientAuthType
}

// LoadServerConfig loads TLS configuration for servers
func LoadServerConfig(cfg Config) (*tls.Config, error) {
	if cfg.CertPath == "" || cfg.KeyPath == "" {
		return nil, fmt.Errorf("certificate and key paths are required")
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("load certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   cfg.MinVersion,
		ServerName:   cfg.ServerName,
		ClientAuth:   cfg.ClientAuth,
	}

	// Load CA certificate if specified
	if cfg.CAPath != "" {
		caCertPool, err := LoadCACertPool(cfg.CAPath)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = caCertPool
		tlsConfig.ClientCAs = caCertPool
	}

	return tlsConfig, nil
}

// LoadClientConfig loads TLS configuration for clients
func LoadClientConfig(cfg Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: cfg.MinVersion,
		ServerName: cfg.ServerName,
	}

	// Load client certificate if specified
	if cfg.CertPath != "" && cfg.KeyPath != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate if specified
	if cfg.CAPath != "" {
		caCertPool, err := LoadCACertPool(cfg.CAPath)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// LoadCACertPool loads a CA certificate pool from a file
func LoadCACertPool(caPath string) (*x509.CertPool, error) {
	caCert, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return caCertPool, nil
}

// DefaultServerConfig returns a secure default server TLS config
func DefaultServerConfig(certPath, keyPath, caPath string) (*tls.Config, error) {
	return LoadServerConfig(Config{
		CertPath:   certPath,
		KeyPath:    keyPath,
		CAPath:     caPath,
		MinVersion: tls.VersionTLS13,
		ClientAuth: tls.RequireAndVerifyClientCert,
	})
}

// DefaultClientConfig returns a secure default client TLS config
func DefaultClientConfig(certPath, keyPath, caPath, serverName string) (*tls.Config, error) {
	return LoadClientConfig(Config{
		CertPath:   certPath,
		KeyPath:    keyPath,
		CAPath:     caPath,
		MinVersion: tls.VersionTLS13,
		ServerName: serverName,
	})
}

// TLSVersionFromString converts a string to TLS version
func TLSVersionFromString(s string) uint16 {
	switch s {
	case "TLS1.0":
		return tls.VersionTLS10
	case "TLS1.1":
		return tls.VersionTLS11
	case "TLS1.2":
		return tls.VersionTLS12
	case "TLS1.3":
		return tls.VersionTLS13
	default:
		return tls.VersionTLS13
	}
}
