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
