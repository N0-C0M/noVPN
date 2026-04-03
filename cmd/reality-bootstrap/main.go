package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"novpn/internal/config"
	"novpn/internal/core/reality"
	"novpn/internal/observability"
)

func main() {
	var (
		configPath   string
		renderOnly   bool
		skipInstall  bool
		skipValidate bool
		skipService  bool
	)

	flag.StringVar(&configPath, "config", "deploy/config.example.yaml", "path to YAML config")
	flag.BoolVar(&renderOnly, "render-only", false, "only render state/config/profile files without installing or restarting Xray")
	flag.BoolVar(&skipInstall, "skip-install", false, "skip automatic Xray installation")
	flag.BoolVar(&skipValidate, "skip-validate", false, "skip xray run -test validation")
	flag.BoolVar(&skipService, "skip-service", false, "skip systemd restart")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if !cfg.Core.Reality.Enabled {
		fmt.Fprintln(os.Stderr, "core.reality.enabled must be true for reality-bootstrap")
		os.Exit(1)
	}

	logger := observability.NewLogger(cfg.Observability).With("command", "reality-bootstrap")
	provisioner := reality.NewProvisioner(cfg.Core.Reality, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result, err := provisioner.Bootstrap(ctx, reality.Options{
		InstallXray:    !renderOnly && !skipInstall,
		ValidateConfig: !renderOnly && !skipValidate,
		ManageService:  !renderOnly && !skipService,
	})
	if err != nil {
		logger.Error("bootstrap reality core", "error", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout,
		"reality core ready\nconfig: %s\nstate: %s\nclient_profile: %s\nuuid: %s\npublic_key: %s\nshort_id: %s\n",
		result.ConfigPath,
		result.StatePath,
		result.ClientProfilePath,
		result.State.UUID,
		result.State.PublicKey,
		result.ClientProfile.ShortID,
	)
}
