# transport-gateway PoC


Быстрый старт для gateway:

```bash
go build -o gateway ./cmd/gateway
./gateway -config deploy/config.example.yaml
```

Быстрый старт для серверного Reality core:

```bash
go build -o reality-bootstrap ./cmd/reality-bootstrap
sudo ./reality-bootstrap -config deploy/config.example.yaml
```

`reality-bootstrap` делает следующее:

- генерирует UUID, X25519 private/public key и short ID, если они не заданы;
- сохраняет их в `core.reality.xray.state_path`;
- рендерит `config.json` для Xray в `core.reality.xray.config_path`;
- экспортирует клиентский профиль в `core.reality.xray.client_profile_path`;
- при `install.method: official-script` вызывает официальный `XTLS/Xray-install`;
- валидирует конфиг через `xray run -test -config ...` и перезапускает `xray.service`.

Для сухого прогона без установки и `systemctl`:

```bash
./reality-bootstrap -config deploy/config.example.yaml -render-only
```

Endpoints gateway:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

Gateway security defaults are now hardened:

- `security.auth.mode: source_ip_allowlist`
- `security.acl.mode: policy`

If you expose listeners publicly, update `security.auth.allowed_cidrs` explicitly in your config.

## Client Profile Sync

The server bootstrap exports a client profile YAML to
`core.reality.xray.client_profile_path` (default: `/var/lib/novpn/reality/client-profile.yaml`).

You can sync that file into the bundled desktop client profile and Android bootstrap asset with:

```bash
go run ./cmd/client-profile-sync \
  -input /path/to/client-profile.yaml \
  -bootstrap-address 2.26.85.47 \
  -bootstrap-api-base http://2.26.85.47/admin
```

## Desktop Client

The desktop client lives under `client/desktop/python/` and now supports:

- a bundled default profile plus imported server profiles;
- invite redemption that can import multiple server profiles at once;
- per-profile `api_base` handling for redeem and disconnect flows;
- desktop-side logs in addition to `xray` and `obfuscator` runtime logs;
- mouse-wheel scrolling in the main window and settings window;
- headless config generation and embedded runtime lifecycle commands;
- portable `.exe` and Inno Setup installer builds for Windows;
- an initial Windows system-tunnel mode based on Xray TUN + `wintun.dll`.

Local repo run:

```powershell
python client/desktop/python/app.py
```

Headless example:

```powershell
python client/desktop/python/app.py --headless --bypass-ru --start-runtime
```

Headless system-tunnel example:

```powershell
python client/desktop/python/app.py --headless --system-tunnel --start-runtime
```

Repo-mode generated files are written under `client/desktop/python/generated/`.
Packaged Windows builds write user state and logs under `%LOCALAPPDATA%\\NoVPN Desktop\\generated`.

Windows build script:

```powershell
powershell -ExecutionPolicy Bypass -File client/desktop/python/build_windows.ps1 -Version 0.1.0
```

If `ISCC.exe` is not in `PATH`, the script also checks `.tools/InnoSetup/ISCC.exe`.

## Admin Panel

The control plane can now run separately from the VPN edge node. The repo includes three entry
points:

- `cmd/admin-service`: admin panel, invite registry, subscription plans, server catalog
- `cmd/pay-service`: independent order/payment facade that issues invites through the control plane
- `cmd/vpn-service`: VPN edge node that syncs registry and traffic usage with the control plane

Deployment presets for the split layout live in:

- `deploy/admin-service/`
- `deploy/pay-service/`
- `deploy/vpn-service/`
- `deploy/README.md`

The admin panel supports:

- creating invite codes with both expiry and max-use limits;
- managing subscription plans and attaching VPN nodes to plans;
- managing VPN node catalog entries independently from plans;
- redeeming invites into per-device client records;
- downloading per-client profile YAML files;
- revoking clients;
- viewing service metrics and quota state on the dashboard.

Default example config:

```yaml
admin:
  enabled: true
  listen_addr: 127.0.0.1:9112
  storage_path: /var/lib/novpn/admin
  token: change-me-admin-token
  base_path: /admin
```

Recommended access pattern is an SSH tunnel:

```bash
ssh -L 9112:127.0.0.1:9112 root@YOUR_SERVER_IP
```

Then open:

```text
http://127.0.0.1:9112/admin
```

The panel supports:

- creating invite codes with both expiry and max-use limits;
- redeeming invites into per-device client records;
- downloading per-client profile YAML files;
- revoking clients;
- viewing gateway Prometheus counters on the dashboard.

API auth works with the configured token via login form, `Authorization: Bearer ...`, or `X-Admin-Token`.

## Full Project Docs (RU)

For a full technical walkthrough of encryption, traffic flow, Android/desktop runtime behavior,
obfuscation internals, and customization guides, see:

- [docs/project-guide-ru.md](/d:/projekt/noVPN/docs/project-guide-ru.md)
