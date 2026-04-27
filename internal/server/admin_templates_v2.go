package server

const adminDashboardTemplateV2 = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>NoVPN Admin</title>
  <style>
    :root { color-scheme: dark; --bg: #07101b; --panel: #0f1824; --panel2: #142131; --text: #edf3ff; --muted: #8ea3bb; --line: #223246; --accent: #73d5ac; --warn: #f0c27b; }
    body { margin:0; font-family: ui-sans-serif, system-ui, sans-serif; background: radial-gradient(circle at top, #102540 0, #07101b 58%); color: var(--text); }
    .wrap { max-width: 1240px; margin: 0 auto; padding: 28px 20px 60px; }
    .hero { display:flex; gap:16px; align-items:flex-start; justify-content:space-between; flex-wrap:wrap; margin-bottom:18px; }
    .title { font-size: 30px; font-weight: 800; letter-spacing: -0.04em; margin: 0; }
    .sub { color: var(--muted); margin-top: 6px; max-width: 760px; }
    .hero-art { width:min(300px, 100%); border-radius:22px; border:1px solid var(--line); box-shadow: 0 18px 55px rgba(0,0,0,0.28); }
    .topline, .nav { display:flex; gap:10px; flex-wrap:wrap; align-items:center; }
    .nav { margin: 18px 0 22px; }
    .chip, .tab { display:inline-flex; align-items:center; gap:8px; padding:8px 12px; border-radius:999px; border:1px solid var(--line); background: rgba(255,255,255,0.03); color: var(--muted); font-size: 12px; text-decoration:none; }
    .tab.active { color: var(--text); background: rgba(115,213,172,0.12); border-color: rgba(115,213,172,0.3); }
    .grid { display:grid; gap:14px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); margin: 18px 0; }
    .card { background: linear-gradient(180deg, rgba(255,255,255,0.03), transparent), var(--panel); border:1px solid var(--line); border-radius:20px; padding:18px; box-shadow:0 18px 52px rgba(0,0,0,0.22); }
    .card h2, .card h3 { margin:0 0 12px; font-size:16px; }
    .kpi { font-size: 30px; font-weight: 800; letter-spacing: -0.04em; }
    .muted { color: var(--muted); font-size: 13px; }
    .notice { margin-bottom: 14px; border-color: rgba(240,194,123,0.3); color: var(--warn); }
    .stack { display:grid; gap:10px; }
    .row { display:grid; gap:12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
    input, textarea, select, button { width:100%; box-sizing:border-box; background:var(--panel2); color:var(--text); border:1px solid var(--line); border-radius:12px; padding:10px 12px; font:inherit; }
    button { background: linear-gradient(180deg, rgba(115,213,172,0.18), rgba(115,213,172,0.08)); cursor:pointer; font-weight:700; }
    .table { width:100%; border-collapse: collapse; font-size: 14px; }
    .table th, .table td { text-align:left; padding: 10px 8px; border-bottom:1px solid var(--line); vertical-align: top; }
    .table th { color: var(--muted); font-size: 12px; text-transform: uppercase; letter-spacing: 0.08em; }
    .badge { display:inline-block; padding:4px 8px; border-radius:999px; border:1px solid rgba(115,213,172,0.3); background:rgba(115,213,172,0.12); color:var(--accent); font-size:12px; }
    .warn { color: var(--warn); }
    .actions { display:flex; gap:12px; align-items:center; justify-content:space-between; flex-wrap:wrap; margin-bottom:10px; }
    .small { font-size:12px; }
    a { color: var(--accent); text-decoration:none; }
  </style>
</head>
<body>
<div class="wrap">
  <div class="hero">
    <div>
      <h1 class="title">NoVPN Admin</h1>
      <div class="sub">Dashboard tabs, VPN node monitoring, subscription plans, invites, promo traffic, policies, and server inventory.</div>
      <div class="topline" style="margin-top:12px;">
        <span class="chip">Panel: {{.PanelVersion}}</span>
        <span class="chip">Generated: {{.GeneratedAt.Format "2006-01-02 15:04 UTC"}}</span>
        <span class="chip">Monitoring rows: {{.ServerMonitoringCount}}</span>
      </div>
    </div>
    <img class="hero-art" src="{{.SiteImageURL}}" alt="NoVPN visual">
  </div>

  <div class="nav">
    {{range .Tabs}}
      <a class="tab {{if .Active}}active{{end}}" href="{{.URL}}">{{.Label}}</a>
    {{end}}
    <a class="tab" href="{{.PlanPageURL}}">Plans</a>
    <a class="tab" href="{{.ServerPageURL}}">Server catalog</a>
  </div>

  <div class="topline" style="margin-bottom:16px;">
    <span class="chip">Registry: {{.RegistryPath}}</span>
    <span class="chip">Config: {{.ConfigPath}}</span>
    <span class="chip">Profiles: {{.ClientProfilePath}}</span>
    <span class="chip">State: {{.StatePath}}</span>
  </div>

  {{if .Notice}}<div class="card notice">{{.Notice}}</div>{{end}}

  {{if eq .DashboardTab "overview"}}
  <div class="grid">
    <div class="card"><div class="muted">Active clients</div><div class="kpi">{{.Summary.ActiveClients}}</div></div>
    <div class="card"><div class="muted">Pending invites</div><div class="kpi">{{.Summary.PendingInvites}}</div></div>
    <div class="card"><div class="muted">Approx traffic</div><div class="kpi">{{formatBytes .Summary.TotalTrafficBytes}}</div></div>
    <div class="card"><div class="muted">Traffic-limited clients</div><div class="kpi">{{.Summary.TrafficBlockedClients}}</div></div>
    <div class="card"><div class="muted">Gateway ready</div><div class="kpi">{{if .Ready}}yes{{else}}check{{end}}</div></div>
    <div class="card"><div class="muted">Primary VPN</div><div class="kpi">{{.Summary.Server.PublicHost}}:{{.Summary.Server.PublicPort}}</div></div>
  </div>

  <div class="grid">
    <div class="card">
      <h2>Local monitoring</h2>
      <div class="stack">
        {{range .Metrics}}
        <div class="topline"><span class="chip">{{.Name}}</span><strong>{{.Value}}</strong></div>
        {{end}}
      </div>
    </div>
    <div class="card">
      <h2>Runtime details</h2>
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
    <div class="card">
      <h2>Infrastructure</h2>
      <div class="stack">
        {{range .InfrastructureRows}}
        <div>
          <strong>{{.Name}}</strong>
          <div class="muted">{{.Purpose}}</div>
          <div class="small">{{.Services}}</div>
          <div class="small">Host: {{.Host}}</div>
          <div class="small">Status: {{.Health}}</div>
        </div>
        {{end}}
      </div>
    </div>
  </div>
  {{end}}

  {{if eq .DashboardTab "clients"}}
  <div class="card">
    <div class="actions">
      <h2>Clients</h2>
      <div class="topline">
        <a href="{{.BasePath}}/dashboard?tab=clients&clients_sort=traffic_desc">Traffic desc</a>
        <a href="{{.BasePath}}/dashboard?tab=clients&clients_sort=traffic_asc">Traffic asc</a>
      </div>
    </div>
    <table class="table">
      <thead><tr><th>Name</th><th>Device</th><th>UUID</th><th>Key</th><th>Traffic</th><th>Remaining</th><th>Status</th><th>Profile</th><th>Action</th></tr></thead>
      <tbody>
      {{range .Clients}}
        <tr>
          <td>{{.Name}}</td>
          <td>{{.DeviceName}}<div class="muted small">{{.DeviceID}}</div>{{if .ObservedDevices}}<div class="muted small">Connected: {{formatObservedDevices .ObservedDevices}}</div>{{end}}</td>
          <td class="small">{{.UUID}}</td>
          <td class="small">{{if .InviteCode}}{{.InviteCode}}{{else}}bootstrap{{end}}</td>
          <td>{{formatBytes .TrafficUsedBytes}}<div class="muted small">limit {{formatTrafficLimit .TrafficLimitBytes}}</div></td>
          <td>{{formatTrafficRemain .}}</td>
          <td>{{if .RevokedAt}}<span class="chip">revoked</span>{{else if .TrafficBlockedAt}}<span class="chip">limit reached</span>{{else if .Active}}<span class="badge">active</span>{{else}}<span class="chip">inactive</span>{{end}}</td>
          <td><a href="{{$.BasePath}}/api/clients/{{.ID}}/profile.yaml">yaml</a> · <a href="{{$.BasePath}}/api/clients/{{.ID}}/vless.txt">vless</a> · <a href="{{$.BasePath}}/api/clients/{{.ID}}/subscription.txt">sub</a></td>
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
  {{end}}

  {{if eq .DashboardTab "access"}}
  <div class="grid">
    <div class="card">
      <h2>Create invite</h2>
      <form method="post" action="{{.BasePath}}/api/invites" class="stack">
        <select name="plan_id">
          <option value="">No subscription plan</option>
          {{range .Plans}}<option value="{{.ID}}">{{.Name}} ({{.ID}})</option>{{end}}
        </select>
        <input name="name" placeholder="Invite name">
        <textarea name="note" rows="3" placeholder="Note"></textarea>
        <input name="max_uses" type="number" min="1" value="1" placeholder="Allowed activations">
        <input name="traffic_limit_gb" type="number" min="0" step="0.1" value="0" placeholder="Traffic limit in GiB">
        <input name="expires_minutes" type="number" min="0" placeholder="Expires in minutes">
        <button type="submit">Create invite</button>
      </form>
    </div>
    <div class="card">
      <h2>Create promo code</h2>
      <form method="post" action="{{.BasePath}}/api/promos" class="stack">
        <input name="code" placeholder="Custom code (optional)">
        <input name="name" placeholder="Promo name">
        <textarea name="note" rows="3" placeholder="Note"></textarea>
        <input name="bonus_gb" type="number" min="0" step="0.1" value="1" placeholder="Bonus traffic in GiB">
        <input name="max_uses" type="number" min="0" value="0" placeholder="Max uses, 0 = unlimited">
        <input name="expires_minutes" type="number" min="0" placeholder="Expires in minutes">
        <button type="submit">Create promo</button>
      </form>
    </div>
    <div class="card">
      <h2>Subscription plans</h2>
      <div class="muted small">Use the dedicated pages for full plan and server management.</div>
      <div class="stack" style="margin-top:12px;">
        {{range .Plans}}
          <div>
            <strong>{{.Name}}</strong>
            <div class="muted">{{.ID}}</div>
            <div class="small">{{if .DurationDays}}{{.DurationDays}} days{{else}}unlimited{{end}} · {{formatTrafficLimit .TrafficLimitBytes}}</div>
          </div>
        {{end}}
      </div>
      <div class="topline" style="margin-top:16px;">
        <a class="tab" href="{{.PlanPageURL}}">Open plans</a>
        <a class="tab" href="{{.ServerPageURL}}">Open server catalog</a>
      </div>
    </div>
  </div>

  <div class="card">
    <h2>Invites</h2>
    <table class="table">
      <thead><tr><th>Code</th><th>Name</th><th>Status</th><th>Uses</th><th>Traffic</th><th>Expiry</th><th>Redeem URL</th></tr></thead>
      <tbody>
      {{range .Invites}}
        <tr>
          <td class="small">{{.Code}}</td>
          <td>{{.Name}}<div class="muted small">{{.Note}}</div></td>
          <td>{{if .Active}}<span class="badge">active</span>{{else}}<span class="chip">inactive</span>{{end}}</td>
          <td class="small">{{.ActiveUses}} / {{.MaxUses}}<div class="muted">total {{.RedeemedUses}}</div></td>
          <td class="small">{{formatTrafficLimit .TrafficLimitBytes}}</td>
          <td class="small">{{if .ExpiresAt}}{{.ExpiresAt.Format "2006-01-02 15:04:05"}}{{else}}never{{end}}</td>
          <td class="small">{{$.BasePath}}/redeem/{{.Code}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>

  <div class="card">
    <h2>Promo codes</h2>
    <table class="table">
      <thead><tr><th>Code</th><th>Name</th><th>Status</th><th>Bonus</th><th>Uses</th><th>Expiry</th></tr></thead>
      <tbody>
      {{range .Promos}}
        <tr>
          <td class="small">{{.Code}}</td>
          <td>{{.Name}}<div class="muted small">{{.Note}}</div></td>
          <td>{{if .Active}}<span class="badge">active</span>{{else}}<span class="chip">inactive</span>{{end}}</td>
          <td class="small">{{formatBytes .BonusBytes}}</td>
          <td class="small">{{formatPromoUses .}}</td>
          <td class="small">{{if .ExpiresAt}}{{.ExpiresAt.Format "2006-01-02 15:04:05"}}{{else}}never{{end}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>
  {{end}}

  {{if eq .DashboardTab "policies"}}
  <div class="grid">
    <div class="card">
      <h2>Client blocklist</h2>
      <div class="muted small">Public API: <a href="{{.ClientPolicyURL}}" target="_blank" rel="noreferrer">{{.ClientPolicyURL}}</a></div>
      <form method="post" action="{{.BasePath}}/api/policy/blocklist" class="stack" style="margin-top:12px;">
        <label class="muted small">Blocked sites/domains</label>
        <textarea name="blocked_sites" rows="8" placeholder="example.com&#10;bad-site.net">{{joinLines .ClientPolicy.BlockedSites "\n"}}</textarea>
        <label class="muted small">Blocked app package IDs</label>
        <textarea name="blocked_apps" rows="8" placeholder="com.example.app">{{joinLines .ClientPolicy.BlockedApps "\n"}}</textarea>
        <button type="submit">Save blocklist</button>
      </form>
    </div>
    <div class="card">
      <h2>Mandatory notifications</h2>
      <div class="muted small">Public API: <a href="{{.ClientNoticesURL}}" target="_blank" rel="noreferrer">{{.ClientNoticesURL}}</a></div>
      <form method="post" action="{{.BasePath}}/api/policy/notices" class="stack" style="margin-top:12px;">
        <input name="title" placeholder="Title">
        <textarea name="message" rows="4" placeholder="Message shown to users"></textarea>
        <input name="expires_minutes" type="number" min="0" placeholder="Expires in minutes">
        <button type="submit">Push notification</button>
      </form>
      <table class="table" style="margin-top:14px;">
        <thead><tr><th>Title</th><th>Status</th><th>Created</th><th>Expiry</th><th>Action</th></tr></thead>
        <tbody>
        {{range .MandatoryNotices}}
          <tr>
            <td>{{.Title}}<div class="muted small">{{.Message}}</div></td>
            <td>{{if .Active}}<span class="badge">active</span>{{else}}<span class="chip">inactive</span>{{end}}</td>
            <td class="small">{{.CreatedAt.Format "2006-01-02 15:04:05"}}</td>
            <td class="small">{{if .ExpiresAt}}{{.ExpiresAt.Format "2006-01-02 15:04:05"}}{{else}}never{{end}}</td>
            <td>
              {{if .Active}}
              <form method="post" action="{{$.BasePath}}/api/policy/notices/{{.ID}}/deactivate">
                <button type="submit">Deactivate</button>
              </form>
              {{else}}<span class="muted small">-</span>{{end}}
            </td>
          </tr>
        {{end}}
        </tbody>
      </table>
    </div>
  </div>
  {{end}}

  {{if eq .DashboardTab "monitoring"}}
  <div class="card">
    <div class="actions">
      <div>
        <h2>VPN server monitoring</h2>
        <div class="muted small">Data refreshes once per hour automatically. Last refresh: {{.MonitoringObservedAt}}</div>
      </div>
      <a class="tab active" href="{{.MonitoringRefreshURL}}">Check now</a>
    </div>
    <table class="table">
      <thead><tr><th>Server</th><th>Status</th><th>CPU load</th><th>Memory</th><th>SSD</th><th>Uptime</th><th>Observed</th></tr></thead>
      <tbody>
      {{range .MonitoringRows}}
        <tr>
          <td>
            {{.Name}}<div class="muted small">{{.Role}} · {{.Endpoint}}</div>
            <div class="muted small">{{.Purpose}}</div>
            <div class="muted small">Monitor: {{.MonitorURL}}</div>
          </td>
          <td>{{if eq .Health "ready"}}<span class="badge">{{.Health}}</span>{{else}}<span class="warn">{{.Health}}</span>{{end}}{{if .Error}}<div class="muted small">{{.Error}}</div>{{end}}</td>
          <td>{{.CPU}}<div class="muted small">{{if .Hostname}}{{.Hostname}}{{else}}unknown host{{end}}</div></td>
          <td>{{.Memory}}</td>
          <td>{{.Disk}}</td>
          <td>{{.Uptime}}</td>
          <td>{{.ObservedAt}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>
  {{end}}

  {{if eq .DashboardTab "servers"}}
  <div class="grid">
    <div class="card">
      <div class="actions">
        <h2>Infrastructure host</h2>
        <a class="tab active" href="{{.BasePath}}/dashboard?tab=servers&refresh_monitoring=1">Check now</a>
      </div>
      <table class="table">
        <thead><tr><th>Name</th><th>Host</th><th>Services</th><th>Purpose</th><th>Health</th></tr></thead>
        <tbody>
        {{range .InfrastructureRows}}
          <tr>
            <td>{{.Name}}<div class="muted small"><a href="{{.PublicURL}}" target="_blank" rel="noreferrer">{{.PublicURL}}</a></div></td>
            <td>{{.Host}}</td>
            <td>{{.Services}}</td>
            <td>{{.Purpose}}</td>
            <td>{{.Health}}</td>
          </tr>
        {{end}}
        </tbody>
      </table>
    </div>
  </div>

  <div class="card">
    <div class="actions">
      <div>
        <h2>VPN server list</h2>
        <div class="muted small">Role, purpose, subscription assignment, and monitoring status.</div>
      </div>
      <a class="tab" href="{{.ServerPageURL}}">Open catalog editor</a>
    </div>
    <table class="table">
      <thead><tr><th>Name</th><th>Role</th><th>Purpose</th><th>Endpoint</th><th>Assigned plans</th><th>Monitoring</th></tr></thead>
      <tbody>
      {{range .VPNInventoryRows}}
        <tr>
          <td>{{.Name}}<div class="muted small">{{.ID}} · {{.Location}}</div></td>
          <td>{{.Role}}</td>
          <td>{{.Purpose}}</td>
          <td>{{.Endpoint}}<div class="muted small">{{.MonitorURL}}</div></td>
          <td>{{.AssignedTo}}</td>
          <td>{{.Health}}<div class="muted small">{{.ObservedAt}}</div><div class="muted small">CPU {{.CPU}} · RAM {{.Memory}} · SSD {{.Disk}}</div>{{if .MonitorNote}}<div class="muted small">{{.MonitorNote}}</div>{{end}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>
  {{end}}
</div>
</body>
</html>`

const adminPlansTemplateV2 = `<!doctype html>
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
    .nav { display:flex; gap:10px; flex-wrap:wrap; margin-bottom: 14px; }
    .tab { display:inline-flex; padding:8px 12px; border-radius:999px; border:1px solid #223246; color:#8ea3bb; text-decoration:none; }
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
  <div class="nav">
    <a class="tab" href="{{.BasePath}}/dashboard?tab=overview">Overview</a>
    <a class="tab" href="{{.BasePath}}/dashboard?tab=access">Access</a>
    <a class="tab" href="{{.BasePath}}/dashboard?tab=servers">Servers</a>
  </div>
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

const adminServersTemplateV2 = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>NoVPN Servers</title>
  <style>
    body { margin:0; font-family: ui-sans-serif, system-ui, sans-serif; background:#0a1018; color:#eef5ff; }
    .wrap { max-width: 1180px; margin: 0 auto; padding: 28px 20px 56px; }
    .card { background:#101925; border:1px solid #223246; border-radius:22px; padding:18px; margin-top:16px; }
    .row { display:grid; gap:12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
    .nav { display:flex; gap:10px; flex-wrap:wrap; margin-bottom: 14px; }
    .tab { display:inline-flex; padding:8px 12px; border-radius:999px; border:1px solid #223246; color:#8ea3bb; text-decoration:none; }
    input, textarea, button { width:100%; box-sizing:border-box; background:#141f2e; color:#eef5ff; border:1px solid #223246; border-radius:14px; padding:10px 12px; font:inherit; }
    button { background:#17324a; cursor:pointer; font-weight:700; }
    .table { width:100%; border-collapse:collapse; margin-top:12px; }
    .table th, .table td { text-align:left; padding:10px 8px; border-bottom:1px solid #223246; vertical-align:top; }
    a { color:#7acaa7; text-decoration:none; }
    .muted { color:#8ea3bb; }
    .actions { display:flex; gap:12px; align-items:center; justify-content:space-between; flex-wrap:wrap; }
  </style>
</head>
<body>
<div class="wrap">
  <div class="nav">
    <a class="tab" href="{{.BasePath}}/dashboard?tab=overview">Overview</a>
    <a class="tab" href="{{.BasePath}}/dashboard?tab=monitoring">Monitoring</a>
    <a class="tab" href="{{.BasePath}}/plans">Plans</a>
  </div>

  <div class="card">
    <h1>Server catalog</h1>
    <p class="muted">Register VPN nodes, monitoring endpoints, roles, and purpose descriptions.</p>
    <form method="post" action="{{.BasePath}}/api/servers">
      <div class="row">
        <input name="id" placeholder="Server ID (optional)">
        <input name="name" placeholder="Display name">
        <input name="address" placeholder="Public IP or host">
        <input name="port" type="number" min="1" placeholder="VPN port">
        <input name="role" placeholder="Role, e.g. vpn-primary">
        <input name="flow" placeholder="Flow, e.g. xtls-rprx-vision">
        <input name="server_name" placeholder="SNI / server name">
        <input name="fingerprint" placeholder="Fingerprint, e.g. chrome">
        <input name="public_key" placeholder="REALITY public key">
        <input name="short_id" placeholder="Primary short ID">
        <input name="location_label" placeholder="Location label">
        <input name="monitor_url" placeholder="Monitor URL, e.g. http://host:9202/control-plane/system">
      </div>
      <textarea name="purpose" rows="3" style="margin-top:12px;" placeholder="Purpose / what this server is used for"></textarea>
      <textarea name="short_ids" rows="3" style="margin-top:12px;" placeholder="Short IDs, one per line"></textarea>
      <div class="row" style="margin-top:12px;">
        <input name="spider_x" placeholder="SpiderX path">
        <label><input type="checkbox" name="vpn_only"> VPN only</label>
        <label><input type="checkbox" name="primary"> Primary node</label>
      </div>
      <button type="submit" style="margin-top:12px;">Create server</button>
    </form>
  </div>

  <div class="card">
    <div class="actions">
      <div>
        <h2>Infrastructure host</h2>
        <div class="muted">Observed at {{.MonitoringObservedAt}}</div>
      </div>
      <a class="tab" href="{{.MonitoringRefreshURL}}">Check now</a>
    </div>
    <table class="table">
      <thead><tr><th>Name</th><th>Host</th><th>Services</th><th>Purpose</th><th>Health</th></tr></thead>
      <tbody>
      {{range .InfrastructureRows}}
        <tr>
          <td>{{.Name}}</td>
          <td>{{.Host}}</td>
          <td>{{.Services}}</td>
          <td>{{.Purpose}}</td>
          <td>{{.Health}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>

  <div class="card">
    <h2>VPN servers</h2>
    <table class="table">
      <thead><tr><th>ID</th><th>Name</th><th>Role</th><th>Purpose</th><th>Endpoint</th><th>Assigned plans</th><th>Monitoring</th></tr></thead>
      <tbody>
      {{range .VPNInventoryRows}}
        <tr>
          <td>{{.ID}}</td>
          <td>{{.Name}}<div class="muted small">{{.Location}}</div></td>
          <td>{{.Role}}</td>
          <td>{{.Purpose}}</td>
          <td>{{.Endpoint}}<div class="muted small">{{.MonitorURL}}</div></td>
          <td>{{.AssignedTo}}</td>
          <td>{{.Health}}<div class="muted small">{{.ObservedAt}}</div><div class="muted small">CPU {{.CPU}} · RAM {{.Memory}} · SSD {{.Disk}}</div>{{if .MonitorNote}}<div class="muted small">{{.MonitorNote}}</div>{{end}}</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </div>
</div>
</body>
</html>`
