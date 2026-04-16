package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"novpn/internal/controlplane"
)

type dashboardViewV2 struct {
	BasePath              string
	GeneratedAt           time.Time
	PanelVersion          string
	DashboardTab          string
	Tabs                  []dashboardTabLink
	Summary               any
	Clients               any
	Invites               any
	Promos                any
	ClientSort            string
	Metrics               []metricRow
	ServerStats           []metricRow
	HealthAddr            string
	RegistryPath          string
	ClientProfilePath     string
	ConfigPath            string
	StatePath             string
	PingURL               string
	DownloadURL           string
	UploadURL             string
	SiteImageURL          string
	Ready                 bool
	Notice                string
	ClientPolicy          any
	MandatoryNotices      any
	ClientPolicyURL       string
	ClientNoticesURL      string
	Plans                 []controlplane.SubscriptionPlan
	PlanPageURL           string
	ServerPageURL         string
	MonitoringObservedAt  string
	MonitoringRefreshURL  string
	MonitoringRows        []serverMonitoringView
	InfrastructureRows    []infrastructureHostView
	VPNInventoryRows      []vpnServerInventoryView
	ServerMonitoringCount int
}

type dashboardTabLink struct {
	ID     string
	Label  string
	URL    string
	Active bool
}

type serverMonitoringView struct {
	Name       string
	ServerID   string
	Role       string
	Purpose    string
	Endpoint   string
	MonitorURL string
	Health     string
	Hostname   string
	ObservedAt string
	CPU        string
	Memory     string
	Disk       string
	Uptime     string
	Error      string
}

type infrastructureHostView struct {
	Name      string
	Host      string
	Services  string
	Purpose   string
	PublicURL string
	Health    string
}

type vpnServerInventoryView struct {
	ID          string
	Name        string
	Role        string
	Purpose     string
	Endpoint    string
	Location    string
	MonitorURL  string
	AssignedTo  string
	Health      string
	ObservedAt  string
	CPU         string
	Memory      string
	Disk        string
	MonitorNote string
}

type serverPageViewV2 struct {
	BasePath             string
	MonitoringRefreshURL string
	MonitoringObservedAt string
	InfrastructureRows   []infrastructureHostView
	VPNInventoryRows     []vpnServerInventoryView
	Servers              []controlplane.ServerNode
}

func normalizeDashboardTab(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "clients":
		return "clients"
	case "access":
		return "access"
	case "policies":
		return "policies"
	case "monitoring":
		return "monitoring"
	case "servers":
		return "servers"
	default:
		return "overview"
	}
}

func (a *adminApp) dashboardTabs(activeTab string) []dashboardTabLink {
	tabs := []dashboardTabLink{
		{ID: "overview", Label: "Overview"},
		{ID: "clients", Label: "Clients"},
		{ID: "access", Label: "Access"},
		{ID: "policies", Label: "Policies"},
		{ID: "monitoring", Label: "Monitoring"},
		{ID: "servers", Label: "Servers"},
	}
	for index := range tabs {
		tabs[index].URL = fmt.Sprintf("%s/dashboard?tab=%s", a.basePath, tabs[index].ID)
		tabs[index].Active = tabs[index].ID == activeTab
	}
	return tabs
}

func (a *adminApp) serverMonitoringPath() string {
	return filepath.Join(a.cfg.StoragePath, "server-monitoring.json")
}

func (a *adminApp) monitorTargets(servers []controlplane.ServerNode) []serverMonitorTarget {
	targets := make([]serverMonitorTarget, 0, len(servers))
	for _, server := range servers {
		if !server.Active {
			continue
		}
		targets = append(targets, serverMonitorTarget{
			ServerID:   server.ID,
			Name:       server.Name,
			Address:    server.Address,
			Role:       server.Role,
			Purpose:    server.Purpose,
			MonitorURL: strings.TrimSpace(server.MonitorURL),
		})
	}
	return targets
}

func (a *adminApp) ensureServerMonitoring(ctx context.Context, servers []controlplane.ServerNode, force bool) (serverMonitorSnapshot, error) {
	if a.serverMonitor == nil {
		return serverMonitorSnapshot{}, nil
	}
	return a.serverMonitor.EnsureFresh(ctx, a.monitorTargets(servers), serverMonitoringRefreshInterval, force)
}

