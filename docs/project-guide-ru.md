# NoVPN: Полная Техническая Документация (RU)

Документ описывает фактическую реализацию в текущем репозитории: серверную часть, Android/desktop клиенты, транспорт, шифрование, обфускацию, split tunneling, активацию по кодам и кастомизацию "скрытия".

## 1. Область и статус проекта

Проект состоит из нескольких подсистем:

- серверный `gateway` на Go (`cmd/gateway`, `internal/*`);
- серверный bootstrap для `Xray VLESS + REALITY` (`cmd/reality-bootstrap`, `internal/core/reality/*`);
- admin-панель и API для invite/promo/device lifecycle (`internal/server/admin.go`);
- desktop-клиент (Python/Tkinter) (`client/desktop/python/*`);
- Android-клиент (Kotlin + `VpnService`) (`client/android/app/src/main/java/com/novpn/*`);
- отдельный бинарь `obfuscator` (`cmd/obfuscator/*`).

Важно:

- `gateway` по умолчанию использует `source_ip_allowlist` для auth и `policy` для ACL (см. `internal/config/config.go`, `internal/auth/source_ip_allowlist.go`, `internal/acl/policy.go`).
- obfuscator поддерживает SOCKS5 `CONNECT` (TCP) и `UDP ASSOCIATE` path.
- Android содержит полноценный TUN-путь (`VpnService + tun2proxy`), desktop-клиент в этом репозитории работает как локальный runtime с SOCKS/HTTP и UI-оркестрацией, без системного TUN.

---

## 2. Общая архитектура и роли компонентов

### 2.1 Сервер

1. `reality-bootstrap`:
- генерирует/поддерживает state (`UUID`, `X25519 keypair`, `short_ids`);
- рендерит `xray config.json`;
- поддерживает registry клиентов/инвайтов/промокодов;
- экспортирует `client-profile.yaml`;
- опционально ставит Xray и перезапускает `xray.service`.

2. `gateway`:
- поднимает TCP/UDP listeners;
- аутентифицирует/авторизует трафик через интерфейсы `auth.Manager` и `acl.Evaluator`;
- проксирует трафик в upstream.

3. `admin`:
- UI+API для invite/promo/client lifecycle;
- публичные endpoint'ы для redeem/disconnect/diagnostics.

### 2.2 Клиенты

1. Android:
- формирует runtime-конфиги Xray и obfuscator;
- запускает embedded бинарники;
- создаёт VPN-интерфейс через `VpnService`;
- мостит TUN трафик в локальный SOCKS через `tun2proxy`.

2. Desktop:
- импорт/хранение профилей;
- генерация Xray/obfuscator конфигов;
- запуск/остановка `xray.exe` и `obfuscator.exe`;
- UI для invite/promo/diagnostics/settings;
- сборка в `exe` и setup installer.

---

## 3. Шифрование и безопасность канала

## 3.1 Где реально происходит шифрование

Основное шифрование внешнего канала реализуется Xray:

- outbound protocol: `vless`;
- stream security: `reality`;
- flow: обычно `xtls-rprx-vision`.

См.:

- desktop builder: `client/desktop/python/novpn_client/config_builder.py`;
- Android builder: `client/android/app/src/main/java/com/novpn/xray/AndroidXrayConfigWriter.kt`;
- server rendering: `internal/core/reality/provision.go`.

## 3.2 Что делает obfuscator относительно шифрования

`obfuscator` не выполняет end-to-end криптографическое шифрование полезной нагрузки. Его задача:

- SOCKS-прокси мост;
- параметризованное изменение временного/пакетного паттерна потока (chunking, jitter, idle gaps).

Шифрование снаружи всё равно обеспечивает Xray (VLESS/REALITY), а obfuscator добавляет слой "маскировки поведения трафика".

## 3.3 Локальные границы безопасности

Android hardening локального SOCKS:

- случайный loopback host (не только `127.0.0.1`, а случайный `127.x.x.x`);
- случайные username/password;
- случайный порт;
- управление UDP-возможностью локального in-proxy.

См. `RuntimeLocalProxyFactory`:

- `client/android/app/src/main/java/com/novpn/vpn/RuntimeLocalProxyConfig.kt`.

Desktop текущего scaffold использует фиксированные локальные порты из профиля (`10808/10809`) и не реализует такой же уровень "randomized local boundary", как Android.

## 3.4 Важная оговорка по admin API

