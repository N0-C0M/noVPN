# transport-gateway PoC

Минимальный Go-каркас TCP/UDP transport gateway перед VPN-ядром.

Что уже есть:

- отдельные TCP и UDP listeners;
- graceful shutdown;
- health/readiness endpoints;
- Prometheus metrics;
- upstream dialer;
- `noop`-реализации для auth и ACL, чтобы PoC запускался без внешней инфраструктуры.

Быстрый старт:

```bash
go mod tidy
go build -o gateway ./cmd/gateway
./gateway -config deploy/config.example.yaml
```

Endpoints:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

Что заменить следующим шагом:

- `internal/auth/manager.go` на реальный `mTLS`/token auth;
- `internal/acl/evaluator.go` на allowlist policy engine;
- `internal/ratelimit/limiter.go` на полноценные per-IP/PPS/BPS лимиты;
- `internal/upstream/dialer.go` на интеграцию с реальным VPN backend profile routing.
