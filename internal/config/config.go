package config

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Storage  StorageConfig  `mapstructure:"storage"`
	APIs     []APIConfig    `mapstructure:"apis"`
	Sites    []SiteConfig   `mapstructure:"sites"`
	Security SecurityConfig `mapstructure:"security"`
	Worker   WorkerConfig   `mapstructure:"worker"`
	Logger   LoggerConfig   `mapstructure:"logger"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type StorageConfig struct {
	DataDir     string `mapstructure:"data_dir"`
	CacheSize   int    `mapstructure:"cache_size"`
	EncryptData bool   `mapstructure:"encrypt_data"`
}

type APIConfig struct {
	Name     string `mapstructure:"name"`
	Endpoint string `mapstructure:"endpoint"`
	APIKey   string `mapstructure:"api_key"`
	QPS      int    `mapstructure:"qps"`
	Timeout  int    `mapstructure:"timeout"`
}

type SiteConfig struct {
	Domain    string   `mapstructure:"domain"`
	Sitemaps  []string `mapstructure:"sitemaps"`
	Enabled   bool     `mapstructure:"enabled"`
	Frequency int      `mapstructure:"frequency"`
}

type SecurityConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"`
	LogSensitive  bool   `mapstructure:"log_sensitive"`
}

type WorkerConfig struct {
	MaxWorkers   int `mapstructure:"max_workers"`
	QueueSize    int `mapstructure:"queue_size"`
	BatchSize    int `mapstructure:"batch_size"`
	TimeoutMs    int `mapstructure:"timeout_ms"`
}

type LoggerConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	TimeFormat string `mapstructure:"time_format"`
}

type Manager interface {
	Load(configPath string) (*Config, error)
	Reload() error
	GetConfig() *Config
}