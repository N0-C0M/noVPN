# transport-gateway PoC

Минимальный Go-каркас transport gateway и bootstrap-утилита для серверного Xray VLESS + XTLS-Reality.

Что уже есть:

- отдельные TCP и UDP listeners для gateway;
- graceful shutdown;
- health/readiness endpoints;
- Prometheus metrics;
- upstream dialer;
- `noop`-реализации для auth и ACL;
- автоматический bootstrap серверного Reality core без заранее заданных ключей.

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

## Client Profile Sync

The server bootstrap exports a client profile YAML to
`core.reality.xray.client_profile_path` (default: `/var/lib/novpn/reality/client-profile.yaml`).

You can sync that file into the bundled desktop client profile and Android bootstrap asset with:

```bash
go run ./cmd/client-profile-sync -input /path/to/client-profile.yaml
```

## Admin Panel

The gateway can expose a small admin panel for Reality registry management and monitoring.

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
