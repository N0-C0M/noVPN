package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"novpn/internal/config"
	"novpn/internal/observability"
	"novpn/internal/server"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "deploy/vpn-service/config.yaml", "path to YAML config")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger := observability.NewLogger(cfg.Observability)
	service, err := server.NewVPNService(cfg, logger)
	if err != nil {
		logger.Error("initialize vpn-service", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := service.Start(ctx); err != nil {
		logger.Error("start vpn-service", "error", err)
		os.Exit(1)
	}

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := service.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown vpn-service", "error", err)
		os.Exit(1)
	}
}
