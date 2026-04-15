package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"novpn/internal/payments"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "deploy/pay-service/config.yaml", "path to YAML config")
	flag.Parse()

	cfg, err := payments.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load pay-service config: %v\n", err)
		os.Exit(1)
	}

	service := payments.New(cfg)
	if err := service.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start pay-service: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	if err := service.Shutdown(cfg.ShutdownTimeout); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown pay-service: %v\n", err)
		os.Exit(1)
	}
}
