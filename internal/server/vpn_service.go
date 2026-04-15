package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"novpn/internal/config"
	"novpn/internal/core/reality"
)

type VPNService struct {
	cfg     config.Config
	logger  *slog.Logger
	gateway *Gateway
	syncer  *controlPlaneSyncer
}

func NewVPNService(cfg config.Config, logger *slog.Logger) (*VPNService, error) {
	cfg.Admin.Enabled = false

	gateway, err := New(cfg, logger)
	if err != nil {
		return nil, err
	}

	service := &VPNService{
		cfg:     cfg,
		logger:  logger.With("service", "vpn-service"),
		gateway: gateway,
	}
	if cfg.ControlPlane.Enabled {
		if gateway.reality == nil {
			return nil, fmt.Errorf("control_plane.enabled requires core.reality.enabled")
		}
		service.syncer = newControlPlaneSyncer(cfg.ControlPlane, gateway.reality, logger)
	}
	return service, nil
}

func (s *VPNService) Start(ctx context.Context) error {
	if s.syncer != nil {
		if err := s.syncer.SyncOnce(ctx); err != nil {
			s.logger.Warn("initial control-plane sync failed", "error", err)
		}
	} else if s.gateway.reality != nil {
		if _, err := s.gateway.reality.Bootstrap(ctx, reality.Options{
			InstallXray:    false,
			ValidateConfig: false,
			ManageService:  true,
		}); err != nil {
			return err
		}
	}

	if err := s.gateway.Start(ctx); err != nil {
		return err
	}
	if s.syncer != nil {
		s.syncer.Start()
	}
	return nil
}

func (s *VPNService) Shutdown(ctx context.Context) error {
	if s.syncer != nil {
		s.syncer.Stop()
	}
	return s.gateway.Shutdown(ctx)
}

type controlPlaneSyncer struct {
	cfg         config.ControlPlaneConfig
	provisioner *reality.Provisioner
	logger      *slog.Logger
	client      *http.Client
	stop        chan struct{}
	done        chan struct{}
}

func newControlPlaneSyncer(cfg config.ControlPlaneConfig, provisioner *reality.Provisioner, logger *slog.Logger) *controlPlaneSyncer {
	return &controlPlaneSyncer{
		cfg:         cfg,
		provisioner: provisioner,
		logger:      logger.With("component", "control-plane-sync"),
		client: &http.Client{
			Timeout: 12 * time.Second,
		},
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (s *controlPlaneSyncer) Start() {
	go s.loop()
}

func (s *controlPlaneSyncer) Stop() {
	close(s.stop)
	<-s.done
}

func (s *controlPlaneSyncer) loop() {
	defer close(s.done)
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), s.cfg.PollInterval)
			if err := s.SyncOnce(ctx); err != nil {
				s.logger.Warn("control-plane sync failed", "error", err)
			}
			cancel()
		case <-s.stop:
			return
		}
	}
}

func (s *controlPlaneSyncer) SyncOnce(ctx context.Context) error {
	if err := s.syncRegistry(ctx); err != nil {
		return err
	}
	if err := s.pushTraffic(ctx); err != nil {
		s.logger.Debug("traffic push skipped", "error", err)
	}
	return nil
}

func (s *controlPlaneSyncer) syncRegistry(ctx context.Context) error {
	endpoint := s.controlPlaneEndpoint("/control-plane/registry")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Control-Plane-Token", s.cfg.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registry sync returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var registry reality.Registry
	if err := json.NewDecoder(resp.Body).Decode(&registry); err != nil {
		return err
	}

	_, _, err = s.provisioner.ApplyRemoteRegistry(ctx, registry)
	return err
}

func (s *controlPlaneSyncer) pushTraffic(ctx context.Context) error {
	usages, err := s.provisioner.ExportTrafficUsages(ctx)
	if err != nil {
		return err
	}
	if len(usages) == 0 {
		return nil
	}

	payload := struct {
		Usages map[string]int64 `json:"usages"`
	}{
		Usages: make(map[string]int64, len(usages)),
	}
	for email, usage := range usages {
		payload.Usages[email] = usage.TotalBytes
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.controlPlaneEndpoint("/control-plane/traffic"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Control-Plane-Token", s.cfg.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("traffic push returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return nil
}

func (s *controlPlaneSyncer) controlPlaneEndpoint(path string) string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.BaseURL), "/")
	return base + path
}
