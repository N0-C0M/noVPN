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
	Pricing             PricingConfig `yaml:"pricing"`
	Plans               []PlanConfig  `yaml:"plans"`
}

type PricingConfig struct {
	PlanID                string               `yaml:"plan_id"`
	ProductName           string               `yaml:"product_name"`
	ProductTagline        string               `yaml:"product_tagline"`
	ProductDescription    string               `yaml:"product_description"`
	BaseMonthlyPriceMinor int64                `yaml:"base_monthly_price_minor"`
	Currency              string               `yaml:"currency"`
	MinDevices            int                  `yaml:"min_devices"`
	MaxDevices            int                  `yaml:"max_devices"`
	DefaultDevices        int                  `yaml:"default_devices"`
	DefaultMonths         int                  `yaml:"default_months"`
	MonthOptions          []PricingMonthOption `yaml:"month_options"`
	Features              []string             `yaml:"features"`
	SBPPaymentNotice      string               `yaml:"sbp_payment_notice"`
	SBPPlaceholderLabel   string               `yaml:"sbp_placeholder_label"`
	AccountPortalHeadline string               `yaml:"account_portal_headline"`
	AccountPortalSubtext  string               `yaml:"account_portal_subtext"`
}

type PricingMonthOption struct {
	Months          int    `yaml:"months"`
	DiscountPercent int    `yaml:"discount_percent"`
	Label           string `yaml:"label"`
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
	c.Pricing.setDefaults(c.Plans)
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

func (p *PricingConfig) setDefaults(plans []PlanConfig) {
	if strings.TrimSpace(p.PlanID) == "" && len(plans) > 0 {
		p.PlanID = strings.TrimSpace(plans[0].ID)
	}
	if strings.TrimSpace(p.ProductName) == "" {
		if len(plans) > 0 && strings.TrimSpace(plans[0].Name) != "" {
			p.ProductName = strings.TrimSpace(plans[0].Name)
		} else {
			p.ProductName = "VPN доступ"
		}
	}
	if strings.TrimSpace(p.ProductTagline) == "" {
		p.ProductTagline = "Быстрый запуск, один кабинет и отдельный ключ на каждое устройство."
	}
	if strings.TrimSpace(p.ProductDescription) == "" {
		if len(plans) > 0 && strings.TrimSpace(plans[0].Description) != "" {
			p.ProductDescription = strings.TrimSpace(plans[0].Description)
		} else {
			p.ProductDescription = "Выберите количество устройств и срок подписки, оплатите по СБП и управляйте всеми выданными ключами из одного кабинета."
		}
	}
	if p.BaseMonthlyPriceMinor <= 0 && len(plans) > 0 && plans[0].PriceMinor > 0 {
		p.BaseMonthlyPriceMinor = plans[0].PriceMinor
	}
	if strings.TrimSpace(p.Currency) == "" {
		if len(plans) > 0 && strings.TrimSpace(plans[0].Currency) != "" {
			p.Currency = strings.TrimSpace(plans[0].Currency)
		} else {
			p.Currency = "RUB"
		}
	}
	if p.MinDevices <= 0 {
		p.MinDevices = 1
	}
	if p.MaxDevices < p.MinDevices {
		p.MaxDevices = 10
	}
	if p.DefaultDevices < p.MinDevices || p.DefaultDevices > p.MaxDevices {
		p.DefaultDevices = p.MinDevices
	}
	if len(p.MonthOptions) == 0 {
		p.MonthOptions = []PricingMonthOption{
			{Months: 1, DiscountPercent: 0, Label: "1 месяц"},
			{Months: 3, DiscountPercent: 7, Label: "3 месяца"},
			{Months: 6, DiscountPercent: 15, Label: "6 месяцев"},
			{Months: 12, DiscountPercent: 25, Label: "12 месяцев"},
		}
	}
	if p.DefaultMonths <= 0 {
		p.DefaultMonths = p.MonthOptions[0].Months
	}
	for index := range p.MonthOptions {
		if p.MonthOptions[index].Months <= 0 {
			p.MonthOptions[index].Months = 1
		}
		if p.MonthOptions[index].DiscountPercent < 0 {
			p.MonthOptions[index].DiscountPercent = 0
		}
		if p.MonthOptions[index].DiscountPercent > 95 {
			p.MonthOptions[index].DiscountPercent = 95
		}
		if strings.TrimSpace(p.MonthOptions[index].Label) == "" {
			p.MonthOptions[index].Label = fmt.Sprintf("%d мес.", p.MonthOptions[index].Months)
		}
	}
	p.Features = normalizeStringList(p.Features)
	if len(p.Features) == 0 {
		p.Features = []string{
			"Отдельная Happ-ссылка для каждого оплаченного слота",
			"Личный ключ от сайта для продления и управления доступом",
			"Чем больше срок, тем выше автоматическая скидка",
		}
	}
	if strings.TrimSpace(p.SBPPaymentNotice) == "" {
		p.SBPPaymentNotice = "Интеграцию СБП можно подключить позже. Пока checkout работает как заглушка и выдает ключи сразу после подтверждения заказа."
	}
	if strings.TrimSpace(p.SBPPlaceholderLabel) == "" {
		p.SBPPlaceholderLabel = "Заглушка СБП"
	}
	if strings.TrimSpace(p.AccountPortalHeadline) == "" {
		p.AccountPortalHeadline = "Личный кабинет и продление"
	}
	if strings.TrimSpace(p.AccountPortalSubtext) == "" {
		p.AccountPortalSubtext = "Каждый заказ привязывается к одному ключу от сайта. Через него пользователь открывает кабинет, копирует ключи и оформляет продление."
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.StoragePath) == "" {
		return fmt.Errorf("storage_path is required")
	}
	if strings.TrimSpace(c.ControlPlaneBaseURL) == "" {
		return fmt.Errorf("control_plane_base_url is required")
	}
	if len(c.Plans) == 0 && strings.TrimSpace(c.Pricing.PlanID) == "" {
		return fmt.Errorf("either pricing.plan_id or at least one plan is required")
	}
	if strings.TrimSpace(c.Pricing.PlanID) == "" {
		return fmt.Errorf("pricing.plan_id is required")
	}
	if c.Pricing.BaseMonthlyPriceMinor < 0 {
		return fmt.Errorf("pricing.base_monthly_price_minor must be non-negative")
	}
	if c.Pricing.MinDevices <= 0 {
		return fmt.Errorf("pricing.min_devices must be greater than zero")
	}
	if c.Pricing.MaxDevices < c.Pricing.MinDevices {
		return fmt.Errorf("pricing.max_devices must be greater than or equal to pricing.min_devices")
	}
	if len(c.Pricing.MonthOptions) == 0 {
		return fmt.Errorf("pricing.month_options must contain at least one option")
	}
	seenMonths := make(map[int]struct{}, len(c.Pricing.MonthOptions))
	for _, option := range c.Pricing.MonthOptions {
		if option.Months <= 0 {
			return fmt.Errorf("pricing month option must have positive months")
		}
		if _, ok := seenMonths[option.Months]; ok {
			return fmt.Errorf("duplicate pricing month option %d", option.Months)
		}
		seenMonths[option.Months] = struct{}{}
		if option.DiscountPercent < 0 || option.DiscountPercent > 95 {
			return fmt.Errorf("pricing month option %d has invalid discount_percent", option.Months)
		}
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
