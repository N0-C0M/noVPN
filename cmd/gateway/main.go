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
	flag.StringVar(&configPath, "config", "deploy/config.example.yaml", "path to YAML config")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger := observability.NewLogger(cfg.Observability)
	gateway, err := server.New(cfg, logger)
	if err != nil {
		logger.Error("initialize gateway", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := gateway.Start(ctx); err != nil {
		logger.Error("start gateway", "error", err)
		os.Exit(1)
	}

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := gateway.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown gateway", "error", err)
		os.Exit(1)
	}
}
