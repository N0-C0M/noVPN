package config

import (
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	uuidPattern    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	shortIDPattern = regexp.MustCompile(`^(|[0-9a-fA-F]{2,16})$`)
)

func (c Config) Validate() error {
	if c.Observability.HealthAddr == "" {
		return errors.New("observability.health_addr must not be empty")
	}
	if c.Observability.MetricsPath == "" {
		return errors.New("observability.metrics_path must not be empty")
	}
	if c.Admin.Enabled {
		if c.Admin.ListenAddr == "" {
			return errors.New("admin.listen_addr must not be empty")
		}
		if _, _, err := net.SplitHostPort(c.Admin.ListenAddr); err != nil {
			return fmt.Errorf("admin.listen_addr: %w", err)
		}
		if c.Admin.StoragePath == "" {
			return errors.New("admin.storage_path must not be empty")
		}
	}

	enabled := 0
	seenNames := make(map[string]struct{})

	for _, listener := range c.Listeners.TCP {
		if !listener.Enabled {
			continue
		}
		enabled++
		if err := validateCommon(listener.CommonListenerConfig); err != nil {
			return fmt.Errorf("listeners.tcp[%s]: %w", listener.Name, err)
		}
		if _, ok := seenNames[listener.Name]; ok {
			return fmt.Errorf("duplicate listener name %q", listener.Name)
		}
		seenNames[listener.Name] = struct{}{}
	}

	for _, listener := range c.Listeners.UDP {
		if !listener.Enabled {
			continue
		}
		enabled++
		if err := validateCommon(listener.CommonListenerConfig); err != nil {
			return fmt.Errorf("listeners.udp[%s]: %w", listener.Name, err)
		}
		if _, ok := seenNames[listener.Name]; ok {
			return fmt.Errorf("duplicate listener name %q", listener.Name)
		}
		seenNames[listener.Name] = struct{}{}
	}

	if err := validateReality(c.Core.Reality); err != nil {
		return err
	}

	if enabled == 0 && !c.Core.Reality.Enabled {
		return errors.New("at least one enabled TCP or UDP listener is required")
	}

	return nil
}

func validateCommon(c CommonListenerConfig) error {
	if c.Name == "" {
		return errors.New("name must not be empty")
	}
	if c.ListenAddr == "" {
		return errors.New("listen_addr must not be empty")
	}
	if c.UpstreamAddr == "" {
		return errors.New("upstream_addr must not be empty")
	}
	return nil
}

func validateReality(c RealityConfig) error {
	if !c.Enabled {
		return nil
	}
	if c.ListenAddr == "" {
		return errors.New("core.reality.listen_addr must not be empty")
	}
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return fmt.Errorf("core.reality.listen_addr: %w", err)
	}
	if c.PublicHost == "" {
		return errors.New("core.reality.public_host must not be empty")
	}
	if c.PublicPort <= 0 || c.PublicPort > 65535 {
		return errors.New("core.reality.public_port must be between 1 and 65535")
	}
	if c.Target == "" {
		return errors.New("core.reality.target must not be empty")
	}
	if _, _, err := net.SplitHostPort(c.Target); err != nil {
		return fmt.Errorf("core.reality.target: %w", err)
	}
	if len(c.ServerNames) == 0 {
		return errors.New("core.reality.server_names must not be empty")
	}
	for _, value := range c.ServerNames {
		if strings.TrimSpace(value) == "" {
			return errors.New("core.reality.server_names must not contain empty values")
		}
	}
	if c.UUID != "" && !uuidPattern.MatchString(c.UUID) {
		return errors.New("core.reality.uuid must be a valid UUID")
	}
	if c.PrivateKey != "" {
		rawKey, err := base64.RawURLEncoding.DecodeString(c.PrivateKey)
		if err != nil {
			return fmt.Errorf("core.reality.private_key: %w", err)
		}
		if _, err := ecdh.X25519().NewPrivateKey(rawKey); err != nil {
			return fmt.Errorf("core.reality.private_key: %w", err)
		}
	}
	for _, value := range c.ShortIDs {
		if !shortIDPattern.MatchString(value) || len(value)%2 != 0 {
			return fmt.Errorf("core.reality.short_ids contains invalid value %q", value)
		}
	}
	if c.Flow == "" {
		return errors.New("core.reality.flow must not be empty")
	}
	if c.Xray.BinaryPath == "" {
		return errors.New("core.reality.xray.binary_path must not be empty")
	}
	if c.Xray.ConfigPath == "" {
		return errors.New("core.reality.xray.config_path must not be empty")
	}
	if c.Xray.StatePath == "" {
		return errors.New("core.reality.xray.state_path must not be empty")
	}
	if c.Xray.RegistryPath == "" {
		return errors.New("core.reality.xray.registry_path must not be empty")
	}
	if c.Xray.ClientProfilePath == "" {
		return errors.New("core.reality.xray.client_profile_path must not be empty")
	}
	if c.Xray.ServiceName == "" {
		return errors.New("core.reality.xray.service_name must not be empty")
	}
	switch strings.ToLower(strings.TrimSpace(c.Xray.Install.Method)) {
	case "", "official-script":
	case "none":
	default:
		return errors.New("core.reality.xray.install.method must be either official-script or none")
	}
	if strings.EqualFold(strings.TrimSpace(c.Xray.Install.Method), "official-script") && filepath.Base(c.Xray.ConfigPath) != "config.json" {
		return errors.New("core.reality.xray.config_path must end with config.json when using the official installer")
	}
	return nil
}