Invite/promo/disconnect в клиентских кодах вызываются по `http://<server>/admin/...` (без TLS на уровне HTTP API).

См.:

- desktop: `client/desktop/python/novpn_client/invite_redeemer.py`;
- Android: `client/android/app/src/main/java/com/novpn/data/InviteRedeemer.kt`.

Рекомендация для production: ограничение доступа (VPN/SSH tunnel/reverse proxy + HTTPS), иначе API-трафик управления не защищён как публичный HTTPS.

---

## 4. Передача данных: цепочки трафика

## 4.1 Android datapath

Базовый путь:

`App traffic -> VpnService(TUN) -> tun2proxy -> local obfuscator SOCKS -> local Xray SOCKS -> VLESS/REALITY -> server`.

Упрощённый YouTube путь (при эвристике YouTube):

`TUN -> tun2proxy -> local Xray SOCKS (UDP enabled) -> VLESS/REALITY`.

Причина: текущий obfuscator не проксирует UDP end-to-end.

См.:

- `NoVpnService.kt`;
- `Tun2ProxyBridge.kt`;
- `EmbeddedRuntimeManager.kt`.

## 4.2 Desktop datapath

Desktop runtime:

- запускает локальный Xray (SOCKS+HTTP inbound);
- запускает локальный obfuscator;
- пишет конфиги и логи;
- даёт управление режимами маршрутизации и маскировки.

Системного TUN в текущем Python scaffold нет; фактическое использование зависит от приложений/сценария, где трафик идёт через локальные прокси.

См.:

- `client/desktop/python/novpn_client/runtime_manager.py`;
- `client/desktop/python/novpn_client/config_builder.py`.

## 4.3 Серверная сторона

### 4.3.1 `gateway` transport-layer

TCP/UDP прокси-сервисы:

- TCP: `internal/transport/tcp/proxy.go`;
- UDP: `internal/transport/udp/proxy.go`.

Возможности:

- per-connection/per-packet auth hook;
- ACL hook;
- rate limit hook;
- upstream dialer.

### 4.3.2 REALITY core и статистика

`reality-bootstrap` рендерит Xray-конфиг с:

- inbound VLESS+REALITY;
- stats API (`127.0.0.1:10085`);
- users из registry.

Traffic sync:

- `xray api statsquery --server=127.0.0.1:10085`;
- суммирование uplink/downlink per-user;
- применение лимитов в registry;
- при достижении лимита клиент блокируется.

См. `internal/core/reality/traffic.go`.

---

## 5. Профили клиента: формат и жизненный цикл

## 5.1 Базовый JSON профиль

`client/common/profiles/reality/default.profile.json`:

- `server`:
  - `address`, `port`, `uuid`, `flow`, `server_name`, `fingerprint`, `public_key`, `short_id`, `spider_x`;
- `local`:
  - `socks_listen/socks_port`, `http_listen/http_port`;
- `obfuscation`:
  - `seed`, `traffic_strategy`, `pattern_strategy`.

## 5.2 Генерация профиля с сервера

Сервер экспортирует `client-profile.yaml`.

Синхронизация в репозиторий:

```bash
go run ./cmd/client-profile-sync -input /path/to/client-profile.yaml
```

Утилита переписывает:

- `client/common/profiles/reality/default.profile.json`;
- `client/android/app/src/main/assets/bootstrap.json`.

Дополнительно поддерживает:

- `-seed` для override shared seed;
- `-name` для display name.

См. `cmd/client-profile-sync/main.go`.

## 5.3 Импорт и валидация на клиентах

И Android, и desktop:

- умеют импорт JSON/YAML;
- проверяют обязательные поля (`address`, `uuid`, `server_name`, `public_key`, `short_id`);
- откажут в запуске runtime при заглушках вида `REPLACE_WITH_...`.

---

## 6. Invite/Promo/Device lifecycle

## 6.1 Registry (сервер)

Registry хранит:

- invites (`InviteRecord`);
- promo codes (`PromoRecord`);
- clients (`ClientRecord`);
- лимиты/бонусы/used bytes;
- привязку device->client.

См. `internal/core/reality/registry.go`.

## 6.2 Redeem поток

Эндпоинт:

- `POST /admin/redeem/{code}`.

Возможные результаты:

- `kind=invite`:
  - выдаётся профиль(и) `client_profile_yaml`, `client_profiles_yaml`;
  - создаётся/обновляется client record.
