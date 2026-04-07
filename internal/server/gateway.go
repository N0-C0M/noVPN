package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"novpn/internal/acl"
	"novpn/internal/auth"
	"novpn/internal/config"
	"novpn/internal/core/reality"
	"novpn/internal/observability"
	"novpn/internal/ratelimit"
	tcpproxy "novpn/internal/transport/tcp"
	udpproxy "novpn/internal/transport/udp"
	"novpn/internal/upstream"
)

type Gateway struct {
	cfg            config.Config
	logger         *slog.Logger
	metrics        *observability.Metrics
	httpServer     *http.Server
	adminServer    *http.Server
	readinessTimer *time.Timer
	ready          atomic.Bool
	tcpProxies     []*tcpproxy.Proxy
	udpProxies     []*udpproxy.Proxy
	reality        *reality.Provisioner
}

func New(cfg config.Config, logger *slog.Logger) (*Gateway, error) {
	metrics := observability.NewMetrics()
	dialer := upstream.NewDirectDialer(cfg.Server.UpstreamDialTimeout)
	authManager := auth.NoopManager{}
	aclEvaluator := acl.AllowAllEvaluator{}

	gateway := &Gateway{
		cfg:     cfg,
		logger:  logger,
		metrics: metrics,
	}

	if cfg.Core.Reality.Enabled {
		gateway.reality = reality.NewProvisioner(cfg.Core.Reality, logger)
	}

	gateway.httpServer = observability.NewHTTPServer(
		cfg.Observability.HealthAddr,
		cfg.Observability.MetricsPath,
		metrics.Registry(),
		gateway.Ready,
		logger.With("component", "health"),
	)

	for _, listener := range cfg.Listeners.TCP {
		if !listener.Enabled {
			continue
		}
		limiter := ratelimit.NewMemoryLimiter(listener.Limits.MaxConnections, listener.Limits.PerIPConnection)
		gateway.tcpProxies = append(gateway.tcpProxies, tcpproxy.NewProxy(listener, tcpproxy.Dependencies{
			Auth:    authManager,
			ACL:     aclEvaluator,
			Limiter: limiter,
			Dialer:  dialer,
			Metrics: metrics,
			Logger:  logger.With("component", "tcp", "listener", listener.Name),
		}))
	}

	for _, listener := range cfg.Listeners.UDP {
		if !listener.Enabled {
			continue
		}
		gateway.udpProxies = append(gateway.udpProxies, udpproxy.NewProxy(listener, udpproxy.Dependencies{
			Auth:    authManager,
			ACL:     aclEvaluator,
			Limiter: ratelimit.NoopLimiter{},
			Dialer:  dialer,
			Metrics: metrics,
			Logger:  logger.With("component", "udp", "listener", listener.Name),
		}))
	}

	if cfg.Admin.Enabled {
		if gateway.reality == nil {
			return nil, fmt.Errorf("admin requires core.reality.enabled")
		}
		gateway.adminServer = newAdminServer(cfg.Admin, gateway.reality, metrics, logger)
	}

	if len(gateway.tcpProxies) == 0 && len(gateway.udpProxies) == 0 {
		return nil, fmt.Errorf("no enabled listeners configured")
	}

	return gateway, nil
}

func (g *Gateway) Start(_ context.Context) (err error) {
	if g.httpServer != nil {
		go func() {
			if serveErr := g.httpServer.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
				g.logger.Error("health server stopped unexpectedly", "error", serveErr)
			}
		}()
	}
	if g.adminServer != nil {
		go func() {
			if serveErr := g.adminServer.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
				g.logger.Error("admin server stopped unexpectedly", "error", serveErr)
			}
		}()
	}

	startedTCP := 0
	startedUDP := 0
	defer func() {
		if err == nil {
			return
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for i := 0; i < startedTCP; i++ {
			_ = g.tcpProxies[i].Shutdown(stopCtx)
		}
		for i := 0; i < startedUDP; i++ {
			_ = g.udpProxies[i].Shutdown(stopCtx)
		}
		if g.adminServer != nil {
			_ = g.adminServer.Shutdown(stopCtx)
		}
		if g.httpServer != nil {
			_ = g.httpServer.Shutdown(stopCtx)
		}
	}()

	for _, proxy := range g.tcpProxies {
		if err = proxy.Start(); err != nil {
			return fmt.Errorf("start tcp listener %q: %w", proxy.Name(), err)
		}
		startedTCP++
	}

	for _, proxy := range g.udpProxies {
		if err = proxy.Start(); err != nil {
			return fmt.Errorf("start udp listener %q: %w", proxy.Name(), err)
		}
		startedUDP++
	}

	if g.cfg.Server.ReadinessGracePeriod > 0 {
		g.readinessTimer = time.AfterFunc(g.cfg.Server.ReadinessGracePeriod, func() {
			g.ready.Store(true)
		})
	} else {
		g.ready.Store(true)
	}

	g.logger.Info("gateway started", "tcp_listeners", len(g.tcpProxies), "udp_listeners", len(g.udpProxies))
	return nil
}

func (g *Gateway) Shutdown(ctx context.Context) error {
	g.metrics.ShutdownInProgress.Set(1)
	defer g.metrics.ShutdownInProgress.Set(0)

	if g.readinessTimer != nil {
		g.readinessTimer.Stop()
	}
	g.ready.Store(false)

	var firstErr error

	for _, proxy := range g.tcpProxies {
		if err := proxy.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	for _, proxy := range g.udpProxies {
		if err := proxy.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if g.httpServer != nil {
		if err := g.httpServer.Shutdown(ctx); err != nil && err != http.ErrServerClosed && firstErr == nil {
			firstErr = err
		}
	}
	if g.adminServer != nil {
		if err := g.adminServer.Shutdown(ctx); err != nil && err != http.ErrServerClosed && firstErr == nil {
			firstErr = err
		}
	}

	g.logger.Info("gateway stopped")
	return firstErr
}

func (g *Gateway) Ready() bool {
	return g.ready.Load()
}
