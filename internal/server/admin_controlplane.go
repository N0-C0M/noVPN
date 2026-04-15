package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"novpn/internal/controlplane"
	"novpn/internal/core/reality"
)

type planPageView struct {
	BasePath string
	Plans    []controlplane.SubscriptionPlan
	Servers  []controlplane.ServerNode
}

type serverPageView struct {
	BasePath string
	Servers  []controlplane.ServerNode
}

type runtimeMutationSnapshot struct {
	ConfigPath        string
	StatePath         string
	RegistryPath      string
	ClientProfilePath string
	State             reality.State
}

func (a *adminApp) runtimeManaged() bool {
	return strings.ToLower(strings.TrimSpace(a.cfg.RuntimeMode)) != "remote"
}

func (a *adminApp) publicBaseURL() string {
	base := strings.TrimRight(strings.TrimSpace(a.cfg.PublicBaseURL), "/")
	if base == "" {
		if strings.HasPrefix(a.cfg.ListenAddr, "127.0.0.1:") || strings.HasPrefix(a.cfg.ListenAddr, "[::1]:") {
			return ""
		}
		base = "http://" + strings.TrimSpace(a.cfg.ListenAddr)
	}
	if strings.HasSuffix(base, a.basePath) {
		return base
	}
	return base + a.basePath
}

func (a *adminApp) requireControlPlaneAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.controlPlaneAuthorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

func (a *adminApp) controlPlaneAuthorized(r *http.Request) bool {
	token := strings.TrimSpace(a.cfg.ControlPlaneToken)
	if token != "" && strings.TrimSpace(r.Header.Get("X-Control-Plane-Token")) == token {
		return true
	}
	return a.isAuthorized(r)
}

func (a *adminApp) handlePlansPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	plans, err := a.catalogStore.ListPlans()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	servers, err := a.catalogStore.ListServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.plansTpl.Execute(w, planPageView{
		BasePath: a.basePath,
		Plans:    plans,
		Servers:  servers,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *adminApp) handleServersPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	servers, err := a.catalogStore.ListServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.serversTpl.Execute(w, serverPageView{
		BasePath: a.basePath,
		Servers:  servers,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *adminApp) handlePlansAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.writeJSON(w, r, func() (any, error) {
			return a.catalogStore.ListPlans()
		})
	case http.MethodPost:
		req, err := decodePlanCreateRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		plan, err := a.catalogStore.CreatePlan(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if wantsHTML(r) {
			http.Redirect(w, r, a.basePath+"/plans", http.StatusSeeOther)
			return
		}
		a.writeJSONPayload(w, http.StatusCreated, plan)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *adminApp) handleServersAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.writeJSON(w, r, func() (any, error) {
			return a.catalogStore.ListServers()
		})
	case http.MethodPost:
		req, err := decodeServerCreateRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		server, err := a.catalogStore.CreateServer(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if wantsHTML(r) {
			http.Redirect(w, r, a.basePath+"/servers", http.StatusSeeOther)
			return
		}
		a.writeJSONPayload(w, http.StatusCreated, server)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *adminApp) handlePublicPlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	plans, err := a.catalogStore.ActivePlans()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.writeJSONPayload(w, http.StatusOK, map[string]any{
		"observed_at": time.Now().UTC(),
		"plans":       plans,
	})
}

func (a *adminApp) handleControlPlaneAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, a.basePath+"/control-plane/"), "/")
	switch path {
	case "registry":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		a.writeJSON(w, r, func() (any, error) {
			return a.reality.LoadRegistry()
		})
	case "traffic":
		if r.Method != http.MethodPost && r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		request, err := decodeControlPlaneTrafficRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		usages := make(map[string]reality.TrafficUsage, len(request.Usages))
		observedAt := time.Now().UTC()
		for email, totalBytes := range request.Usages {
			trimmedEmail := strings.TrimSpace(email)
			if trimmedEmail == "" {
				continue
			}
			usages[trimmedEmail] = reality.TrafficUsage{
				TotalBytes: totalBytes,
				ObservedAt: observedAt,
			}
		}
		result, err := a.reality.ApplyTrafficUsages(usages)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.writeJSONPayload(w, http.StatusOK, result)
	case "payments/activate":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		request, err := decodePaymentActivationRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		inviteRequest, err := a.buildInviteRequestFromPlan(request.PlanID, request.Name, request.Note, request.MaxUses)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		invite, err := a.reality.CreateInvite(inviteRequest)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.writeJSONPayload(w, http.StatusCreated, map[string]any{
			"status":         "issued",
			"invite":         invite,
			"redeem_url":     a.basePath + "/redeem/" + invite.Code,
			"api_redeem_url": a.basePath + "/api/invites/" + invite.Code + "/redeem",
			"public_api":     a.publicBaseURL(),
		})
	default:
		http.NotFound(w, r)
	}
}