- `kind=promo`:
  - начисляется трафик (bonus bytes) уже привязанному устройству.

См. `internal/server/admin.go`.

## 6.3 Создание кастомных и временных промокодов

Промокоды теперь поддерживают:

- кастомный `code` при создании;
- ограничение по количеству активаций `max_uses` (`0 = unlimited`);
- ограничение по времени `expires_minutes` (временный промокод).

Поведение:

- если `code` пустой, сервер генерирует код автоматически;
- кастомный код нормализуется к lowercase и проверяется на уникальность среди invite/promo кодов;
- при достижении `max_uses` промокод становится неактивным;
- при истечении `expires_minutes` промокод перестаёт активироваться.

Поля доступны и в HTML-форме админки, и в JSON API `POST /admin/api/promos`.

## 6.4 Disconnect устройства

Эндпоинт:

- `POST /admin/disconnect`.

По `device_id` (+ опционально `client_uuid`) устройство отвязывается, client деактивируется, runtime server-side refresh выполняется.

---

## 7. Android клиент: подробная работа

## 7.1 Runtime startup sequence

1. Получение входных параметров сервиса (`profileId`, routing mode, стратегии).
2. Preflight:
  - профиль валиден;
  - есть `xray`/`obfuscator` для ABI;
  - есть `geoip.dat`/`geosite.dat`;
  - загружены native libs для tun2proxy.
3. Подготовка effective profile:
  - seed из `ObfuscationSeedStore`;
  - runtime стратегии.
4. Построение session obfuscation plan.
5. Генерация Xray и obfuscator configs.
6. Старт obfuscator -> старт Xray.
7. `Tun2ProxyBridge.waitForLocalProxy(...)`.
8. `VpnService.Builder.establish()`, затем запуск tun2proxy.

См. `NoVpnService.kt`.

## 7.2 Routing и split tunneling

Режимы:

- `EXCLUDE_SELECTED`: выбранные пакеты идут мимо VPN (`addDisallowedApplication`).
- `ONLY_SELECTED`: только выбранные пакеты через VPN (`addAllowedApplication`).

Дополнительно:

- upstream bypass route (Android 13+) через `excludeRoute(...)` для адреса сервера.

## 7.3 Конфиг Xray на Android

Особенности:

- socks inbound с опциональной auth/password;
- `sniffing.destOverride`: `http`, `tls`, `quic`;
- routing:
  - `geoip:private -> direct`;
  - google-related домены форсируются в `proxy`;
  - при `bypassRu` добавляются RU domain rules и локальный ru-catalog domains;
  - default `tcp,udp -> proxy`.

См. `AndroidXrayConfigWriter.kt`.

## 7.4 Embedded runtime assets

ABI-aware loading:

- сначала пытается использовать `libnovpn_<bin>_exec.so` из `jniLibs`;
- fallback для старых API — установка из assets.

См.:

- `EmbeddedRuntimeManager.kt`;
- `EmbeddedRuntimeAssets.kt`;
- `app/build.gradle.kts` (`prepareEmbeddedRuntimeExecutables`).

## 7.5 RU-каталог и автодобавление приложений

Каталог собирается в assets из файлов репозитория:

- `ru app package.txt`;
- `ru site list.txt`.

Gradle task:

- `prepareRuExclusionCatalogAssets` в `client/android/app/build.gradle.kts`.

Matcher:

- exact packages;
- vendor prefixes;
- `ru.` package pattern.

Новый установившийся RU-пакет может автоматически добавляться в exclusions (`RuAppInstallReceiver`).

---

## 8. Desktop клиент: подробная работа

## 8.1 Основные блоки

- `MainWindow` — UI + действия пользователя;
- `ProfileStore` — импорт/загрузка профилей;
- `XrayConfigBuilder` и `ObfuscatorConfigBuilder` — генерация runtime JSON;
- `DesktopRuntimeManager` — lifecycle бинарников;
- `NetworkDiagnosticsRunner` — ping/download/upload через локальный SOCKS.

## 8.2 Что есть в текущей desktop реализации

- import profile JSON/YAML;
- activate code / promo;
- disconnect device;
- runtime preflight;
- runtime start/stop;
- diagnostics;
- app routing mode + app list;
- traffic/pattern strategy выбор;
- автоподбор RU-приложений (по установленным программам Windows + ключевые слова).

См.:

- `client/desktop/python/novpn_client/ui/main_window.py`;
- `client/desktop/python/novpn_client/app_catalog_service.py`.

