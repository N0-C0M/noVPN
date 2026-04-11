package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"gopkg.in/yaml.v3"

	"novpn/internal/config"
	"novpn/internal/core/reality"
	"novpn/internal/observability"
)

type adminApp struct {
	cfg           config.AdminConfig
	reality       *reality.Provisioner
	metrics       *observability.Metrics
	logger        *slog.Logger
	dashboardTpl  *template.Template
	loginTpl      *template.Template
	basePath      string
	token         string
	cookieName    string
	httpServer    *http.Server
	siteImagePath string
}

type dashboardView struct {
	BasePath          string
	GeneratedAt       time.Time
	PanelVersion      string
	Summary           reality.RegistrySummary
	Clients           []reality.ClientRecord
	Invites           []reality.InviteRecord
	Promos            []reality.PromoRecord
	ClientSort        string
	Metrics           []metricRow
	ServerStats       []metricRow
	HealthAddr        string
	RegistryPath      string
	ClientProfilePath string
	ConfigPath        string
	StatePath         string
	PingURL           string
	DownloadURL       string
	UploadURL         string
	SiteImageURL      string
	Ready             bool
	Notice            string
}

type metricRow struct {
	Name  string
	Value string
}

func newAdminServer(cfg config.AdminConfig, realityProvisioner *reality.Provisioner, metrics *observability.Metrics, logger *slog.Logger) *http.Server {
	app := &adminApp{
		cfg:           cfg,
		reality:       realityProvisioner,
		metrics:       metrics,
		logger:        logger.With("component", "admin"),
		basePath:      strings.TrimRight(cfg.BasePath, "/"),
		token:         strings.TrimSpace(cfg.Token),
		cookieName:    "novpn_admin_token",
		siteImagePath: locateSiteImagePath(cfg.StoragePath),
	}
	if app.basePath == "" {
		app.basePath = "/admin"
	}
	funcs := template.FuncMap{
		"formatBytes":         formatTrafficBytes,
		"formatTrafficLimit":  formatTrafficLimit,
		"formatTrafficRemain": formatTrafficRemain,
	}
	app.dashboardTpl = template.Must(template.New("dashboard").Funcs(funcs).Parse(adminDashboardTemplate))
	app.loginTpl = template.Must(template.New("login").Funcs(funcs).Parse(adminLoginTemplate))
	app.httpServer = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: app.routes(),
	}
	return app.httpServer
}

func (a *adminApp) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/image.png", a.handleSiteImage)
	mux.HandleFunc("/", a.redirectDashboard)
	mux.HandleFunc(a.basePath+"/", a.routeBySuffix)
	mux.HandleFunc(a.basePath, a.redirectDashboard)
	return a.withLogging(mux)
}

func (a *adminApp) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		a.logger.Debug("admin request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (a *adminApp) redirectDashboard(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, a.basePath+"/dashboard", http.StatusFound)
}

