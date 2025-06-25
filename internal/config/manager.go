package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type manager struct {
	mu     sync.RWMutex
	config *Config
	viper  *viper.Viper
}

func NewManager() Manager {
	return &manager{
		viper: viper.New(),
	}
}

func (m *manager) Load(configPath string) (*Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.setupViper(configPath); err != nil {
		return nil, fmt.Errorf("failed to setup viper: %w", err)
	}

	if err := m.viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := m.viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := m.validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	m.config = &config
	return &config, nil
}

func (m *manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.viper == nil {
		return fmt.Errorf("config not loaded")
	}

	if err := m.viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	var config Config
	if err := m.viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := m.validateConfig(&config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	m.config = &config
	return nil
}

func (m *manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func (m *manager) setupViper(configPath string) error {
	m.viper.SetConfigFile(configPath)
	
	m.viper.SetEnvPrefix("SITEMAP")
	m.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	m.viper.AutomaticEnv()

	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	return nil
}

func (m *manager) validateConfig(config *Config) error {
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	if config.Worker.MaxWorkers <= 0 {
		return fmt.Errorf("max_workers must be positive")
	}

	if config.Worker.QueueSize <= 0 {
		return fmt.Errorf("queue_size must be positive")
	}

	if config.Storage.DataDir == "" {
		return fmt.Errorf("data_dir cannot be empty")
	}

	return nil
}