## 8.3 Новые изменения по UX/доступности

В текущем состоянии добавлены:

- прокрутка в главном окне;
- прокрутка в окне настроек;
- поддержка колеса мыши;
- отдельный блок "Каталог RU-приложений" в настройках.

## 8.4 Runtime пути и хранение данных

Для запуска из установленного `exe`:

- runtime бинарники ищутся в `client/desktop/runtime/bin` рядом с приложением;
- generated state/logs пишутся в `%LOCALAPPDATA%\NoVPN Desktop\generated`.

См.:

- `client/desktop/python/novpn_client/app_paths.py`;
- `client/desktop/python/novpn_client/runtime_layout.py`.

## 8.5 Сборка и установщик

Скрипт:

- `client/desktop/python/build_windows.ps1`.

Шаги:

1. сборка PyInstaller;
2. формирование portable dist;
3. (опционально) сборка setup через Inno Setup (`ISCC.exe`) по `client/desktop/installer/novpn-desktop.iss`.

---

## 9. Obfuscator: как работает

## 9.1 Конфиг

Ключевые секции:

- `seed`;
- `listen` (локальный SOCKS endpoint);
- `upstream` (куда проксировать SOCKS connect);
- `session`:
  - `nonce`, `rotation_bucket`, `selected_fingerprint`, `selected_spider_x`, pools;
- `pattern_tuning`:
  - `padding_profile`, `jitter_window_ms`, bytes/ms диапазоны.

См. `cmd/obfuscator/config.go`.

## 9.2 Runtime поведение

1. слушает SOCKS5 на `listen`.
2. обрабатывает `CONNECT` и `UDP ASSOCIATE`.
3. при необходимости делает auth (`username/password`).
4. для `CONNECT`: устанавливает upstream SOCKS CONNECT.
5. для `UDP ASSOCIATE`: поднимает UDP relay path через upstream SOCKS.
6. запускает relay с `relayPlan`.

См. `cmd/obfuscator/runtime.go`.

## 9.3 Relay masking mechanics

`relayPlan` управляет:

- startup delay;
- inter-chunk delay;
- burst budget;
- idle pause;
- размером чанков (warm/steady window).

PRNG seed:

- `SHA256(seed | session.nonce | destination | rotation_bucket | direction | sessionID)`.

Это даёт стабильный, но детерминированно изменяемый паттерн в пределах rotation bucket.

## 9.4 Ограничения

- обфускация применяется к SOCKS TCP relay; UDP path ограничен семантикой SOCKS `UDP ASSOCIATE` и возможностями upstream;
- качество UDP зависит от доступности/поведения upstream SOCKS сервера.

---

## 10. "Скрытие": что это в проекте

Термин "скрытие" в проекте покрывает два слоя:

1. Скрытие/маскирование сетевого паттерна:
- strategy (`traffic_strategy`, `pattern_strategy`);
- session-based вариации fingerprint/spiderX/paths;
- chunk/jitter/padding управление.

2. Скрытие приложения (Android disguise):
- смена app label + applicationId на этапе сборки;
- генерация `rebuildCommand` в UI.

См.:

- `DisguiseIdentity.kt`;
- `client/android/app/build.gradle.kts` (`novpnAppId`, `novpnAppName`).

---

## 11. Как сделать кастомное "скрытие" (практический гайд)

## 11.1 Базовый уровень (без кода)

1. Выбрать traffic strategy:
- `balanced`, `cdn_mimic`, `fragmented`, `mobile_mix`, `tls_blend`.

2. Выбрать pattern strategy:
- `steady`, `pulse`, `randomized`, `burst_fade`, `quiet_sweep`.

3. Обновить серверный профиль и seed:
- через admin+profile sync;
- или импортом нового YAML/JSON.

## 11.2 Кастом session behavior (с кодом)

Редактировать planner:

- Android: `com/novpn/obfs/SessionObfuscationPlanner.kt`;
- desktop: `client/desktop/python/novpn_client/session_obfuscation.py`.

Точки кастомизации:

- `ROTATION_INTERVAL_MS`;
- fingerprint pool rules;
- cover path generation;
- ranges: padding/burst/idle/jitter.

Важно: Android и desktop лучше держать синхронными по логике planner, чтобы поведение было предсказуемым между платформами.

## 11.3 Кастом obfuscator runtime tuning

Редактировать:

