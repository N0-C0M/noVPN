package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Observability ObservabilityConfig `yaml:"observability"`
	Listeners     ListenerSet         `yaml:"listeners"`
}

type ServerConfig struct {
	ShutdownTimeout      time.Duration `yaml:"shutdown_timeout"`
	ReadinessGracePeriod time.Duration `yaml:"readiness_grace_period"`
	UpstreamDialTimeout  time.Duration `yaml:"upstream_dial_timeout"`
}

type ObservabilityConfig struct {
	LogLevel    string `yaml:"log_level"`
	JSONLogs    bool   `yaml:"json_logs"`
	HealthAddr  string `yaml:"health_addr"`
	MetricsPath string `yaml:"metrics_path"`
}

type ListenerSet struct {
	TCP []TCPListenerConfig `yaml:"tcp"`
	UDP []UDPListenerConfig `yaml:"udp"`
}

type CommonListenerConfig struct {
	Name         string `yaml:"name"`
	Enabled      bool   `yaml:"enabled"`
	ListenAddr   string `yaml:"listen_addr"`
	UpstreamAddr string `yaml:"upstream_addr"`
}

type TCPListenerConfig struct {
	CommonListenerConfig `yaml:",inline"`
	Timeouts             TCPTimeouts `yaml:"timeouts"`
	Limits               TCPLimits   `yaml:"limits"`
}

type TCPTimeouts struct {
	Dial time.Duration `yaml:"dial"`
	Idle time.Duration `yaml:"idle"`
}

type TCPLimits struct {
	MaxConnections  int `yaml:"max_connections"`
	PerIPConnection int `yaml:"per_ip_connections"`
}

type UDPListenerConfig struct {
	CommonListenerConfig `yaml:",inline"`
	Session              UDPSessionConfig `yaml:"session"`
	Limits               UDPLimits        `yaml:"limits"`
}

type UDPSessionConfig struct {
	IdleTTL         time.Duration `yaml:"idle_ttl"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	MaxSessions     int           `yaml:"max_sessions"`
}

type UDPLimits struct {
	MaxPacketSize int `yaml:"max_packet_size"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.setDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) setDefaults() {
	if c.Server.ShutdownTimeout <= 0 {
		c.Server.ShutdownTimeout = 15 * time.Second
	}
	if c.Server.ReadinessGracePeriod < 0 {
		c.Server.ReadinessGracePeriod = 0
	}
	if c.Server.UpstreamDialTimeout <= 0 {
		c.Server.UpstreamDialTimeout = 5 * time.Second
	}

	if c.Observability.LogLevel == "" {
		c.Observability.LogLevel = "info"
	}
	if c.Observability.HealthAddr == "" {
		c.Observability.HealthAddr = "127.0.0.1:9101"
	}
	if c.Observability.MetricsPath == "" {
		c.Observability.MetricsPath = "/metrics"
	}

	for i := range c.Listeners.TCP {
		if c.Listeners.TCP[i].Timeouts.Dial <= 0 {
			c.Listeners.TCP[i].Timeouts.Dial = 5 * time.Second
		}
		if c.Listeners.TCP[i].Timeouts.Idle <= 0 {
			c.Listeners.TCP[i].Timeouts.Idle = 2 * time.Minute
		}
		if c.Listeners.TCP[i].Limits.MaxConnections <= 0 {
			c.Listeners.TCP[i].Limits.MaxConnections = 10000
		}
		if c.Listeners.TCP[i].Limits.PerIPConnection <= 0 {
			c.Listeners.TCP[i].Limits.PerIPConnection = 200
		}
	}

	for i := range c.Listeners.UDP {
		if c.Listeners.UDP[i].Session.IdleTTL <= 0 {
			c.Listeners.UDP[i].Session.IdleTTL = 60 * time.Second
		}
		if c.Listeners.UDP[i].Session.CleanupInterval <= 0 {
			c.Listeners.UDP[i].Session.CleanupInterval = 15 * time.Second
		}
		if c.Listeners.UDP[i].Session.MaxSessions <= 0 {
			c.Listeners.UDP[i].Session.MaxSessions = 50000
		}
		if c.Listeners.UDP[i].Limits.MaxPacketSize <= 0 {
			c.Listeners.UDP[i].Limits.MaxPacketSize = 1500
		}
	}
}
