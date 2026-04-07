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
	Mode        string            `json:"mode"`
	Seed        string            `json:"seed"`
	Remote      remoteConfig      `json:"remote"`
	Integration integrationConfig `json:"integration"`
}

type remoteConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type integrationConfig struct {
	XrayConfigPath string `json:"xrayConfigPath"`
	ExpectedCLI    string `json:"expectedCli"`
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
	if cfg.Integration.XrayConfigPath != "" {
		log.Printf("xray_config=%s", cfg.Integration.XrayConfigPath)
	}
	if cfg.Integration.ExpectedCLI != "" {
		log.Printf("expected_cli=%s", cfg.Integration.ExpectedCLI)
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
