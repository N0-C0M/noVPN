package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

const version = "0.2.0"

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
	if err := cfg.hydrateRuntimeEndpoints(); err != nil {
		log.Fatalf("runtime endpoint error: %v", err)
	}

	absConfigPath, _ := filepath.Abs(configPath)
	log.Printf("starting runtime")
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if !cfg.proxyRuntimeReady() {
		log.Printf("listen/upstream endpoints are incomplete; entering compatibility idle mode")
		<-ctx.Done()
		log.Printf("shutdown complete")
		return
	}

	if err := runProxyRuntime(ctx, cfg); err != nil && ctx.Err() == nil {
		log.Fatalf("runtime error: %v", err)
	}
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
