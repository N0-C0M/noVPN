# NoVPN Full Technical Guide (EN)

This document describes the implementation currently present in this repository:
server, Android and desktop clients, traffic flow, encryption/obfuscation, code-based activation,
client policy controls, and operational/security guidance.

## 1. Project Scope

Main components:

- Go gateway: `cmd/gateway`, `internal/*`
- Reality bootstrap/provisioning: `cmd/reality-bootstrap`, `internal/core/reality/*`
- Admin panel and API: `internal/server/admin.go`
- Android client: `client/android/app/src/main/java/com/novpn/*`
- Desktop client (Python/Tkinter): `client/desktop/python/novpn_client/*`
- Obfuscator runtime: `cmd/obfuscator/*`

Current baseline:

- Gateway defaults are hardened (`security.auth.mode=source_ip_allowlist`, `security.acl.mode=policy`).
- Obfuscator supports SOCKS5 `CONNECT` and `UDP ASSOCIATE` forwarding paths.
- Android provides a full TUN path (`VpnService` + `tun2proxy`).
- Desktop orchestrates local runtime binaries and proxies, includes bundled/imported multi-server profiles, writes dedicated desktop logs, and now has an initial Windows system-tunnel path based on Xray TUN + `wintun.dll`.

## 2. High-Level Architecture

### 2.1 Server Side

1. `reality-bootstrap`
- Generates and persists state (UUID, X25519 keys, short IDs)
- Renders Xray config
- Manages registry data (clients, invites, promos)
- Exports client profile YAML

2. `gateway`
- Starts TCP/UDP listeners
- Applies auth and ACL checks
- Proxies to upstream targets
- Exposes health/readiness/metrics

3. `admin`
- Dashboard and lifecycle management (invites, promos, clients)
- Public redeem/disconnect/diagnostics endpoints
- Client blocklist + mandatory notices management

### 2.2 Client Side

Android client:

- Builds runtime configs for Xray and obfuscator
- Starts embedded native/runtime binaries
- Creates VPN TUN interface via `VpnService`
- Bridges TUN traffic to local proxy chain

Desktop client:

- Loads a bundled default profile and imported profiles
- Preserves per-profile `server_id` and `api_base`
- Accepts invite responses that return multiple server profiles
- Builds runtime configs
- Starts/stops `xray.exe` and `obfuscator.exe`
- Writes `desktop-client.log` plus runtime logs
- Provides UI for activation, promo, diagnostics, routing, settings, and mouse-wheel scrolling
- Supports Windows installer flow (Inno Setup)
- Supports a Windows system-tunnel mode when launched with Administrator rights

## 3. Encryption and Obfuscation

### 3.1 Channel Encryption

External channel encryption is handled by Xray (`vless` + `reality` stream security).
Obfuscator is not replacing transport cryptography; it modifies traffic behavior patterns.

Relevant code:

- Android config writer: `client/android/.../AndroidXrayConfigWriter.kt`
- Desktop config builder: `client/desktop/python/novpn_client/config_builder.py`
- Server provisioning: `internal/core/reality/provision.go`

### 3.2 Obfuscator Behavior

Obfuscator runtime (`cmd/obfuscator/runtime.go`):

- accepts SOCKS5 requests
- handles both `CONNECT` and `UDP ASSOCIATE`
- applies relay plan timing/chunking behavior
- forwards through upstream SOCKS

Pattern controls are generated from seed + session context (`session nonce`, rotation bucket, destination).

## 4. Data Flow

### 4.1 Android Path

`App traffic -> VpnService(TUN) -> tun2proxy -> local obfuscator SOCKS -> local Xray SOCKS -> VLESS/REALITY -> server`

At service startup the client probes `UDP ASSOCIATE` support on the local obfuscator bridge. If the
probe passes, that bridge is used for the whole VPN session. If it fails, the whole VPN session
falls back to `TUN -> tun2proxy -> local Xray SOCKS (UDP enabled) -> VLESS/REALITY`. This is a
session-wide bridge choice, not a per-site or YouTube-specific heuristic.

### 4.2 Desktop Path

Desktop can now operate in two modes:

- local runtime mode: local SOCKS/HTTP inbounds plus embedded `xray.exe` and `obfuscator.exe`
- Windows system-tunnel mode: the same runtime plus an Xray `tun` inbound backed by `wintun.dll`

The Windows system-tunnel path currently:

- requires Administrator rights
- applies temporary IPv4 route changes for the current session
- still keeps local SOCKS/HTTP inbounds alive for diagnostics and explicit proxy use
- does not yet include a packaged WFP helper in the default desktop build