- генераторы obfuscator config:
  - Android `ObfuscatorConfigWriter.kt`;
  - desktop `obfuscator_config_builder.py`;
- и/или сам runtime:
  - `cmd/obfuscator/runtime.go`.

Можно добавить:

- новые density профили;
- дополнительные режимы пауз;
- адаптивность к типу destination.

## 11.4 Кастом "дисгейз" APK

Вариант 1: из UI `SettingsActivity` сгенерировать команду.

Вариант 2: вручную:

```powershell
cd client/android
.\gradlew.bat :app:assembleDebug -PnovpnAppName="My Utility" -PnovpnAppId=com.example.myutility
```

Ограничения:

- `applicationId` должен быть валидным Android package id;
- после изменения нужен rebuild + reinstall APK.

## 11.5 Кастом REALITY fingerprints/spiderX

Можно менять на уровне профиля:

- `server.fingerprint`;
- `server.spider_x`.

Эти поля участвуют в planner и формировании `realitySettings` outbound на клиентах.

---

## 12. Диагностика и наблюдаемость

## 12.1 Клиентская диагностика

Android/desktop делают тесты через локальный SOCKS:

- latency;
- download;
- upload.

Server side endpoints:

- `/admin/diag/ping`;
- `/admin/diag/download`;
- `/admin/diag/upload`.

## 12.2 Server observability

- health/readiness/metrics для gateway;
- runtime stats в admin dashboard;
- traffic sync из Xray stats API.

---

## 13. Ограничения и риски текущей реализации

1. Desktop scaffold: без полноценного системного VPN/TUN path.
2. Invite/promo API в клиентах вызывается через HTTP.
3. Android bypass RU relies на локальном каталоге и доменных правилах; качество зависит от актуальности каталога.
4. Нужен внешний контур защиты админки (SSH tunnel/VPN/reverse proxy + TLS), если панель не только localhost.
5. При ослаблении security-конфига (`auth=noop`, `acl=allow_all`) gateway возвращается в PoC-режим и должен считаться небезопасным для публичной экспозиции.

---

## 14. Рекомендуемый production checklist

1. Включить реальную auth и ACL политику для gateway.
2. Защитить admin endpoint:
- закрытая сеть / SSH tunnel / reverse proxy + TLS.
3. Регулярно обновлять:
- `geoip.dat`;
- `geosite.dat`;
- локальный RU каталог.
4. Синхронизировать planner-логику Android/desktop.
5. Автоматизировать regression-tests obfuscator runtime (TCP+UDP correctness + perf budgets).
6. Для desktop production рассмотреть системный tunnel backend (Wintun/WFP) вместо только локального proxy orchestration.

---

## 15. Быстрые ссылки по коду

Сервер:

- bootstrap: `cmd/reality-bootstrap/main.go`
- reality provisioning: `internal/core/reality/provision.go`
- registry: `internal/core/reality/registry.go`
- traffic sync: `internal/core/reality/traffic.go`
- admin panel/API: `internal/server/admin.go`
- пример конфигурации: `deploy/config.example.yaml`

Obfuscator:

- cli: `cmd/obfuscator/main.go`
- config schema: `cmd/obfuscator/config.go`
- runtime: `cmd/obfuscator/runtime.go`

Android:

- service runtime: `client/android/app/src/main/java/com/novpn/vpn/NoVpnService.kt`
- xray config writer: `client/android/app/src/main/java/com/novpn/xray/AndroidXrayConfigWriter.kt`
- obfuscator config writer: `client/android/app/src/main/java/com/novpn/vpn/ObfuscatorConfigWriter.kt`
- session planner: `client/android/app/src/main/java/com/novpn/obfs/SessionObfuscationPlanner.kt`
- profile repository: `client/android/app/src/main/java/com/novpn/data/ProfileRepository.kt`
- disguise: `client/android/app/src/main/java/com/novpn/data/DisguiseIdentity.kt`

Desktop:

- entry: `client/desktop/python/novpn_client/app.py`
- paths: `client/desktop/python/novpn_client/app_paths.py`
- UI: `client/desktop/python/novpn_client/ui/main_window.py`
- xray builder: `client/desktop/python/novpn_client/config_builder.py`
- obfuscator builder: `client/desktop/python/novpn_client/obfuscator_config_builder.py`
- planner: `client/desktop/python/novpn_client/session_obfuscation.py`
- build: `client/desktop/python/build_windows.ps1`
- installer script: `client/desktop/installer/novpn-desktop.iss`
