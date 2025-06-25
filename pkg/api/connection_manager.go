package api

import (
	"net"
	"net/http"
	"time"

	"sitemap-go/pkg/logger"
)

// ConnectionConfig holds configuration for HTTP connections
type ConnectionConfig struct {
	MaxConnsPerHost     int           `json:"max_conns_per_host"`
	MaxIdleConns        int           `json:"max_idle_conns"`
	MaxIdleConnsPerHost int           `json:"max_idle_conns_per_host"`
	IdleConnTimeout     time.Duration `json:"idle_conn_timeout"`
	DialTimeout         time.Duration `json:"dial_timeout"`
	KeepAlive           time.Duration `json:"keep_alive"`
	TLSHandshakeTimeout time.Duration `json:"tls_handshake_timeout"`
	RequestTimeout      time.Duration `json:"request_timeout"`
	ResponseHeaderTimeout time.Duration `json:"response_header_timeout"`
}

// DefaultConnectionConfig returns optimized default connection settings
func DefaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		MaxConnsPerHost:       100,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DialTimeout:           10 * time.Second,
		KeepAlive:             30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		RequestTimeout:        30 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
}

// HighThroughputConnectionConfig returns config optimized for high throughput
func HighThroughputConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		MaxConnsPerHost:       200,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       120 * time.Second,
		DialTimeout:           5 * time.Second,
		KeepAlive:             60 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		RequestTimeout:        15 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}
}

// ConnectionManager manages HTTP client connections with pooling and optimization
type ConnectionManager struct {
	config ConnectionConfig
	client *http.Client
	log    *logger.Logger
}

// NewConnectionManager creates a new connection manager with specified config
func NewConnectionManager(config ConnectionConfig) *ConnectionManager {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		DisableKeepAlives:     false,
		DisableCompression:    false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.RequestTimeout,
	}

	return &ConnectionManager{
		config: config,
		client: client,
		log:    logger.GetLogger().WithField("component", "connection_manager"),
	}
}

// GetClient returns the managed HTTP client
func (cm *ConnectionManager) GetClient() *http.Client {
	return cm.client
}

// GetConnectionStats returns current connection statistics
func (cm *ConnectionManager) GetConnectionStats() ConnectionStats {
	if _, ok := cm.client.Transport.(*http.Transport); ok {
		return ConnectionStats{
			MaxConnsPerHost:     cm.config.MaxConnsPerHost,
			MaxIdleConns:        cm.config.MaxIdleConns,
			MaxIdleConnsPerHost: cm.config.MaxIdleConnsPerHost,
			IdleConnTimeout:     cm.config.IdleConnTimeout,
			// Note: Go's http.Transport doesn't expose current connection counts
			// This would require custom instrumentation for real-time stats
			ActiveConnections: 0, // Would need custom tracking
			IdleConnections:   0, // Would need custom tracking
		}
	}
	return ConnectionStats{}
}

// UpdateConfig updates the connection configuration
func (cm *ConnectionManager) UpdateConfig(config ConnectionConfig) {
	cm.log.Info("Updating connection configuration")
	
	// Create new transport with updated config
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		DisableKeepAlives:     false,
		DisableCompression:    false,
	}

	// Close existing idle connections
	if oldTransport, ok := cm.client.Transport.(*http.Transport); ok {
		oldTransport.CloseIdleConnections()
	}

	// Update client
	cm.client.Transport = transport
	cm.client.Timeout = config.RequestTimeout
	cm.config = config
	
	cm.log.WithFields(map[string]interface{}{
		"max_conns_per_host":     config.MaxConnsPerHost,
		"max_idle_conns":         config.MaxIdleConns,
		"request_timeout":        config.RequestTimeout,
	}).Info("Connection configuration updated")
}

// Close closes all idle connections
func (cm *ConnectionManager) Close() {
	cm.log.Info("Closing connection manager")
	if transport, ok := cm.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// ConnectionStats represents connection pool statistics
type ConnectionStats struct {
	MaxConnsPerHost     int           `json:"max_conns_per_host"`
	MaxIdleConns        int           `json:"max_idle_conns"`
	MaxIdleConnsPerHost int           `json:"max_idle_conns_per_host"`
	IdleConnTimeout     time.Duration `json:"idle_conn_timeout"`
	ActiveConnections   int           `json:"active_connections"`
	IdleConnections     int           `json:"idle_connections"`
}