### 4.3 Server Path

- TCP proxy: `internal/transport/tcp/proxy.go`
- UDP proxy: `internal/transport/udp/proxy.go`
- Traffic accounting and limits: `internal/core/reality/traffic.go`

## 5. Invite / Promo / Device Lifecycle

Registry entities (`internal/core/reality/registry.go`):

- `InviteRecord`
- `PromoRecord`
- `ClientRecord`

Public redeem endpoint:

- `POST /admin/redeem/{code}`

Behavior:

- invite redemption creates/updates a per-device client record
- invite redemption may return one or more client profiles for different VPN servers
- promo redemption adds traffic bonus to an already bound device

Disconnect endpoint:

- `POST /admin/disconnect`

## 6. Custom and Temporary Promo Codes

Promo creation now supports:

- custom code (`code`)
- max usage count (`max_uses`, `0` means unlimited)
- expiry window (`expires_minutes`) for temporary promo codes

Rules:

- If `code` is empty, server auto-generates one.
- Custom code is normalized to lowercase and validated.
- Custom code must be unique across invite and promo namespaces.
- Promo becomes inactive when expired or when max usage is reached.

Admin API/UI:

- endpoint: `POST /admin/api/promos`
- HTML dashboard form includes `code`, `bonus_gb`, `max_uses`, `expires_minutes`

## 7. Android Default Whitelist Mode

Android client supports default whitelist mode and can be toggled in settings.
Default package set includes YouTube family, Telegram, Instagram, X, selected Supercell games,
MEGA, ChatGPT, and Gemini package IDs (see `ClientPreferences.kt` and `SettingsActivity.kt`).

## 8. Admin Client Policy Controls

Admin panel supports:

- Blocked sites/domains list
- Blocked application package IDs list
- Mandatory user notices (with optional expiry)

Public endpoints:

- `GET /admin/client/policy`
- `GET /admin/client/notices`

Admin update endpoints:

- `POST /admin/api/policy/blocklist`
- `POST /admin/api/policy/notices`
- `POST /admin/api/policy/notices/{id}/deactivate`

## 9. Build and Deployment

Basic build:

```bash
go build -o gateway ./cmd/gateway
go build -o reality-bootstrap ./cmd/reality-bootstrap
```

Bootstrap and render:

```bash
./reality-bootstrap -config deploy/config.example.yaml -render-only
```

Windows desktop build and installer:

- Build script: `client/desktop/python/build_windows.ps1`
- Inno Setup script: `client/desktop/installer/novpn-desktop.iss`
- The build script accepts `bootstrap.json` from either `client/android/app/src/main/secure/` or `client/android/app/src/main/assets/`
- If `ISCC.exe` is not in `PATH`, the build script also checks `.tools/InnoSetup/ISCC.exe`

Desktop log locations:

- repo mode: `client/desktop/python/generated/logs/desktop-client.log`
- packaged mode: `%LOCALAPPDATA%\NoVPN Desktop\generated\logs\desktop-client.log`

Windows tunnel / WFP scaffolding:

- TUN orchestration: `client/desktop/python/novpn_client/windows_tunnel.py`
- WFP helper source scaffold: `client/desktop/windows/wfp/novpn_wfp_helper.cpp`

## 10. Security Notes

Production recommendations:

1. Keep gateway security modes in hardened config (`source_ip_allowlist` + `policy`).
2. Protect admin UI/API with private access pattern (SSH tunnel/VPN/reverse proxy + TLS).
3. Treat public redeem/disconnect endpoints as sensitive and guard with perimeter controls/rate-limits.
4. Rotate admin token and avoid exposing admin listener to public internet.
5. Keep geo data and app/domain catalogs up to date.

## 11. Code Pointers

Server:

- `cmd/gateway/main.go`
- `internal/server/gateway.go`
- `internal/server/admin.go`
- `internal/core/reality/registry.go`
- `internal/core/reality/traffic.go`
- `deploy/config.example.yaml`

Android:

- `client/android/.../NoVpnService.kt`
- `client/android/.../AndroidXrayConfigWriter.kt`
- `client/android/.../ObfuscatorConfigWriter.kt`
- `client/android/.../ClientPreferences.kt`
- `client/android/.../SettingsActivity.kt`

Desktop:

- `client/desktop/python/novpn_client/app.py`
- `client/desktop/python/novpn_client/runtime_manager.py`
- `client/desktop/python/novpn_client/config_builder.py`
- `client/desktop/python/novpn_client/obfuscator_config_builder.py`
- `client/desktop/python/build_windows.ps1`