func (a *adminApp) expandInvitePlan(req reality.InviteCreateRequest) (reality.InviteCreateRequest, error) {
	if strings.TrimSpace(req.PlanID) == "" {
		return req, nil
	}
	plan, err := a.catalogStore.FindPlan(req.PlanID)
	if err != nil {
		return reality.InviteCreateRequest{}, err
	}
	req.PlanID = plan.ID
	req.PlanName = plan.Name
	req.AllowedServerIDs = append([]string(nil), plan.ServerIDs...)
	req.AccessDurationDays = plan.DurationDays
	if strings.TrimSpace(req.Name) == "" {
		req.Name = plan.Name
	}
	if req.TrafficLimitBytes <= 0 && plan.TrafficLimitBytes > 0 {
		req.TrafficLimitBytes = plan.TrafficLimitBytes
	}
	return req, nil
}

func (a *adminApp) buildInviteRequestFromPlan(planID string, name string, note string, maxUses int) (reality.InviteCreateRequest, error) {
	req := reality.InviteCreateRequest{
		Name:    strings.TrimSpace(name),
		Note:    strings.TrimSpace(note),
		PlanID:  strings.TrimSpace(planID),
		MaxUses: maxUses,
	}
	return a.expandInvitePlan(req)
}

func (a *adminApp) buildClientProfiles(state reality.State, client reality.ClientRecord) []reality.ClientProfile {
	servers, err := a.catalogStore.FindServers(client.AllowedServerIDs)
	if err == nil && len(servers) > 0 {
		profiles := reality.BuildClientProfilesForCatalog(state, client, servers)
		if len(profiles) > 0 {
			return a.applyAPIBase(profiles)
		}
	}
	return a.applyAPIBase(a.reality.BuildClientProfilesFor(state, client))
}

func (a *adminApp) applyAPIBase(profiles []reality.ClientProfile) []reality.ClientProfile {
	apiBase := a.publicBaseURL()
	if apiBase == "" {
		return profiles
	}
	for index := range profiles {
		profiles[index].APIBase = apiBase
	}
	return profiles
}

func (a *adminApp) currentMutationSnapshot() (runtimeMutationSnapshot, error) {
	state, err := a.reality.LoadState()
	if err != nil {
		return runtimeMutationSnapshot{}, err
	}
	cfg := a.reality.Config()
	return runtimeMutationSnapshot{
		ConfigPath:        cfg.Xray.ConfigPath,
		StatePath:         cfg.Xray.StatePath,
		RegistryPath:      cfg.Xray.RegistryPath,
		ClientProfilePath: cfg.Xray.ClientProfilePath,
		State:             state,
	}, nil
}

func mutationSnapshotFromResult(result reality.Result) runtimeMutationSnapshot {
	return runtimeMutationSnapshot{
		ConfigPath:        result.ConfigPath,
		StatePath:         result.StatePath,
		RegistryPath:      result.RegistryPath,
		ClientProfilePath: result.ClientProfilePath,
		State:             result.State,
	}
}

func (a *adminApp) redeemInviteWithSnapshot(ctx context.Context, code string, deviceID string, deviceName string) (reality.InviteRedeemResult, runtimeMutationSnapshot, error) {
	if a.runtimeManaged() {
		result, refreshResult, err := a.reality.RedeemInvite(ctx, code, deviceID, deviceName)
		if err != nil {
			return reality.InviteRedeemResult{}, runtimeMutationSnapshot{}, err
		}
		return result, mutationSnapshotFromResult(refreshResult), nil
	}
	result, err := a.reality.RedeemInviteNoRefresh(code, deviceID, deviceName)
	if err != nil {
		return reality.InviteRedeemResult{}, runtimeMutationSnapshot{}, err
	}
	snapshot, err := a.currentMutationSnapshot()
	if err != nil {
		return reality.InviteRedeemResult{}, runtimeMutationSnapshot{}, err
	}
	return result, snapshot, nil
}

