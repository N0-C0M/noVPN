package config

import (
	"errors"
	"fmt"
)

func (c Config) Validate() error {
	if c.Observability.HealthAddr == "" {
		return errors.New("observability.health_addr must not be empty")
	}
	if c.Observability.MetricsPath == "" {
		return errors.New("observability.metrics_path must not be empty")
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

	if enabled == 0 {
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
