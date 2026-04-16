package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const serverMonitoringRefreshInterval = time.Hour

type serverMonitorTarget struct {
	ServerID   string
	Name       string
	Address    string
	Role       string
	Purpose    string
	MonitorURL string
}

type serverMonitorRecord struct {
	ServerID   string               `json:"server_id"`
	Name       string               `json:"name"`
	Address    string               `json:"address,omitempty"`
	Role       string               `json:"role,omitempty"`
	Purpose    string               `json:"purpose,omitempty"`
	MonitorURL string               `json:"monitor_url,omitempty"`
	CheckedAt  time.Time            `json:"checked_at"`
	Healthy    bool                 `json:"healthy"`
	Error      string               `json:"error,omitempty"`
	Status     systemStatusSnapshot `json:"status"`
}

type serverMonitorSnapshot struct {
	UpdatedAt time.Time             `json:"updated_at"`
	Servers   []serverMonitorRecord `json:"servers,omitempty"`
}

type serverMonitorStore struct {
	path   string
	token  string
	logger *slog.Logger
	client *http.Client
	mu     sync.Mutex
	stop   chan struct{}
	done   chan struct{}
}

func newServerMonitorStore(path string, token string, logger *slog.Logger) *serverMonitorStore {
	return &serverMonitorStore{
		path:   path,
		token:  strings.TrimSpace(token),
		logger: logger,
		client: &http.Client{Timeout: 15 * time.Second},
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (s *serverMonitorStore) StartAutoRefresh(targets func() []serverMonitorTarget, interval time.Duration) {
	if interval <= 0 {
		interval = serverMonitoringRefreshInterval
	}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if _, err := s.Refresh(ctx, targets()); err != nil && s.logger != nil {
					s.logger.Warn("server monitoring refresh failed", "error", err)
				}
				cancel()
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *serverMonitorStore) Stop() {
	close(s.stop)
	<-s.done
}

func (s *serverMonitorStore) Load() (serverMonitorSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *serverMonitorStore) EnsureFresh(ctx context.Context, targets []serverMonitorTarget, maxAge time.Duration, force bool) (serverMonitorSnapshot, error) {
	if maxAge <= 0 {
		maxAge = serverMonitoringRefreshInterval
	}
	snapshot, err := s.Load()
	if err != nil {
		if !force {
			return s.Refresh(ctx, targets)
		}
	}
	if force || snapshot.UpdatedAt.IsZero() || time.Since(snapshot.UpdatedAt) >= maxAge {
		return s.Refresh(ctx, targets)
	}
	return snapshot, nil
}

func (s *serverMonitorStore) Refresh(ctx context.Context, targets []serverMonitorTarget) (serverMonitorSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, _ := s.loadLocked()
	previousByID := make(map[string]serverMonitorRecord, len(previous.Servers))
	for _, record := range previous.Servers {
		previousByID[record.ServerID] = record
	}

	snapshot := serverMonitorSnapshot{
		UpdatedAt: time.Now().UTC(),
		Servers:   make([]serverMonitorRecord, 0, len(targets)),
	}
	for _, target := range targets {
		record := serverMonitorRecord{
			ServerID:   target.ServerID,
			Name:       target.Name,
			Address:    target.Address,
			Role:       target.Role,
			Purpose:    target.Purpose,
			MonitorURL: strings.TrimSpace(target.MonitorURL),
			CheckedAt:  time.Now().UTC(),
		}
		if err := s.fetchStatus(ctx, target, &record); err != nil {
			record.Healthy = false
			record.Error = err.Error()
			if previousRecord, ok := previousByID[target.ServerID]; ok {
				record.Status = previousRecord.Status
			}
		}
		snapshot.Servers = append(snapshot.Servers, record)
	}
	if err := s.saveLocked(snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (s *serverMonitorStore) fetchStatus(ctx context.Context, target serverMonitorTarget, record *serverMonitorRecord) error {
	endpoint := strings.TrimSpace(target.MonitorURL)
	if endpoint == "" {
		return fmt.Errorf("monitor url is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if s.token != "" {
		req.Header.Set("X-Control-Plane-Token", s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("monitor returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var snapshot systemStatusSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return err
	}

	record.Healthy = true
	record.Error = ""
	record.Status = snapshot
	return nil
}

func (s *serverMonitorStore) loadLocked() (serverMonitorSnapshot, error) {
	payload, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return serverMonitorSnapshot{}, nil
		}
		return serverMonitorSnapshot{}, fmt.Errorf("read server monitoring snapshot: %w", err)
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return serverMonitorSnapshot{}, nil
	}
	var snapshot serverMonitorSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return serverMonitorSnapshot{}, fmt.Errorf("unmarshal server monitoring snapshot: %w", err)
	}
	return snapshot, nil
}

func (s *serverMonitorStore) saveLocked(snapshot serverMonitorSnapshot) error {
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal server monitoring snapshot: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create server monitoring directory: %w", err)
	}
	if err := os.WriteFile(s.path, payload, 0o600); err != nil {
		return fmt.Errorf("write server monitoring snapshot: %w", err)
	}
	return nil
}
