package reality

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const defaultXrayStatsAPIListen = "127.0.0.1:10085"

type xrayStatsResponse struct {
	Stat []struct {
		Name  string          `json:"name"`
		Value json.RawMessage `json:"value"`
	} `json:"stat"`
}

func (p *Provisioner) SyncTraffic(ctx context.Context) (TrafficSyncResult, error) {
	result, err := p.collectTraffic(ctx)
	if err != nil {
		return TrafficSyncResult{}, err
	}
	if result.RequiresRefresh {
		if _, err := p.Bootstrap(ctx, Options{
			InstallXray:    false,
			ValidateConfig: false,
			ManageService:  true,
		}); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (p *Provisioner) refreshRuntime(ctx context.Context) (Result, error) {
	if _, err := p.collectTraffic(ctx); err != nil {
		p.logger.Warn("traffic sync before refresh failed", "error", err)
	}
	return p.Bootstrap(ctx, Options{
		InstallXray:    false,
		ValidateConfig: false,
		ManageService:  true,
	})
}

func (p *Provisioner) collectTraffic(ctx context.Context) (TrafficSyncResult, error) {
	usages, err := p.queryTrafficUsages(ctx)
	if err != nil {
		return TrafficSyncResult{}, err
	}
	if len(usages) == 0 {
		return TrafficSyncResult{}, nil
	}
	return p.registryStore.ApplyTrafficStats(usages)
}

func (p *Provisioner) queryTrafficUsages(ctx context.Context) (map[string]TrafficUsage, error) {
	commandCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		commandCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(
		commandCtx,
		p.cfg.Xray.BinaryPath,
		"api",
		"statsquery",
		"--server="+defaultXrayStatsAPIListen,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("query xray traffic stats: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var response xrayStatsResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("decode xray traffic stats: %w", err)
	}

	observedAt := time.Now().UTC()
	usages := make(map[string]TrafficUsage)
	for _, stat := range response.Stat {
		email, ok := extractTrafficEmail(stat.Name)
		if !ok {
			continue
		}
		value, err := parseStatValue(stat.Value)
		if err != nil {
			return nil, fmt.Errorf("parse xray stat %q: %w", stat.Name, err)
		}
		usage := usages[email]
		usage.TotalBytes += value
		usage.ObservedAt = observedAt
		usages[email] = usage
	}
	return usages, nil
}

func extractTrafficEmail(name string) (string, bool) {
	if !strings.HasPrefix(name, "user>>>") || !strings.Contains(name, ">>>traffic>>>") {
		return "", false
	}
	remainder := strings.TrimPrefix(name, "user>>>")
	parts := strings.Split(remainder, ">>>")
	if len(parts) < 3 {
		return "", false
	}
	email := strings.TrimSpace(parts[0])
	if email == "" {
		return "", false
	}
	if parts[1] != "traffic" {
		return "", false
	}
	switch parts[2] {
	case "uplink", "downlink":
		return email, true
	default:
		return "", false
	}
}

func parseStatValue(raw json.RawMessage) (int64, error) {
	var number int64
	if err := json.Unmarshal(raw, &number); err == nil {
		return number, nil
	}

	var stringNumber string
	if err := json.Unmarshal(raw, &stringNumber); err == nil {
		value, parseErr := strconv.ParseInt(strings.TrimSpace(stringNumber), 10, 64)
		if parseErr != nil {
			return 0, parseErr
		}
		return value, nil
	}

	return 0, fmt.Errorf("unsupported stat value %s", string(raw))
}
