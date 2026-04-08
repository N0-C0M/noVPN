package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type config struct {
	Mode            string              `json:"mode"`
	Seed            string              `json:"seed"`
	TrafficStrategy string              `json:"traffic_strategy"`
	PatternStrategy string              `json:"pattern_strategy"`
	Remote          remoteConfig        `json:"remote"`
	Listen          socksEndpoint       `json:"listen"`
	Upstream        socksEndpoint       `json:"upstream"`
	Integration     integrationConfig   `json:"integration"`
	Session         sessionConfig       `json:"session"`
	PatternTuning   patternTuningConfig `json:"pattern_tuning"`
}

type remoteConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type integrationConfig struct {
	XrayConfigPath string `json:"xrayConfigPath"`
	ExpectedCLI    string `json:"expectedCli"`
}

type sessionConfig struct {
	Nonce               string   `json:"nonce"`
	RotationBucket      int64    `json:"rotation_bucket"`
	SelectedFingerprint string   `json:"selected_fingerprint"`
	SelectedSpiderX     string   `json:"selected_spider_x"`
	FingerprintPool     []string `json:"fingerprint_pool"`
	CoverPathPool       []string `json:"cover_path_pool"`
}

type patternTuningConfig struct {
	PaddingProfile     string `json:"padding_profile"`
	JitterWindowMs     int    `json:"jitter_window_ms"`
	PaddingMinBytes    int    `json:"padding_min_bytes"`
	PaddingMaxBytes    int    `json:"padding_max_bytes"`
	BurstIntervalMinMs int    `json:"burst_interval_min_ms"`
	BurstIntervalMaxMs int    `json:"burst_interval_max_ms"`
	IdleGapMinMs       int    `json:"idle_gap_min_ms"`
	IdleGapMaxMs       int    `json:"idle_gap_max_ms"`
}

type socksEndpoint struct {
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func loadConfig(path string) (*config, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg config
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	if cfg.Mode == "" {
		return nil, errors.New("mode is required")
	}
	if cfg.Seed == "" {
		return nil, errors.New("seed is required")
	}
	if cfg.Remote.Address == "" || cfg.Remote.Port == 0 {
		return nil, errors.New("remote.address and remote.port are required")
	}
	return &cfg, nil
}

func (cfg *config) hydrateRuntimeEndpoints() error {
	cfg.Listen.normalizeLoopback()
	cfg.Upstream.normalizeLoopback()
	if cfg.Upstream.isConfigured() {
		return nil
	}
	if cfg.Integration.XrayConfigPath == "" {
		return nil
	}

	endpoint, err := loadSocksEndpointFromXrayConfig(cfg.Integration.XrayConfigPath)
	if err != nil {
		return err
	}
	cfg.Upstream = endpoint
	cfg.Upstream.normalizeLoopback()
	return nil
}

func (cfg *config) proxyRuntimeReady() bool {
	return cfg.Listen.isConfigured() && cfg.Upstream.isConfigured()
}

func (endpoint *socksEndpoint) normalizeLoopback() {
	if endpoint.Address == "" {
		endpoint.Address = "127.0.0.1"
	}
}

func (endpoint socksEndpoint) isConfigured() bool {
	return endpoint.Port > 0
}

func (endpoint socksEndpoint) requiresAuth() bool {
	return endpoint.Username != "" || endpoint.Password != ""
}

func loadSocksEndpointFromXrayConfig(path string) (socksEndpoint, error) {
	type xrayAccount struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	type xraySettings struct {
		Auth     string        `json:"auth"`
		Accounts []xrayAccount `json:"accounts"`
	}
	type xrayInbound struct {
		Protocol string       `json:"protocol"`
		Listen   string       `json:"listen"`
		Port     int          `json:"port"`
		Settings xraySettings `json:"settings"`
	}
	type xrayDocument struct {
		Inbounds []xrayInbound `json:"inbounds"`
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return socksEndpoint{}, fmt.Errorf("read xray config %s: %w", path, err)
	}

	var doc xrayDocument
	if err := json.Unmarshal(payload, &doc); err != nil {
		return socksEndpoint{}, fmt.Errorf("decode xray config %s: %w", path, err)
	}

	for _, inbound := range doc.Inbounds {
		if inbound.Protocol != "socks" || inbound.Port == 0 {
			continue
		}
		endpoint := socksEndpoint{
			Address: inbound.Listen,
			Port:    inbound.Port,
		}
		if inbound.Settings.Auth == "password" && len(inbound.Settings.Accounts) > 0 {
			endpoint.Username = inbound.Settings.Accounts[0].User
			endpoint.Password = inbound.Settings.Accounts[0].Pass
		}
		endpoint.normalizeLoopback()
		return endpoint, nil
	}

	return socksEndpoint{}, fmt.Errorf("no SOCKS inbound found in %s", path)
}
