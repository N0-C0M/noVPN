package payments

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr          string        `yaml:"listen_addr"`
	StoragePath         string        `yaml:"storage_path"`
	AdminToken          string        `yaml:"admin_token"`
	PublicBaseURL       string        `yaml:"public_base_url"`
	ControlPlaneBaseURL string        `yaml:"control_plane_base_url"`
	ControlPlaneToken   string        `yaml:"control_plane_token"`
	ShutdownTimeout     time.Duration `yaml:"shutdown_timeout"`
	BrandName           string        `yaml:"brand_name"`
	SupportLink         string        `yaml:"support_link"`
	PaymentCardNumber   string        `yaml:"payment_card_number"`
	PaymentCardHolder   string        `yaml:"payment_card_holder"`
	PaymentCardBank     string        `yaml:"payment_card_bank"`
	AndroidLauncherURL  string        `yaml:"android_launcher_url"`
	WindowsLauncherURL  string        `yaml:"windows_launcher_url"`
	HappDownloadURL     string        `yaml:"happ_download_url"`
	Plans               []PlanConfig  `yaml:"plans"`
}

type PlanConfig struct {
	ID           string       `yaml:"id"`
	Name         string       `yaml:"name"`
	Description  string       `yaml:"description"`
	Badge        string       `yaml:"badge"`
	PriceMinor   int64        `yaml:"price_minor"`
	Currency     string       `yaml:"currency"`
	DurationDays int          `yaml:"duration_days"`
	DeliveryMode DeliveryMode `yaml:"delivery_mode"`
	MaxUses      int          `yaml:"max_uses"`
	Features     []string     `yaml:"features"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	cfg.setDefaults(path)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) setDefaults(configPath string) {
	if strings.TrimSpace(c.ListenAddr) == "" {
		c.ListenAddr = "127.0.0.1:9120"
	}
	if strings.TrimSpace(c.StoragePath) == "" {
		baseDir := filepath.Dir(configPath)
		c.StoragePath = filepath.Join(baseDir, "orders.json")
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 15 * time.Second
	}
	if strings.TrimSpace(c.BrandName) == "" {
		c.BrandName = "NoVPN"
	}
	for index := range c.Plans {
		if strings.TrimSpace(c.Plans[index].Currency) == "" {
			c.Plans[index].Currency = "RUB"
		}
		if c.Plans[index].MaxUses <= 0 {
			c.Plans[index].MaxUses = 1
		}
		c.Plans[index].Features = normalizeStringList(c.Plans[index].Features)
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.StoragePath) == "" {
		return fmt.Errorf("storage_path is required")
	}
	if strings.TrimSpace(c.ControlPlaneBaseURL) == "" {
		return fmt.Errorf("control_plane_base_url is required")
	}
	if len(c.Plans) == 0 {
		return fmt.Errorf("at least one plan is required")
	}

	seen := make(map[string]struct{}, len(c.Plans))
	for _, plan := range c.Plans {
		id := strings.TrimSpace(plan.ID)
		if id == "" {
			return fmt.Errorf("plan id is required")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate plan id %q", id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(plan.Name) == "" {
			return fmt.Errorf("plan %q name is required", id)
		}
		switch plan.DeliveryMode {
		case DeliveryModeInviteCode, DeliveryModeProfileBundle:
		default:
			return fmt.Errorf("plan %q has unsupported delivery_mode %q", id, plan.DeliveryMode)
		}
		if plan.PriceMinor < 0 {
			return fmt.Errorf("plan %q price_minor must be non-negative", id)
		}
	}
	return nil
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