func (a *adminApp) routeBySuffix(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, a.basePath+"/login") {
		a.handleLogin(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, a.basePath+"/diag/") {
		a.handlePublicDiagnostics(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, a.basePath+"/redeem/") {
		a.handlePublicRedeem(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, a.basePath+"/disconnect") {
		a.handlePublicDisconnect(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, a.basePath+"/logout") {
		a.handleLogout(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, a.basePath+"/api/") {
		a.requireAuth(http.HandlerFunc(a.handleAPI)).ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, a.basePath+"/dashboard") {
		a.requireAuth(http.HandlerFunc(a.handleDashboard)).ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func (a *adminApp) handleLogin(w http.ResponseWriter, r *http.Request) {
	if a.token == "" {
		http.Redirect(w, r, a.basePath+"/dashboard", http.StatusFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.renderLogin(w, r, "")
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(r.FormValue("token")) != a.token {
			a.renderLogin(w, r, "invalid token")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     a.cookieName,
			Value:    a.token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, a.basePath+"/dashboard", http.StatusFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *adminApp) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, a.basePath+"/login", http.StatusFound)
}

func (a *adminApp) renderLogin(w http.ResponseWriter, r *http.Request, notice string) {
	if a.token == "" {
		http.Redirect(w, r, a.basePath+"/dashboard", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	payload := struct {
		BasePath string
		Notice   string
	}{
		BasePath: a.basePath,
		Notice:   notice,
	}
	if err := a.loginTpl.Execute(w, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *adminApp) requireAuth(next http.Handler) http.Handler {
	if a.token == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.isAuthorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		if wantsHTML(r) {
			http.Redirect(w, r, a.basePath+"/login", http.StatusFound)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

func (a *adminApp) isAuthorized(r *http.Request) bool {
	if a.token == "" {
		return true
	}

	if header := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(header), "bearer ") {
		if strings.TrimSpace(header[7:]) == a.token {
			return true
		}
	}
	if token := strings.TrimSpace(r.Header.Get("X-Admin-Token")); token != "" && token == a.token {
		return true
	}
	if cookie, err := r.Cookie(a.cookieName); err == nil && cookie.Value == a.token {
		return true
	}
	return false
}

func (a *adminApp) handleDashboard(w http.ResponseWriter, r *http.Request) {
	a.syncTraffic(r.Context())

	summary, err := a.reality.RegistrySummary()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	clients, err := a.reality.ListClients()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	invites, err := a.reality.ListInvites()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	promos, err := a.reality.ListPromos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	state, err := a.reality.LoadState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	realityCfg := a.reality.Config()
	clientSort := normalizeClientSort(r.URL.Query().Get("clients_sort"))
	sortClients(clients, clientSort)

	view := dashboardView{
		BasePath:          a.basePath,
		GeneratedAt:       time.Now().UTC(),
		PanelVersion:      adminPanelVersion(),
		Summary:           summary,
		Clients:           clients,
		Invites:           invites,
		Promos:            promos,
		ClientSort:        clientSort,
		Metrics:           a.metricSnapshot(),
		ServerStats:       a.serverStatSnapshot(),
		HealthAddr:        a.metricsAddress(),
		RegistryPath:      a.realityRegistryPath(),
		ClientProfilePath: realityCfg.Xray.ClientProfilePath,
		ConfigPath:        realityCfg.Xray.ConfigPath,
		StatePath:         realityCfg.Xray.StatePath,
		PingURL:           a.basePath + "/diag/ping",
		DownloadURL:       a.basePath + "/diag/download?bytes=4194304",
		UploadURL:         a.basePath + "/diag/upload",
		SiteImageURL:      "/image.png",
		Ready:             true,
		Notice:            "",
	}

	if len(state.ShortIDs) == 0 {
		view.Ready = false
		view.Notice = "Reality state does not have any short IDs yet."
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.dashboardTpl.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *adminApp) handleAPI(w http.ResponseWriter, r *http.Request) {
	a.syncTraffic(r.Context())

	switch {
	case r.URL.Path == a.basePath+"/api/summary":
		a.writeJSON(w, r, func() (any, error) {
			return a.reality.RegistrySummary()
		})
	case r.URL.Path == a.basePath+"/api/diag/system":
		a.writeJSON(w, r, func() (any, error) {
			return map[string]any{
				"observed_at": time.Now().UTC(),
				"rows":        a.serverStatSnapshot(),
			}, nil
		})
	case r.URL.Path == a.basePath+"/api/clients":
		switch r.Method {
		case http.MethodGet:
			a.writeJSON(w, r, func() (any, error) {
				clients, err := a.reality.ListClients()
				if err != nil {
					return nil, err
				}
				sortClients(clients, normalizeClientSort(r.URL.Query().Get("sort")))
				return clients, nil
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case r.URL.Path == a.basePath+"/api/invites":
		switch r.Method {
		case http.MethodGet:
			a.writeJSON(w, r, func() (any, error) {
				return a.reality.ListInvites()
			})
		case http.MethodPost:
			a.handleCreateInvite(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case r.URL.Path == a.basePath+"/api/promos":
		switch r.Method {
		case http.MethodGet:
			a.writeJSON(w, r, func() (any, error) {
				return a.reality.ListPromos()
			})
		case http.MethodPost:
			a.handleCreatePromo(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case strings.HasPrefix(r.URL.Path, a.basePath+"/api/invites/"):
		a.handleInviteAction(w, r)
	case strings.HasPrefix(r.URL.Path, a.basePath+"/api/clients/"):
		a.handleClientAction(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (a *adminApp) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	req, err := decodeInviteCreateRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	invite, err := a.reality.CreateInvite(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if wantsHTML(r) {
		http.Redirect(w, r, a.basePath+"/dashboard", http.StatusSeeOther)
		return
	}

	a.writeJSONPayload(w, http.StatusCreated, map[string]any{
		"invite":         invite,
		"redeem_url":     a.basePath + "/redeem/" + invite.Code,
		"api_redeem_url": a.basePath + "/api/invites/" + invite.Code + "/redeem",
		"dashboard":      a.basePath + "/dashboard",
		"status":         "created",
	})
}

func (a *adminApp) handleCreatePromo(w http.ResponseWriter, r *http.Request) {
	req, err := decodePromoCreateRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	promo, err := a.reality.CreatePromo(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if wantsHTML(r) {
		http.Redirect(w, r, a.basePath+"/dashboard", http.StatusSeeOther)
		return
	}

	a.writeJSONPayload(w, http.StatusCreated, map[string]any{
		"promo":     promo,
		"dashboard": a.basePath + "/dashboard",
		"status":    "created",
	})
}

func (a *adminApp) handleInviteAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, a.basePath+"/api/invites/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	code := parts[0]
	action := parts[1]
	if action != "redeem" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var payload redeemInviteRequest
		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			if err := decodeJSON(r, &payload); err != nil && !errorsIsEmptyBody(err) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		} else if err := r.ParseForm(); err == nil {
			payload.DeviceID = strings.TrimSpace(r.FormValue("device_id"))
			payload.DeviceName = strings.TrimSpace(r.FormValue("device_name"))
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		redeemResult, refreshResult, err := a.reality.RedeemInvite(r.Context(), code, payload.DeviceID, payload.DeviceName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		clientProfiles := a.reality.BuildClientProfilesFor(refreshResult.State, redeemResult.Client)
		if len(clientProfiles) == 0 {
			http.Error(w, "server did not build client profiles", http.StatusInternalServerError)
			return
		}
		clientProfile := clientProfiles[0]
		yamlPayload, err := marshalClientProfileYAML(clientProfile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		yamlPayloadList, err := marshalClientProfileYAMLList(clientProfiles)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if wantsYAML(r) {
			w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, redeemResult.Client.ID))
			_, _ = w.Write(yamlPayload)
			return
		}

		a.writeJSONPayload(w, http.StatusCreated, map[string]any{
			"invite":               redeemResult.Invite,
			"client":               redeemResult.Client,
			"client_profile":       clientProfile,
			"client_profile_yaml":  string(yamlPayload),
			"client_profiles":      clientProfiles,
			"client_profiles_yaml": yamlPayloadList,
			"config_path":          refreshResult.ConfigPath,
			"client_profile_path":  refreshResult.ClientProfilePath,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *adminApp) handlePublicRedeem(w http.ResponseWriter, r *http.Request) {
	code := strings.Trim(strings.TrimPrefix(r.URL.Path, a.basePath+"/redeem/"), "/")
	if code == "" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	payload, err := decodeRedeemInviteRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	redeemResult, refreshResult, err := a.reality.RedeemInvite(r.Context(), code, payload.DeviceID, payload.DeviceName)
	if err == nil {
		clientProfiles := a.reality.BuildClientProfilesFor(refreshResult.State, redeemResult.Client)
		if len(clientProfiles) == 0 {
			http.Error(w, "server did not build client profiles", http.StatusInternalServerError)
			return
		}
		clientProfile := clientProfiles[0]
		yamlPayload, yamlErr := marshalClientProfileYAML(clientProfile)
		if yamlErr != nil {
			http.Error(w, yamlErr.Error(), http.StatusInternalServerError)
			return
		}
		yamlPayloadList, yamlListErr := marshalClientProfileYAMLList(clientProfiles)
		if yamlListErr != nil {
			http.Error(w, yamlListErr.Error(), http.StatusInternalServerError)
			return
		}

		if wantsYAML(r) {
			w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
			_, _ = w.Write(yamlPayload)
			return
		}

		a.writeJSONPayload(w, http.StatusCreated, map[string]any{
			"kind":                 "invite",
			"invite":               redeemResult.Invite,
			"client":               redeemResult.Client,
			"client_profile":       clientProfile,
			"client_profile_yaml":  string(yamlPayload),
			"client_profiles":      clientProfiles,
			"client_profiles_yaml": yamlPayloadList,
			"config_path":          refreshResult.ConfigPath,
			"client_profile_path":  refreshResult.ClientProfilePath,
		})
		return
	}

	promoResult, promoRefreshResult, promoErr := a.reality.RedeemPromo(r.Context(), code, payload.DeviceID, payload.DeviceName)
	if promoErr != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.writeJSONPayload(w, http.StatusCreated, map[string]any{
		"kind":                "promo",
		"promo":               promoResult.Promo,
		"client":              promoResult.Client,
		"bonus_bytes":         promoResult.Promo.BonusBytes,
		"traffic_used_bytes":  promoResult.Client.TrafficUsedBytes,
		"traffic_limit_bytes": promoResult.Client.TrafficLimitBytes,
		"config_path":         promoRefreshResult.ConfigPath,
		"client_profile_path": promoRefreshResult.ClientProfilePath,
	})
}

type disconnectDeviceRequest struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	ClientUUID string `json:"client_uuid"`
}

func (a *adminApp) handlePublicDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload disconnectDeviceRequest
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := decodeJSON(r, &payload); err != nil && !errorsIsEmptyBody(err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else if err := r.ParseForm(); err == nil {
		payload.DeviceID = strings.TrimSpace(r.FormValue("device_id"))
		payload.ClientUUID = strings.TrimSpace(r.FormValue("client_uuid"))
		payload.DeviceName = strings.TrimSpace(r.FormValue("device_name"))
	} else {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, refreshResult, err := a.reality.DisconnectDevice(r.Context(), payload.DeviceID, payload.ClientUUID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.writeJSONPayload(w, http.StatusOK, map[string]any{
		"status":              "disconnected",
		"client":              client,
		"config_path":         refreshResult.ConfigPath,
		"client_profile_path": refreshResult.ClientProfilePath,
	})
}

func (a *adminApp) handlePublicDiagnostics(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, a.basePath+"/diag/")
	switch path {
	case "ping":
		a.handleDiagnosticsPing(w, r)
	case "download":
		a.handleDiagnosticsDownload(w, r)
	case "upload":
		a.handleDiagnosticsUpload(w, r)
	default:
		http.NotFound(w, r)
	}
}

type redeemInviteRequest struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

func (a *adminApp) handleClientAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, a.basePath+"/api/clients/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	clientID := parts[0]
	action := parts[1]

	switch action {
	case "revoke":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		client, refreshResult, err := a.reality.RevokeClient(r.Context(), clientID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.writeJSONPayload(w, http.StatusOK, map[string]any{
			"client":              client,
			"config_path":         refreshResult.ConfigPath,
			"client_profile_path": refreshResult.ClientProfilePath,
			"registry_path":       refreshResult.RegistryPath,
		})
	case "profile.yaml":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		state, err := a.reality.LoadState()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		clients, err := a.reality.ListClients()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var target reality.ClientRecord
		found := false
		for _, client := range clients {
			if client.ID == clientID {
				target = client
				found = true
				break
			}
		}
		if !found {
			http.NotFound(w, r)
			return
		}

		clientProfile := a.reality.BuildClientProfileFor(state, target)
		yamlPayload, err := marshalClientProfileYAML(clientProfile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, target.ID))
		_, _ = w.Write(yamlPayload)
	default:
		http.NotFound(w, r)
	}
}

func (a *adminApp) writeJSON(w http.ResponseWriter, r *http.Request, fn func() (any, error)) {
	value, err := fn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.writeJSONPayload(w, http.StatusOK, value)
}

func (a *adminApp) writeJSONPayload(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func (a *adminApp) metricSnapshot() []metricRow {
	if a.metrics == nil || a.metrics.Registry() == nil {
		return nil
	}

	gathered, err := a.metrics.Registry().Gather()
	if err != nil {
		return nil
	}

	wanted := []string{
		"gateway_tcp_connections_active",
		"gateway_tcp_rejected_total",
		"gateway_udp_sessions_active",
		"gateway_auth_failures_total",
		"gateway_acl_denies_total",
		"gateway_tcp_bytes_in_total",
		"gateway_tcp_bytes_out_total",
		"gateway_udp_packets_in_total",
		"gateway_udp_packets_out_total",
		"gateway_shutdown_in_progress",
	}

	values := make([]metricRow, 0, len(wanted))
	for _, name := range wanted {
		if value, ok := lookupMetric(gathered, name); ok {
			values = append(values, metricRow{Name: name, Value: value})
		}
	}
	return values
}

func (a *adminApp) serverStatSnapshot() []metricRow {
	return collectServerRuntimeRows()
}

func (a *adminApp) handleDiagnosticsPing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostname, _ := os.Hostname()
	payload := map[string]any{
		"ok":          true,
		"server_time": time.Now().UTC(),
		"hostname":    hostname,
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	a.writeJSONPayload(w, http.StatusOK, payload)
}

func (a *adminApp) handleDiagnosticsDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	byteCount := clampByteCount(r.URL.Query().Get("bytes"), 4*1024*1024, 64*1024, 32*1024*1024)
	buffer := make([]byte, 32*1024)
	for i := range buffer {
		buffer[i] = 'N'
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.FormatInt(byteCount, 10))
	w.Header().Set("X-NoVPN-Bytes", strconv.FormatInt(byteCount, 10))

	remaining := byteCount
	for remaining > 0 {
		chunk := int64(len(buffer))
		if remaining < chunk {
			chunk = remaining
		}
		if _, err := w.Write(buffer[:chunk]); err != nil {
			return
		}
		remaining -= chunk
	}
}

func (a *adminApp) handleDiagnosticsUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	const maxBytes = 16 * 1024 * 1024
	startedAt := time.Now()
	limited := io.LimitReader(r.Body, maxBytes+1)
	written, err := io.Copy(io.Discard, limited)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if written > maxBytes {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	a.writeJSONPayload(w, http.StatusOK, map[string]any{
		"received_bytes": written,
		"duration_ms":    time.Since(startedAt).Milliseconds(),
	})
}

func (a *adminApp) metricsAddress() string {
	return a.cfg.ListenAddr
}

func (a *adminApp) realityRegistryPath() string {
	return a.reality.Config().Xray.RegistryPath
}

func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}

func wantsYAML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/x-yaml") || strings.Contains(accept, "text/yaml")
}

func decodeRedeemInviteRequest(r *http.Request) (redeemInviteRequest, error) {
	var payload redeemInviteRequest
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := decodeJSON(r, &payload); err != nil && !errorsIsEmptyBody(err) {
			return redeemInviteRequest{}, err
		}
		return payload, nil
	}
	if err := r.ParseForm(); err != nil {
		return redeemInviteRequest{}, err
	}
	payload.DeviceID = strings.TrimSpace(r.FormValue("device_id"))
	payload.DeviceName = strings.TrimSpace(r.FormValue("device_name"))
	return payload, nil
}

func decodeInviteCreateRequest(r *http.Request) (reality.InviteCreateRequest, error) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			Name           string  `json:"name"`
			Note           string  `json:"note"`
			MaxUses        int     `json:"max_uses"`
			TrafficLimitGB float64 `json:"traffic_limit_gb"`
			ExpiresMinutes int     `json:"expires_minutes"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			return reality.InviteCreateRequest{}, err
		}
		return reality.InviteCreateRequest{
			Name:              payload.Name,
			Note:              payload.Note,
			MaxUses:           payload.MaxUses,
			TrafficLimitBytes: trafficGBToBytes(payload.TrafficLimitGB),
			ExpiresAfter:      time.Duration(payload.ExpiresMinutes) * time.Minute,
		}, nil
	}

	if err := r.ParseForm(); err != nil {
		return reality.InviteCreateRequest{}, err
	}
	minutes, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("expires_minutes")))
	maxUses, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("max_uses")))
	trafficLimitGB, err := parseOptionalFloat(r.FormValue("traffic_limit_gb"))
	if err != nil {
		return reality.InviteCreateRequest{}, err
	}
	return reality.InviteCreateRequest{
		Name:              r.FormValue("name"),
		Note:              r.FormValue("note"),
		MaxUses:           maxUses,
		TrafficLimitBytes: trafficGBToBytes(trafficLimitGB),
		ExpiresAfter:      time.Duration(minutes) * time.Minute,
	}, nil
}

func decodePromoCreateRequest(r *http.Request) (reality.PromoCreateRequest, error) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			Name           string  `json:"name"`
			Note           string  `json:"note"`
			BonusGB        float64 `json:"bonus_gb"`
			ExpiresMinutes int     `json:"expires_minutes"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			return reality.PromoCreateRequest{}, err
		}
		return reality.PromoCreateRequest{
			Name:         payload.Name,
			Note:         payload.Note,
			BonusBytes:   trafficGBToBytes(payload.BonusGB),
			ExpiresAfter: time.Duration(payload.ExpiresMinutes) * time.Minute,
		}, nil
	}

	if err := r.ParseForm(); err != nil {
		return reality.PromoCreateRequest{}, err
	}
	minutes, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("expires_minutes")))
	bonusGB, err := parseOptionalFloat(r.FormValue("bonus_gb"))
	if err != nil {
		return reality.PromoCreateRequest{}, err
	}
	return reality.PromoCreateRequest{
		Name:         r.FormValue("name"),
		Note:         r.FormValue("note"),
		BonusBytes:   trafficGBToBytes(bonusGB),
		ExpiresAfter: time.Duration(minutes) * time.Minute,
	}, nil
}

func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	return nil
}

func errorsIsEmptyBody(err error) bool {
	return errors.Is(err, io.EOF)
}

func lookupMetric(mfs []*dto.MetricFamily, name string) (string, bool) {
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		if len(mf.Metric) == 0 {
			return "0", true
		}
		metric := mf.Metric[0]
		switch {
		case metric.Gauge != nil:
			return strconv.FormatFloat(metric.GetGauge().GetValue(), 'f', -1, 64), true
		case metric.Counter != nil:
			return strconv.FormatFloat(metric.GetCounter().GetValue(), 'f', -1, 64), true
		default:
			return "0", true
		}
	}
	return "", false
}

func marshalClientProfileYAML(profile reality.ClientProfile) ([]byte, error) {
	payload, err := yaml.Marshal(profile)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func marshalClientProfileYAMLList(profiles []reality.ClientProfile) ([]string, error) {
	result := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		payload, err := marshalClientProfileYAML(profile)
		if err != nil {
			return nil, err
		}
		result = append(result, string(payload))
	}
	return result, nil
}

func clampByteCount(raw string, fallback int64, min int64, max int64) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value == 0 {
		value = fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func parseOptionalFloat(raw string) (float64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", raw)
	}
	if value < 0 {
		return 0, fmt.Errorf("value must not be negative")
	}
	return value, nil
}

func trafficGBToBytes(value float64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(value * 1024 * 1024 * 1024)
}

func formatTrafficBytes(value int64) string {
	const unit = 1024
	if value <= 0 {
		return "0 B"
	}
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := int64(unit), 0
	for n := value / unit; n >= unit && exp < 5; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}

func formatTrafficLimit(value int64) string {
	if value <= 0 {
		return "unlimited"
	}
	return formatTrafficBytes(value)
}

func formatTrafficRemain(client reality.ClientRecord) string {
	if client.TrafficLimitBytes <= 0 {
		return "unlimited"
	}
	return formatTrafficBytes(client.TrafficRemainingBytes())
}

func normalizeClientSort(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "traffic_asc":
		return "traffic_asc"
	case "created_desc":
		return "created_desc"
	default:
		return "traffic_desc"
	}
}

func sortClients(clients []reality.ClientRecord, sortBy string) {
	switch sortBy {
	case "traffic_asc":
		sort.SliceStable(clients, func(i, j int) bool {
			if clients[i].TrafficUsedBytes == clients[j].TrafficUsedBytes {
				return clients[i].CreatedAt.Before(clients[j].CreatedAt)
			}
			return clients[i].TrafficUsedBytes < clients[j].TrafficUsedBytes
		})
	default:
		sort.SliceStable(clients, func(i, j int) bool {
			if clients[i].TrafficUsedBytes == clients[j].TrafficUsedBytes {
				return clients[i].CreatedAt.Before(clients[j].CreatedAt)
			}
			return clients[i].TrafficUsedBytes > clients[j].TrafficUsedBytes
		})
	}
}

func adminPanelVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	version := strings.TrimSpace(info.Main.Version)
	if version == "" {
		version = "devel"
	}

	revision := buildSetting(info, "vcs.revision")
	if revision != "" && len(revision) > 12 {
		revision = revision[:12]
	}

	modified := buildSetting(info, "vcs.modified") == "true"
	vcsTime := strings.TrimSpace(buildSetting(info, "vcs.time"))
	if parsed, err := time.Parse(time.RFC3339, vcsTime); err == nil {
		vcsTime = parsed.UTC().Format("2006-01-02 15:04 UTC")
	} else {
		vcsTime = ""
	}

	meta := make([]string, 0, 3)
	if revision != "" {
		meta = append(meta, revision)
	}
	if modified {
		meta = append(meta, "dirty")
	}
	if vcsTime != "" {
		meta = append(meta, vcsTime)
	}
	if len(meta) == 0 {
		return version
	}
	return fmt.Sprintf("%s (%s)", version, strings.Join(meta, ", "))
}

func buildSetting(info *debug.BuildInfo, key string) string {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}

func locateSiteImagePath(storagePath string) string {
	candidates := []string{
		filepath.Join(".", "image.png"),
		filepath.Join(storagePath, "image.png"),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func (a *adminApp) handleSiteImage(w http.ResponseWriter, r *http.Request) {
	if a.siteImagePath == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, a.siteImagePath)
}

func (a *adminApp) syncTraffic(ctx context.Context) {
	if _, err := a.reality.SyncTraffic(ctx); err != nil {
		a.logger.Debug("traffic sync skipped", "error", err)
	}
}

const adminDashboardTemplate = `
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>NoVPN Admin</title>
  <style>
    :root { color-scheme: dark; --bg: #070b12; --panel: #0f1722; --panel2: #141f2e; --text: #ecf2ff; --muted: #8a9ab0; --accent: #6dd6a6; --line: #213044; }
    body { margin: 0; font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, sans-serif; background: radial-gradient(circle at top, #10203a 0, #070b12 60%); color: var(--text); }
    .wrap { max-width: 1200px; margin: 0 auto; padding: 28px 20px 56px; }
    .hero { display: flex; gap: 16px; flex-wrap: wrap; align-items: center; justify-content: space-between; margin-bottom: 18px; }
    .hero-copy { flex: 1 1 420px; }
    .hero-media { flex: 0 1 320px; display: flex; justify-content: flex-end; }
    .hero-art { width: min(320px, 100%); border-radius: 22px; border: 1px solid var(--line); box-shadow: 0 18px 55px rgba(0,0,0,0.28); }
    .title { font-size: 30px; font-weight: 800; letter-spacing: -0.04em; margin: 0; }
    .sub { color: var(--muted); margin-top: 6px; }
    .chip { display: inline-block; padding: 6px 10px; border: 1px solid var(--line); border-radius: 999px; background: rgba(255,255,255,0.03); color: var(--muted); font-size: 12px; }
    .grid { display: grid; gap: 14px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); margin: 18px 0; }
    .card { background: linear-gradient(180deg, rgba(255,255,255,0.03), transparent), var(--panel); border: 1px solid var(--line); border-radius: 18px; padding: 18px; box-shadow: 0 20px 55px rgba(0,0,0,0.22); }
    .card h2, .card h3 { margin: 0 0 10px; font-size: 16px; }
    .kpi { font-size: 30px; font-weight: 800; letter-spacing: -0.04em; }
    .muted { color: var(--muted); font-size: 13px; }
    .table { width: 100%; border-collapse: collapse; font-size: 14px; }
    .table th, .table td { text-align: left; padding: 10px 8px; border-bottom: 1px solid var(--line); vertical-align: top; }
    .table th { color: var(--muted); font-size: 12px; text-transform: uppercase; letter-spacing: 0.08em; }
    .badge { display: inline-block; padding: 4px 8px; border-radius: 999px; background: rgba(109,214,166,0.12); color: var(--accent); border: 1px solid rgba(109,214,166,0.25); font-size: 12px; }
    .bad { color: #f0c27b; }
    .stack { display: grid; gap: 10px; }
    input, textarea, select, button { width: 100%; box-sizing: border-box; background: var(--panel2); color: var(--text); border: 1px solid var(--line); border-radius: 12px; padding: 10px 12px; font: inherit; }
    button { background: linear-gradient(180deg, rgba(109,214,166,0.18), rgba(109,214,166,0.08)); cursor: pointer; font-weight: 700; }
    form.inline { display: flex; gap: 10px; align-items: end; flex-wrap: wrap; }
    form.inline > * { flex: 1 1 180px; }
    .small { font-size: 12px; }
    .topline { display:flex; gap:12px; flex-wrap:wrap; align-items:center; }
    .actions { display:flex; gap:12px; align-items:center; justify-content:space-between; flex-wrap:wrap; margin-bottom: 10px; }
    a { color: var(--accent); text-decoration: none; }
  </style>
</head>
<body>
<div class="wrap">
  <div class="hero">
    <div class="hero-copy">
      <h1 class="title">NoVPN Admin</h1>
      <div class="sub">Registry, reusable invite codes, promo traffic bonuses, device-bound client records, and runtime monitoring.</div>
      <div class="topline" style="margin-top:12px;">
        <span class="chip">Panel: {{.PanelVersion}}</span>
        <span class="chip">Traffic sync: approximate</span>
        <span class="chip">Client sort: {{.ClientSort}}</span>
      </div>
    </div>
    <div class="hero-media">
      <img class="hero-art" src="{{.SiteImageURL}}" alt="NoVPN visual">
    </div>
    <div class="topline">
      <span class="chip">Registry: {{.RegistryPath}}</span>
      <span class="chip">Config: {{.ConfigPath}}</span>
      <span class="chip">Profiles: {{.ClientProfilePath}}</span>
      <span class="chip">State: {{.StatePath}}</span>
    </div>
  </div>

  {{if .Notice}}<div class="card bad">{{.Notice}}</div>{{end}}

  <div class="grid">
    <div class="card"><div class="muted">Active clients</div><div class="kpi">{{.Summary.ActiveClients}}</div></div>
    <div class="card"><div class="muted">Pending invites</div><div class="kpi">{{.Summary.PendingInvites}}</div></div>
    <div class="card"><div class="muted">Approx traffic</div><div class="kpi">{{formatBytes .Summary.TotalTrafficBytes}}</div></div>
    <div class="card"><div class="muted">Traffic-limited clients</div><div class="kpi">{{.Summary.TrafficBlockedClients}}</div></div>
    <div class="card"><div class="muted">Gateway ready</div><div class="kpi">{{if .Ready}}yes{{else}}check{{end}}</div></div>
    <div class="card"><div class="muted">Xray profile</div><div class="kpi">{{.Summary.Server.PublicHost}}:{{.Summary.Server.PublicPort}}</div></div>
  </div>

  <div class="grid">
    <div class="card">
      <h2>Create invite</h2>
      <form method="post" action="{{.BasePath}}/api/invites" class="stack">
        <input name="name" placeholder="Invite name, e.g. Alice phone">
        <textarea name="note" rows="3" placeholder="Note"></textarea>
        <input name="max_uses" type="number" min="1" value="1" placeholder="Allowed activations, 1 = single-device">
        <input name="traffic_limit_gb" type="number" min="0" step="0.1" value="0" placeholder="Traffic limit in GiB, 0 = unlimited">
        <input name="expires_minutes" type="number" min="0" placeholder="Expires in minutes, 0 = no expiry">
        <button type="submit">Create invite</button>
      </form>
    </div>
    <div class="card">
      <h2>Create promo code</h2>
      <form method="post" action="{{.BasePath}}/api/promos" class="stack">
        <input name="name" placeholder="Promo name, e.g. Spring bonus">
        <textarea name="note" rows="3" placeholder="Note"></textarea>
        <input name="bonus_gb" type="number" min="0" step="0.1" value="1" placeholder="Bonus traffic in GiB">
        <input name="expires_minutes" type="number" min="0" placeholder="Expires in minutes, 0 = no expiry">
        <button type="submit">Create promo</button>
      </form>
    </div>
    <div class="card">
      <h2>Monitoring</h2>
      <div class="stack">
        {{range .Metrics}}
        <div class="topline"><span class="chip">{{.Name}}</span><strong>{{.Value}}</strong></div>
        {{end}}
      </div>
    </div>
    <div class="card">
      <h2>Server runtime</h2>
      <div class="stack">
        {{range .ServerStats}}
        <div class="topline"><span class="chip">{{.Name}}</span><strong>{{.Value}}</strong></div>
        {{end}}
        <div class="muted small">
          Diagnostics:
          <a href="{{.PingURL}}" target="_blank" rel="noreferrer">ping</a>
          · <a href="{{.DownloadURL}}" target="_blank" rel="noreferrer">download</a>
          · <span>{{.UploadURL}} (POST)</span>
        </div>
      </div>
    </div>
  </div>

  <div class="card">
    <div class="actions">
      <h2>Clients</h2>
      <div class="topline">
        <a href="{{.BasePath}}/dashboard?clients_sort=traffic_desc">Sort by traffic desc</a>
        <a href="{{.BasePath}}/dashboard?clients_sort=traffic_asc">Sort by traffic asc</a>
      </div>
    </div>
    <table class="table">
      <thead><tr><th>Name</th><th>Device</th><th>UUID</th><th>Key</th><th>Traffic</th><th>Remaining</th><th>Status</th><th>Profile</th><th>Action</th></tr></thead>
      <tbody>
      {{range .Clients}}
        <tr>
          <td>{{.Name}}</td>
          <td>{{.DeviceName}}<div class="muted small">{{.DeviceID}}</div></td>
          <td class="small">{{.UUID}}</td>
          <td class="small">{{if .InviteCode}}{{.InviteCode}}{{else}}bootstrap{{end}}</td>
          <td>{{formatBytes .TrafficUsedBytes}}<div class="muted small">limit {{formatTrafficLimit .TrafficLimitBytes}}</div></td>
          <td>{{formatTrafficRemain .}}</td>
          <td>{{if .RevokedAt}}<span class="chip">revoked</span>{{else if .TrafficBlockedAt}}<span class="chip">limit reached</span>{{else if .Active}}<span class="badge">active</span>{{else}}<span class="chip">inactive</span>{{end}}</td>
          <td><a href="{{$.BasePath}}/api/clients/{{.ID}}/profile.yaml">download</a></td>
          <td>
            {{if .Bound}}
            <form method="post" action="{{$.BasePath}}/api/clients/{{.ID}}/revoke">
              <button type="submit">Revoke</button>
            </form>
            {{else}}<span class="muted">revoked</span>{{end}}
          </td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>

  <div class="card">
    <h2>Invites</h2>
    <table class="table">
      <thead><tr><th>Code</th><th>Name</th><th>Status</th><th>Active uses</th><th>Traffic limit</th><th>Expiry</th><th>Last redeemed</th><th>Redeem URL</th></tr></thead>
      <tbody>
      {{range .Invites}}
        <tr>
          <td class="small">{{.Code}}</td>
          <td>{{.Name}}<div class="muted small">{{.Note}}</div></td>
          <td>
            {{if .Active}}<span class="badge">active</span>{{else}}<span class="chip">inactive</span>{{end}}
            <div class="muted small">created {{.CreatedAt.Format "2006-01-02 15:04:05"}}</div>
          </td>
          <td class="small">{{.ActiveUses}} / {{.MaxUses}}<div class="muted">total {{.RedeemedUses}}</div></td>
          <td class="small">{{formatTrafficLimit .TrafficLimitBytes}}</td>
          <td class="small">{{if .ExpiresAt}}{{.ExpiresAt.Format "2006-01-02 15:04:05"}}{{else}}never{{end}}</td>
          <td class="small">{{if .RedeemedAt}}{{.RedeemedAt.Format "2006-01-02 15:04:05"}}<div class="muted">device {{.RedeemedDeviceName}}</div>{{else}}never{{end}}</td>
          <td class="small">{{$.BasePath}}/redeem/{{.Code}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>

  <div class="card">
    <h2>Promo Codes</h2>
    <table class="table">
      <thead><tr><th>Code</th><th>Name</th><th>Status</th><th>Bonus</th><th>Uses</th><th>Expiry</th></tr></thead>
      <tbody>
      {{range .Promos}}
        <tr>
          <td class="small">{{.Code}}</td>
          <td>{{.Name}}<div class="muted small">{{.Note}}</div></td>
          <td>{{if .Active}}<span class="badge">active</span>{{else}}<span class="chip">inactive</span>{{end}}</td>
          <td class="small">{{formatBytes .BonusBytes}}</td>
          <td class="small">{{.RedeemedUses}}</td>
          <td class="small">{{if .ExpiresAt}}{{.ExpiresAt.Format "2006-01-02 15:04:05"}}{{else}}never{{end}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>
</div>
</body>
</html>
`

const adminLoginTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>NoVPN Admin Login</title>
  <style>
    body { margin:0; min-height:100vh; display:grid; place-items:center; font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, sans-serif; background:#07111f; color:#eef5ff; }
    .card { width:min(420px, calc(100vw - 32px)); background:#0f1722; border:1px solid #213044; border-radius:18px; padding:24px; }
    input, button { width:100%; box-sizing:border-box; margin-top:12px; padding:12px 14px; border-radius:12px; border:1px solid #213044; background:#141f2e; color:#eef5ff; font:inherit; }
    button { background:linear-gradient(180deg, rgba(109,214,166,0.18), rgba(109,214,166,0.08)); font-weight:700; }
    .notice { color:#f0c27b; min-height:1.2em; }
  </style>
</head>
<body>
  <form class="card" method="post" action="{{.BasePath}}/login">
    <h1>NoVPN Admin</h1>
    <p>Enter the admin token to continue.</p>
    <div class="notice">{{.Notice}}</div>
    <input type="password" name="token" placeholder="Admin token" autofocus>
    <button type="submit">Login</button>
  </form>
</body>
</html>`