func buildServerMonitoringViews(servers []controlplane.ServerNode, snapshot serverMonitorSnapshot) []serverMonitoringView {
	records := make(map[string]serverMonitorRecord, len(snapshot.Servers))
	for _, record := range snapshot.Servers {
		records[record.ServerID] = record
	}

	rows := make([]serverMonitoringView, 0, len(servers))
	for _, server := range servers {
		record, ok := records[server.ID]
		if !ok {
			rows = append(rows, serverMonitoringView{
				Name:       server.Name,
				ServerID:   server.ID,
				Role:       server.Role,
				Purpose:    server.Purpose,
				Endpoint:   fmt.Sprintf("%s:%d", server.Address, server.Port),
				MonitorURL: server.MonitorURL,
				Health:     "unknown",
				Error:      "No monitoring data yet",
			})
			continue
		}
		rows = append(rows, serverMonitoringView{
			Name:       server.Name,
			ServerID:   server.ID,
			Role:       server.Role,
			Purpose:    server.Purpose,
			Endpoint:   fmt.Sprintf("%s:%d", server.Address, server.Port),
			MonitorURL: record.MonitorURL,
			Health:     formatMonitorHealth(record),
			Hostname:   strings.TrimSpace(record.Status.Hostname),
			ObservedAt: formatMonitorTimestamp(record.Status.ObservedAt),
			CPU:        formatCPULoad(record.Status),
			Memory:     formatUsage(record.Status.MemoryUsedBytes, record.Status.MemoryTotalBytes),
			Disk:       formatUsage(record.Status.DiskUsedBytes, record.Status.DiskTotalBytes),
			Uptime:     formatUptime(record.Status.UptimeSeconds),
			Error:      record.Error,
		})
	}
	return rows
}

func buildVPNInventoryViews(servers []controlplane.ServerNode, plans []controlplane.SubscriptionPlan, snapshot serverMonitorSnapshot) []vpnServerInventoryView {
	planMap := make(map[string][]string)
	for _, plan := range plans {
		for _, serverID := range plan.ServerIDs {
			planMap[serverID] = append(planMap[serverID], plan.Name)
		}
	}
	monitorRows := buildServerMonitoringViews(servers, snapshot)
	monitorMap := make(map[string]serverMonitoringView, len(monitorRows))
	for _, row := range monitorRows {
		monitorMap[row.ServerID] = row
	}

	rows := make([]vpnServerInventoryView, 0, len(servers))
	for _, server := range servers {
		monitor := monitorMap[server.ID]
		assignedTo := "not assigned"
		if values := planMap[server.ID]; len(values) > 0 {
			sort.Strings(values)
			assignedTo = strings.Join(values, ", ")
		}
		rows = append(rows, vpnServerInventoryView{
			ID:          server.ID,
			Name:        server.Name,
			Role:        server.Role,
			Purpose:     server.Purpose,
			Endpoint:    fmt.Sprintf("%s:%d", server.Address, server.Port),
			Location:    server.LocationLabel,
			MonitorURL:  server.MonitorURL,
			AssignedTo:  assignedTo,
			Health:      monitor.Health,
			ObservedAt:  monitor.ObservedAt,
			CPU:         monitor.CPU,
			Memory:      monitor.Memory,
			Disk:        monitor.Disk,
			MonitorNote: monitor.Error,
		})
	}
	return rows
}

func (a *adminApp) infrastructureRows() []infrastructureHostView {
	host := strings.TrimSpace(extractURLHost(a.publicBaseURL()))
	if host == "" {
		host = strings.TrimSpace(a.cfg.ListenAddr)
	}
	healthSummary := "admin-service online"
	if !checkHTTPHealth("http://127.0.0.1:9120/healthz") {
		healthSummary += " · pay-service unavailable"
	} else {
		healthSummary += " · pay-service online"
	}
	return []infrastructureHostView{
		{
			Name:      "Admin / control-plane host",
			Host:      host,
			Services:  "admin-service, control-plane API, pay-service",
			Purpose:   "Admin panel, subscription catalog, invite activation, payment service",
			PublicURL: a.publicBaseURL(),
			Health:    healthSummary,
		},
	}
}

func extractURLHost(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return parsed.Host
}

func checkHTTPHealth(endpoint string) bool {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func formatMonitorHealth(record serverMonitorRecord) string {
	switch {
	case !record.Healthy && strings.TrimSpace(record.Error) != "":
		return "unreachable"
	case record.Status.Ready:
		return "ready"
	default:
		return "degraded"
	}
}

func formatMonitorTimestamp(value time.Time) string {
	if value.IsZero() {
		return "never"
	}
	return value.UTC().Format("2006-01-02 15:04 UTC")
}

func formatCPULoad(snapshot systemStatusSnapshot) string {
	if snapshot.CPUCores <= 0 {
		return fmt.Sprintf("%.2f / %.2f / %.2f", snapshot.Load1, snapshot.Load5, snapshot.Load15)
	}
	return fmt.Sprintf("%.2f / %.2f / %.2f (%d cores)", snapshot.Load1, snapshot.Load5, snapshot.Load15, snapshot.CPUCores)
}

func formatUsage(used uint64, total uint64) string {
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%s / %s (%.0f%%)", formatTrafficBytes(int64(used)), formatTrafficBytes(int64(total)), float64(used)*100/float64(total))
}

func formatUptime(seconds uint64) string {
	if seconds == 0 {
		return "n/a"
	}
	duration := time.Duration(seconds) * time.Second
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