func (a *adminApp) redeemPromoWithSnapshot(ctx context.Context, code string, deviceID string, deviceName string) (reality.PromoRedeemResult, runtimeMutationSnapshot, error) {
	if a.runtimeManaged() {
		result, refreshResult, err := a.reality.RedeemPromo(ctx, code, deviceID, deviceName)
		if err != nil {
			return reality.PromoRedeemResult{}, runtimeMutationSnapshot{}, err
		}
		return result, mutationSnapshotFromResult(refreshResult), nil
	}
	result, err := a.reality.RedeemPromoNoRefresh(code, deviceID, deviceName)
	if err != nil {
		return reality.PromoRedeemResult{}, runtimeMutationSnapshot{}, err
	}
	snapshot, err := a.currentMutationSnapshot()
	if err != nil {
		return reality.PromoRedeemResult{}, runtimeMutationSnapshot{}, err
	}
	return result, snapshot, nil
}

func (a *adminApp) revokeClientWithSnapshot(ctx context.Context, clientID string) (reality.ClientRecord, runtimeMutationSnapshot, error) {
	if a.runtimeManaged() {
		client, refreshResult, err := a.reality.RevokeClient(ctx, clientID)
		if err != nil {
			return reality.ClientRecord{}, runtimeMutationSnapshot{}, err
		}
		return client, mutationSnapshotFromResult(refreshResult), nil
	}
	client, err := a.reality.RevokeClientNoRefresh(clientID)
	if err != nil {
		return reality.ClientRecord{}, runtimeMutationSnapshot{}, err
	}
	snapshot, err := a.currentMutationSnapshot()
	if err != nil {
		return reality.ClientRecord{}, runtimeMutationSnapshot{}, err
	}
	return client, snapshot, nil
}

func (a *adminApp) disconnectClientWithSnapshot(ctx context.Context, deviceID string, clientUUID string) (reality.ClientRecord, runtimeMutationSnapshot, error) {
	if a.runtimeManaged() {
		client, refreshResult, err := a.reality.DisconnectDevice(ctx, deviceID, clientUUID)
		if err != nil {
			return reality.ClientRecord{}, runtimeMutationSnapshot{}, err
		}
		return client, mutationSnapshotFromResult(refreshResult), nil
	}
	client, err := a.reality.DisconnectDeviceNoRefresh(deviceID, clientUUID)
	if err != nil {
		return reality.ClientRecord{}, runtimeMutationSnapshot{}, err
	}
	snapshot, err := a.currentMutationSnapshot()
	if err != nil {
		return reality.ClientRecord{}, runtimeMutationSnapshot{}, err
	}
	return client, snapshot, nil
}

type controlPlaneTrafficRequest struct {
	Usages map[string]int64 `json:"usages"`
}

type paymentActivationRequest struct {
	PlanID  string `json:"plan_id"`
	Name    string `json:"name"`
	Note    string `json:"note"`
	MaxUses int    `json:"max_uses"`
}

