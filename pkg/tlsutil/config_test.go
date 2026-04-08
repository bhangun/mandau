package tlsutil

import (
	"crypto/tls"
	"testing"
)

func TestLoadCACertPoolNotFound(t *testing.T) {
	_, err := LoadCACertPool("/nonexistent/ca.pem")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestTLSVersionFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected uint16
	}{
		{"TLS1.0", tls.VersionTLS10},
		{"TLS1.1", tls.VersionTLS11},
		{"TLS1.2", tls.VersionTLS12},
		{"TLS1.3", tls.VersionTLS13},
		{"invalid", tls.VersionTLS13}, // defaults to 1.3
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := TLSVersionFromString(tt.input)
			if result != tt.expected {
				t.Errorf("TLSVersionFromString(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadServerConfigMissingCert(t *testing.T) {
	_, err := LoadServerConfig(Config{})
	if err == nil {
		t.Error("expected error for missing cert/key paths")
	}
}

func TestDefaultServerConfigMissingCert(t *testing.T) {
	_, err := DefaultServerConfig("", "", "")
	if err == nil {
		t.Error("expected error for missing cert/key paths")
	}
}

func TestDefaultClientConfigNoCert(t *testing.T) {
	// Client config without client certificates should work fine (no mTLS, just server validation)
	// But without CA it will still create a config (just won't validate server)
	cfg, err := DefaultClientConfig("", "", "", "test-server")
	if err != nil {
		t.Fatalf("DefaultClientConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("DefaultClientConfig returned nil")
	}
	if cfg.ServerName != "test-server" {
		t.Errorf("expected ServerName test-server, got %s", cfg.ServerName)
	}
}
