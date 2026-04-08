// Package transport provides a transport abstraction layer supporting
// both gRPC and WebSocket fallback for Mandau core-agent communication.
package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Type defines the transport mechanism
type Type string

const (
	// GRPC is the primary transport
	GRPC Type = "grpc"
	// WebSocket is the fallback transport
	WebSocket Type = "websocket"
)

// Config holds transport configuration
type Config struct {
	// Type is the preferred transport type
	Type Type
	// ServerAddr is the address to connect to
	ServerAddr string
	// TLS holds TLS configuration
	TLS TLSConfig
	// WebSocket holds WebSocket-specific config
	WebSocket WSConfig
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	CertPath   string
	KeyPath    string
	CAPath     string
	MinVersion string
	ServerName string
}

// WSConfig holds WebSocket configuration
type WSConfig struct {
	// Path is the WebSocket endpoint path
	Path string
	// PingInterval is how often to send ping frames
	PingInterval time.Duration
	// ReadDeadline is the read deadline for WebSocket connections
	ReadDeadline time.Duration
}

// Client is the transport-agnostic client interface
type Client interface {
	// Connect establishes the connection
	Connect(ctx context.Context) error
	// Close closes the connection
	Close() error
	// GRPCConn returns the underlying gRPC connection (nil for WebSocket)
	GRPCConn() *grpc.ClientConn
	// IsConnected returns true if the connection is active
	IsConnected() bool
	// Type returns the transport type
	Type() Type
}

// grpcClient implements Client for gRPC transport
type grpcClient struct {
	conn   *grpc.ClientConn
	config Config
	mu     sync.RWMutex
}

// NewGRPCClient creates a new gRPC client
func NewGRPCClient(config Config) Client {
	return &grpcClient{
		config: config,
	}
}

func (g *grpcClient) Connect(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var creds credentials.TransportCredentials

	if g.config.TLS.CertPath != "" && g.config.TLS.KeyPath != "" {
		// mTLS
		cert, err := tls.LoadX509KeyPair(g.config.TLS.CertPath, g.config.TLS.KeyPath)
		if err != nil {
			return fmt.Errorf("failed to load client certificate: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
			ServerName:   g.config.TLS.ServerName,
		}

		if g.config.TLS.CAPath != "" {
			caCert, err := os.ReadFile(g.config.TLS.CAPath)
			if err != nil {
				return fmt.Errorf("failed to read CA certificate: %w", err)
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return fmt.Errorf("failed to parse CA certificate")
			}
			tlsConfig.RootCAs = caCertPool
		}

		creds = credentials.NewTLS(tlsConfig)
	} else {
		creds = insecure.NewCredentials()
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
	}

	conn, err := grpc.DialContext(ctx, g.config.ServerAddr, opts...)
	if err != nil {
		return fmt.Errorf("failed to dial gRPC server: %w", err)
	}

	g.conn = conn
	return nil
}

func (g *grpcClient) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn != nil {
		return g.conn.Close()
	}
	return nil
}

func (g *grpcClient) GRPCConn() *grpc.ClientConn {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.conn
}

func (g *grpcClient) IsConnected() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.conn != nil
}

func (g *grpcClient) Type() Type {
	return GRPC
}