func decodePlanCreateRequest(r *http.Request) (controlplane.PlanCreateRequest, error) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			ID             string   `json:"id"`
			Name           string   `json:"name"`
			Description    string   `json:"description"`
			DurationDays   int      `json:"duration_days"`
			TrafficLimitGB float64  `json:"traffic_limit_gb"`
			PriceMinor     int64    `json:"price_minor"`
			Currency       string   `json:"currency"`
			ServerIDs      []string `json:"server_ids"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			return controlplane.PlanCreateRequest{}, err
		}
		return controlplane.PlanCreateRequest{
			ID:                payload.ID,
			Name:              payload.Name,
			Description:       payload.Description,
			DurationDays:      payload.DurationDays,
			TrafficLimitBytes: trafficGBToBytes(payload.TrafficLimitGB),
			PriceMinor:        payload.PriceMinor,
			Currency:          payload.Currency,
			ServerIDs:         payload.ServerIDs,
		}, nil
	}

	if err := r.ParseForm(); err != nil {
		return controlplane.PlanCreateRequest{}, err
	}
	durationDays, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("duration_days")))
	priceMinor, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("price_minor")), 10, 64)
	trafficLimitGB, err := parseOptionalFloat(r.FormValue("traffic_limit_gb"))
	if err != nil {
		return controlplane.PlanCreateRequest{}, err
	}
	return controlplane.PlanCreateRequest{
		ID:                r.FormValue("id"),
		Name:              r.FormValue("name"),
		Description:       r.FormValue("description"),
		DurationDays:      durationDays,
		TrafficLimitBytes: trafficGBToBytes(trafficLimitGB),
		PriceMinor:        priceMinor,
		Currency:          r.FormValue("currency"),
		ServerIDs:         splitMultilineList(r.FormValue("server_ids")),
	}, nil
}

func decodeServerCreateRequest(r *http.Request) (controlplane.ServerCreateRequest, error) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			ID            string   `json:"id"`
			Name          string   `json:"name"`
			Address       string   `json:"address"`
			Port          int      `json:"port"`
			Flow          string   `json:"flow"`
			ServerName    string   `json:"server_name"`
			Fingerprint   string   `json:"fingerprint"`
			PublicKey     string   `json:"public_key"`
			ShortID       string   `json:"short_id"`
			ShortIDs      []string `json:"short_ids"`
			SpiderX       string   `json:"spider_x"`
			LocationLabel string   `json:"location_label"`
			VPNOnly       bool     `json:"vpn_only"`
			Primary       bool     `json:"primary"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			return controlplane.ServerCreateRequest{}, err
		}
		return controlplane.ServerCreateRequest{
			ID:            payload.ID,
			Name:          payload.Name,
			Address:       payload.Address,
			Port:          payload.Port,
			Flow:          payload.Flow,
			ServerName:    payload.ServerName,
			Fingerprint:   payload.Fingerprint,
			PublicKey:     payload.PublicKey,
			ShortID:       payload.ShortID,
			ShortIDs:      payload.ShortIDs,
			SpiderX:       payload.SpiderX,
			LocationLabel: payload.LocationLabel,
			VPNOnly:       payload.VPNOnly,
			Primary:       payload.Primary,
		}, nil
	}

	if err := r.ParseForm(); err != nil {
		return controlplane.ServerCreateRequest{}, err
	}
	port, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	return controlplane.ServerCreateRequest{
		ID:            r.FormValue("id"),
		Name:          r.FormValue("name"),
		Address:       r.FormValue("address"),
		Port:          port,
		Flow:          r.FormValue("flow"),
		ServerName:    r.FormValue("server_name"),
		Fingerprint:   r.FormValue("fingerprint"),
		PublicKey:     r.FormValue("public_key"),
		ShortID:       r.FormValue("short_id"),
		ShortIDs:      splitMultilineList(r.FormValue("short_ids")),
		SpiderX:       r.FormValue("spider_x"),
		LocationLabel: r.FormValue("location_label"),
		VPNOnly:       strings.EqualFold(strings.TrimSpace(r.FormValue("vpn_only")), "on"),
		Primary:       strings.EqualFold(strings.TrimSpace(r.FormValue("primary")), "on"),
	}, nil
}

func decodeControlPlaneTrafficRequest(r *http.Request) (controlPlaneTrafficRequest, error) {
	var payload controlPlaneTrafficRequest
	if err := decodeJSON(r, &payload); err != nil {
		return controlPlaneTrafficRequest{}, err
	}
	return payload, nil
}

func decodePaymentActivationRequest(r *http.Request) (paymentActivationRequest, error) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload paymentActivationRequest
		if err := decodeJSON(r, &payload); err != nil {
			return paymentActivationRequest{}, err
		}
		return payload, nil
	}
	if err := r.ParseForm(); err != nil {
		return paymentActivationRequest{}, err
	}
	maxUses, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("max_uses")))
	return paymentActivationRequest{
		PlanID:  r.FormValue("plan_id"),
		Name:    r.FormValue("name"),
		Note:    r.FormValue("note"),
		MaxUses: maxUses,
	}, nil
}

const adminPlansTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>NoVPN Plans</title>
  <style>
    body { margin:0; font-family: ui-sans-serif, system-ui, sans-serif; background:#0a1018; color:#eef5ff; }
    .wrap { max-width: 1120px; margin: 0 auto; padding: 28px 20px 56px; }
    .card { background:#101925; border:1px solid #223246; border-radius:22px; padding:18px; margin-top:16px; }
    .row { display:grid; gap:12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
    input, textarea, button { width:100%; box-sizing:border-box; background:#141f2e; color:#eef5ff; border:1px solid #223246; border-radius:14px; padding:10px 12px; font:inherit; }
    button { background:#17324a; cursor:pointer; font-weight:700; }
    .table { width:100%; border-collapse:collapse; margin-top:12px; }
    .table th, .table td { text-align:left; padding:10px 8px; border-bottom:1px solid #223246; vertical-align:top; }
    a { color:#7acaa7; text-decoration:none; }
    .muted { color:#8ea3bb; }
  </style>
</head>
<body>
<div class="wrap">
  <div><a href="{{.BasePath}}/dashboard">← Back to dashboard</a></div>
  <div class="card">
    <h1>Subscription plans</h1>
    <p class="muted">Create product types and bind specific VPN nodes to each plan.</p>
    <form method="post" action="{{.BasePath}}/api/plans">
      <div class="row">
        <input name="id" placeholder="Plan ID (optional), e.g. premium-30">
        <input name="name" placeholder="Plan name">
        <input name="duration_days" type="number" min="0" placeholder="Duration in days, 0 = unlimited">
        <input name="traffic_limit_gb" type="number" min="0" step="0.1" placeholder="Traffic limit in GiB">
        <input name="price_minor" type="number" min="0" placeholder="Price in minor units">
        <input name="currency" placeholder="Currency, e.g. USD">
      </div>
      <textarea name="description" rows="3" style="margin-top:12px;" placeholder="Description"></textarea>
      <textarea name="server_ids" rows="4" style="margin-top:12px;" placeholder="Server IDs, one per line">{{range .Servers}}{{.ID}}
{{end}}</textarea>
      <button type="submit" style="margin-top:12px;">Create plan</button>
    </form>
  </div>
  <div class="card">
    <h2>Existing plans</h2>
    <table class="table">
      <thead><tr><th>ID</th><th>Name</th><th>Duration</th><th>Traffic</th><th>Servers</th></tr></thead>
      <tbody>
      {{range .Plans}}
        <tr>
          <td>{{.ID}}</td>
          <td>{{.Name}}<div class="muted">{{.Description}}</div></td>
          <td>{{if .DurationDays}}{{.DurationDays}} days{{else}}unlimited{{end}}</td>
          <td>{{formatTrafficLimit .TrafficLimitBytes}}</td>
          <td>{{range .ServerIDs}}<div>{{.}}</div>{{end}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>
</div>
</body>
</html>`

const adminServersTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>NoVPN Servers</title>
  <style>
    body { margin:0; font-family: ui-sans-serif, system-ui, sans-serif; background:#0a1018; color:#eef5ff; }
    .wrap { max-width: 1120px; margin: 0 auto; padding: 28px 20px 56px; }
    .card { background:#101925; border:1px solid #223246; border-radius:22px; padding:18px; margin-top:16px; }
    .row { display:grid; gap:12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
    input, textarea, button { width:100%; box-sizing:border-box; background:#141f2e; color:#eef5ff; border:1px solid #223246; border-radius:14px; padding:10px 12px; font:inherit; }
    button { background:#17324a; cursor:pointer; font-weight:700; }
    .table { width:100%; border-collapse:collapse; margin-top:12px; }
    .table th, .table td { text-align:left; padding:10px 8px; border-bottom:1px solid #223246; vertical-align:top; }
    a { color:#7acaa7; text-decoration:none; }
    .muted { color:#8ea3bb; }
  </style>
</head>
<body>
<div class="wrap">
  <div><a href="{{.BasePath}}/dashboard">← Back to dashboard</a></div>
  <div class="card">
    <h1>VPN nodes</h1>
    <p class="muted">Register additional VPN servers for subscription plans and client profile generation.</p>
    <form method="post" action="{{.BasePath}}/api/servers">
      <div class="row">
        <input name="id" placeholder="Server ID (optional)">
        <input name="name" placeholder="Display name">
        <input name="address" placeholder="Public IP or host">
        <input name="port" type="number" min="1" placeholder="Port">
        <input name="flow" placeholder="Flow, e.g. xtls-rprx-vision">
        <input name="server_name" placeholder="SNI / server name">
        <input name="fingerprint" placeholder="Fingerprint, e.g. chrome">
        <input name="public_key" placeholder="REALITY public key (optional for primary)">
        <input name="short_id" placeholder="Primary short ID">
        <input name="location_label" placeholder="Location label">
        <input name="spider_x" placeholder="SpiderX path">
      </div>
      <textarea name="short_ids" rows="3" style="margin-top:12px;" placeholder="Short IDs, one per line"></textarea>
      <div class="row" style="margin-top:12px;">
        <label><input type="checkbox" name="vpn_only"> VPN only</label>
        <label><input type="checkbox" name="primary"> Primary node</label>
      </div>
      <button type="submit" style="margin-top:12px;">Create server</button>
    </form>
  </div>
  <div class="card">
    <h2>Registered servers</h2>
    <table class="table">
      <thead><tr><th>ID</th><th>Name</th><th>Endpoint</th><th>Server name</th><th>Notes</th></tr></thead>
      <tbody>
      {{range .Servers}}
        <tr>
          <td>{{.ID}}</td>
          <td>{{.Name}}</td>
          <td>{{.Address}}:{{.Port}}</td>
          <td>{{.ServerName}}</td>
          <td><div>{{.LocationLabel}}</div><div class="muted">{{if .Primary}}primary{{else if .VPNOnly}}vpn-only{{else}}shared{{end}}</div></td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>
</div>
</body>
</html>`
