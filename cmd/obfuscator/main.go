package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

const version = "0.1.0-scaffold"

type config struct {
	Mode            string              `json:"mode"`
	Seed            string              `json:"seed"`
	TrafficStrategy string              `json:"traffic_strategy"`
	PatternStrategy string              `json:"pattern_strategy"`
	Remote          remoteConfig        `json:"remote"`
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

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[obfuscator] ")

	configPath, showVersion, err := parseFlags(os.Args[1:])
	if err != nil {
		log.Fatalf("argument error: %v", err)
	}
	if showVersion {
		fmt.Printf("obfuscator %s\n", version)
		return
	}
	if configPath == "" {
		log.Fatal("missing required --config <path>")
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	absConfigPath, _ := filepath.Abs(configPath)
	log.Printf("starting scaffold runtime")
	log.Printf("config=%s", absConfigPath)
	log.Printf("mode=%s remote=%s:%d seed_length=%d", cfg.Mode, cfg.Remote.Address, cfg.Remote.Port, len(cfg.Seed))
	if cfg.TrafficStrategy != "" || cfg.PatternStrategy != "" {
		log.Printf("strategies traffic=%s pattern=%s", cfg.TrafficStrategy, cfg.PatternStrategy)
	}
	if cfg.Integration.XrayConfigPath != "" {
		log.Printf("xray_config=%s", cfg.Integration.XrayConfigPath)
	}
	if cfg.Integration.ExpectedCLI != "" {
		log.Printf("expected_cli=%s", cfg.Integration.ExpectedCLI)
	}
	if cfg.Session.Nonce != "" {
		log.Printf(
			"session nonce=%s rotation_bucket=%d fingerprint=%s spider_x=%s",
			cfg.Session.Nonce,
			cfg.Session.RotationBucket,
			cfg.Session.SelectedFingerprint,
			cfg.Session.SelectedSpiderX,
		)
	}
	if len(cfg.Session.FingerprintPool) > 0 || len(cfg.Session.CoverPathPool) > 0 {
		log.Printf(
			"session_pools fingerprints=%d cover_paths=%d",
			len(cfg.Session.FingerprintPool),
			len(cfg.Session.CoverPathPool),
		)
	}
	if cfg.PatternTuning.PaddingProfile != "" || cfg.PatternTuning.JitterWindowMs > 0 {
		log.Printf(
			"pattern_tuning padding=%s jitter_ms=%d padding_bytes=%d-%d burst_ms=%d-%d idle_ms=%d-%d",
			cfg.PatternTuning.PaddingProfile,
			cfg.PatternTuning.JitterWindowMs,
			cfg.PatternTuning.PaddingMinBytes,
			cfg.PatternTuning.PaddingMaxBytes,
			cfg.PatternTuning.BurstIntervalMinMs,
			cfg.PatternTuning.BurstIntervalMaxMs,
			cfg.PatternTuning.IdleGapMinMs,
			cfg.PatternTuning.IdleGapMaxMs,
		)
	}
	log.Printf("note=this is a placeholder obfuscator binary; replace with the real module 1 implementation when available")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	log.Printf("shutdown complete")
}

func parseFlags(args []string) (string, bool, error) {
	fs := flag.NewFlagSet("obfuscator", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "path to obfuscator config file")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return "", false, err
	}
	if fs.NArg() > 0 {
		return "", false, fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	return *configPath, *showVersion, nil
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
