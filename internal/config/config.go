package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Observability ObservabilityConfig `yaml:"observability"`
	Admin         AdminConfig         `yaml:"admin"`
	Listeners     ListenerSet         `yaml:"listeners"`
	Core          CoreConfig          `yaml:"core"`
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

type AdminConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ListenAddr  string `yaml:"listen_addr"`
	StoragePath string `yaml:"storage_path"`
	Token       string `yaml:"token"`
	BasePath    string `yaml:"base_path"`
}

type ListenerSet struct {
	TCP []TCPListenerConfig `yaml:"tcp"`
	UDP []UDPListenerConfig `yaml:"udp"`
}

type CoreConfig struct {
	Reality RealityConfig `yaml:"reality"`
}

type RealityConfig struct {
	Enabled           bool                  `yaml:"enabled"`
	ListenAddr        string                `yaml:"listen_addr"`
	PublicHost        string                `yaml:"public_host"`
	PublicPort        int                   `yaml:"public_port"`
	Target            string                `yaml:"target"`
	ServerNames       []string              `yaml:"server_names"`
	UUID              string                `yaml:"uuid"`
	PrivateKey        string                `yaml:"private_key"`
	ShortIDs          []string              `yaml:"short_ids"`
	Flow              string                `yaml:"flow"`
	UserEmail         string                `yaml:"user_email"`
	Fingerprint       string                `yaml:"fingerprint"`
	SpiderX           string                `yaml:"spider_x"`
	Show              bool                  `yaml:"show"`
	Xver              int                   `yaml:"xver"`
	MinClientVer      string                `yaml:"min_client_ver"`
	MaxClientVer      string                `yaml:"max_client_ver"`
	MaxTimeDiffMillis int                   `yaml:"max_time_diff_ms"`
	Sniffing          RealitySniffingConfig `yaml:"sniffing"`
	Xray              XrayConfig            `yaml:"xray"`
}

type RealitySniffingConfig struct {
	Enabled      bool     `yaml:"enabled"`
	DestOverride []string `yaml:"dest_override"`
}

type XrayConfig struct {
	BinaryPath        string            `yaml:"binary_path"`
	ConfigPath        string            `yaml:"config_path"`
	StatePath         string            `yaml:"state_path"`
	RegistryPath      string            `yaml:"registry_path"`
	ClientProfilePath string            `yaml:"client_profile_path"`
	ServiceName       string            `yaml:"service_name"`
	Install           XrayInstallConfig `yaml:"install"`
	Log               XrayLogConfig     `yaml:"log"`
}

type XrayInstallConfig struct {
	Method    string `yaml:"method"`
	ScriptURL string `yaml:"script_url"`
}

type XrayLogConfig struct {
	Level      string `yaml:"level"`
	AccessPath string `yaml:"access_path"`
	ErrorPath  string `yaml:"error_path"`
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

	if c.Admin.ListenAddr == "" {
		c.Admin.ListenAddr = "127.0.0.1:9112"
	}
	if c.Admin.BasePath == "" {
		c.Admin.BasePath = "/admin"
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

	c.Core.Reality.setDefaults()
}

func (c *RealityConfig) setDefaults() {
	if c.Flow == "" {
		c.Flow = "xtls-rprx-vision"
	}
	if c.UserEmail == "" {
		c.UserEmail = "default@novpn"
	}
	if c.Fingerprint == "" {
		c.Fingerprint = "chrome"
	}
	if !c.Sniffing.Enabled && len(c.Sniffing.DestOverride) == 0 {
		c.Sniffing.Enabled = true
	}
	if len(c.Sniffing.DestOverride) == 0 {
		c.Sniffing.DestOverride = []string{"http", "tls", "quic"}
	}
	if len(c.ServerNames) == 0 {
		if host := hostPart(c.Target); host != "" {
			c.ServerNames = []string{host}
		}
	}
	if c.PublicPort <= 0 {
		if _, port, err := net.SplitHostPort(c.ListenAddr); err == nil {
			if value := portFromString(port); value > 0 {
				c.PublicPort = value
			}
		}
	}

	if c.Xray.BinaryPath == "" {
		c.Xray.BinaryPath = "/usr/local/bin/xray"
	}
	if c.Xray.ConfigPath == "" {
		c.Xray.ConfigPath = "/usr/local/etc/xray/config.json"
	}
	if c.Xray.StatePath == "" {
		c.Xray.StatePath = "/var/lib/novpn/reality/state.yaml"
	}
	if c.Xray.RegistryPath == "" {
		c.Xray.RegistryPath = "/var/lib/novpn/reality/registry.json"
	}
	if c.Xray.ClientProfilePath == "" {
		c.Xray.ClientProfilePath = "/var/lib/novpn/reality/client-profile.yaml"
	}
	if c.Xray.ServiceName == "" {
		c.Xray.ServiceName = "xray"
	}
	if c.Xray.Install.Method == "" {
		c.Xray.Install.Method = "official-script"
	}
	if c.Xray.Install.ScriptURL == "" {
		c.Xray.Install.ScriptURL = "https://github.com/XTLS/Xray-install/raw/main/install-release.sh"
	}
	if c.Xray.Log.Level == "" {
		c.Xray.Log.Level = "warning"
	}
	if c.Xray.Log.AccessPath == "" {
		c.Xray.Log.AccessPath = "/var/log/xray/access.log"
	}
	if c.Xray.Log.ErrorPath == "" {
		c.Xray.Log.ErrorPath = "/var/log/xray/error.log"
	}
}

func hostPart(raw string) string {
	host := raw
	if strings.Contains(raw, ":") {
		if parsedHost, _, err := net.SplitHostPort(raw); err == nil {
			host = parsedHost
		}
	}

	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return ""
	}
	return host
}

func portFromString(raw string) int {
	var port int
	_, _ = fmt.Sscanf(raw, "%d", &port)
	return port
}

func (c RealityConfig) ConfigDir() string {
	return filepath.Dir(c.Xray.ConfigPath)
}
