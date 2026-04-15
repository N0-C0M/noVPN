package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"novpn/internal/config"
	"novpn/internal/core/reality"
	"novpn/internal/observability"
)

type AdminService struct {
	cfg        config.Config
	logger     *slog.Logger
	reality    *reality.Provisioner
	metrics    *observability.Metrics
	httpServer *http.Server
	health     *http.Server
	ready      atomic.Bool
}

func NewAdminService(cfg config.Config, logger *slog.Logger) (*AdminService, error) {
	if !cfg.Admin.Enabled {
		return nil, fmt.Errorf("admin.enabled must be true for admin-service")
	}

	metrics := observability.NewMetrics()
	provisioner := reality.NewProvisioner(cfg.Core.Reality, logger)
	service := &AdminService{
		cfg:        cfg,
		logger:     logger.With("service", "admin-service"),
		reality:    provisioner,
		metrics:    metrics,
		httpServer: newAdminServer(cfg.Admin, provisioner, metrics, logger),
	}
	service.health = observability.NewHTTPServer(
		cfg.Observability.HealthAddr,
		cfg.Observability.MetricsPath,
		metrics.Registry(),
		func() bool { return service.ready.Load() },
		logger.With("component", "admin-health"),
	)
	return service, nil
}

func (s *AdminService) Start(context.Context) error {
	s.ready.Store(true)

	if s.health != nil {
		go func() {
			if err := s.health.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Error("admin health server stopped unexpectedly", "error", err)
			}
		}()
	}
	if s.httpServer != nil {
		go func() {
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Error("admin server stopped unexpectedly", "error", err)
			}
		}()
	}

	s.logger.Info("admin service started", "listen_addr", s.cfg.Admin.ListenAddr, "runtime_mode", s.cfg.Admin.RuntimeMode)
	return nil
}

func (s *AdminService) Shutdown(ctx context.Context) error {
	s.ready.Store(false)

	var firstErr error
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil && err != http.ErrServerClosed && firstErr == nil {
			firstErr = err
		}
	}
	if s.health != nil {
		if err := s.health.Shutdown(ctx); err != nil && err != http.ErrServerClosed && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
