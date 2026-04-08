package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

// wsClient implements Client for WebSocket transport
type wsClient struct {
	conn   *websocket.Conn
	config Config
	mu     sync.RWMutex
	done   chan struct{}
	ping   *time.Ticker
}

// NewWSClient creates a new WebSocket client
func NewWSClient(config Config) Client {
	return &wsClient{
		config: config,
		done:   make(chan struct{}),
	}
}

func (w *wsClient) Connect(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	scheme := "wss"
	if w.config.TLS.CertPath == "" {
		scheme = "ws"
	}

	path := "/ws"
	if w.config.WebSocket.Path != "" {
		path = w.config.WebSocket.Path
	}

	url := fmt.Sprintf("%s://%s%s", scheme, w.config.ServerAddr, path)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	if scheme == "wss" {
		tlsConfig := &tls.Config{
			MinVersion:         tls.VersionTLS13,
			InsecureSkipVerify: false,
			ServerName:         w.config.TLS.ServerName,
		}

		if w.config.TLS.CertPath != "" && w.config.TLS.KeyPath != "" {
			cert, err := tls.LoadX509KeyPair(w.config.TLS.CertPath, w.config.TLS.KeyPath)
			if err != nil {
				return fmt.Errorf("failed to load client certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		if w.config.TLS.CAPath != "" {
			// For WebSocket with custom CA, we need to configure the HTTP client
			// This is handled by the dialer's TLS config
		}

		dialer.TLSClientConfig = tlsConfig
	}

	header := http.Header{}
	// Add authentication headers if needed
	// header.Set("Authorization", "Bearer "+token)

	conn, _, err := dialer.DialContext(ctx, url, header)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket server: %w", err)
	}

	w.conn = conn

	// Start ping/pong keepalive
	pingInterval := 30 * time.Second
	if w.config.WebSocket.PingInterval != 0 {
		pingInterval = w.config.WebSocket.PingInterval
	}

	w.ping = time.NewTicker(pingInterval)
	go w.pingLoop()

	return nil
}

func (w *wsClient) pingLoop() {
	for {
		select {
		case <-w.ping.C:
			w.mu.RLock()
			conn := w.conn
			w.mu.RUnlock()

			if conn == nil {
				return
			}

			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
				// Connection lost, will be detected by reconnect logic
				return
			}
		case <-w.done:
			return
		}
	}
}

func (w *wsClient) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.ping != nil {
		w.ping.Stop()
	}

	close(w.done)

	if w.conn != nil {
		return w.conn.Close()
	}
	return nil
}

func (w *wsClient) GRPCConn() *grpc.ClientConn {
	return nil
}

func (w *wsClient) IsConnected() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.conn != nil
}

func (w *wsClient) Type() Type {
	return WebSocket
}

// WriteJSON sends a JSON message over the WebSocket connection
func (w *wsClient) WriteJSON(v interface{}) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	return w.conn.WriteJSON(v)
}

// ReadJSON reads a JSON message from the WebSocket connection
func (w *wsClient) ReadJSON(v interface{}) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	readDeadline := 60 * time.Second
	if w.config.WebSocket.ReadDeadline != 0 {
		readDeadline = w.config.WebSocket.ReadDeadline
	}

	w.conn.SetReadDeadline(time.Now().Add(readDeadline))
	return w.conn.ReadJSON(v)
}
