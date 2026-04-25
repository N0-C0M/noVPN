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
- Android содержит полноценный TUN-путь (`VpnService + tun2proxy`), а desktop-клиент поддерживает и локальный runtime-режим с SOCKS/HTTP, и начальный Windows system-tunnel режим на базе Xray TUN + `wintun.dll`.

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

Из-за этого Android manifest пока вынужден держать `android:usesCleartextTraffic="true"`.
После перевода bootstrap/admin API на HTTPS-only этот флаг нужно переключить в `false`.
`android:allowBackup` в Android-клиенте должен оставаться выключенным (`false`).

---

## 4. Передача данных: цепочки трафика

## 4.1 Android datapath

Предпочтительный путь:

`App traffic -> VpnService(TUN) -> tun2proxy -> local obfuscator SOCKS -> local Xray SOCKS -> VLESS/REALITY -> server`.

Session-wide fallback path:

`TUN -> tun2proxy -> local Xray SOCKS (UDP enabled) -> VLESS/REALITY`.

Причина: при старте сервиса выполняется `UDP ASSOCIATE` probe к локальному obfuscator bridge. Если probe проходит, используется цепочка `obfuscator -> Xray`; если нет, вся VPN-сессия переводится на direct local Xray bridge. Это больше не YouTube-specific эвристика.

См.:

- `NoVpnService.kt`;
- `Tun2ProxyBridge.kt`;
- `EmbeddedRuntimeManager.kt`.

## 4.2 Desktop datapath

Desktop runtime:

- поддерживает два режима: local runtime и Windows system-tunnel;
- в local runtime запускает локальный Xray (SOCKS+HTTP inbound);
- запускает локальный obfuscator;
- пишет конфиги и логи;
- даёт управление режимами маршрутизации и маскировки.

Windows system-tunnel path в текущем Python scaffold:

- использует Xray `tun` inbound и `wintun.dll`;
- требует запуск клиента с правами Administrator;
- делает временное IPv4 route switching для текущей сессии;
- сохраняет локальные SOCKS/HTTP inbounds для диагностики и явного proxy-use;
- пока не включает упакованный WFP helper по умолчанию.

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
- `client/android/app/src/main/secure/bootstrap.json`.

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

### 7.2.1 Где именно принимается решение

В Android-клиенте решение принимается в два этапа:

1. `VpnService.Builder` решает, попадёт ли трафик процесса в TUN вообще.
2. Если поток попал в TUN, `tun2proxy` передаёт его в локальный bridge (`obfuscator SOCKS`, либо fallback напрямую в локальный Xray SOCKS), после чего Xray выбирает `direct` или `proxy`.

Это ключевая граница: доменные и IP-правила Xray не видят трафик приложений, которые были отфильтрованы на этапе `addAllowedApplication` / `addDisallowedApplication`.

### 7.2.2 Что означает белый список по умолчанию

Опция default whitelist mode в Android-настройках фактически переводит routing в `ONLY_SELECTED`. Effective список пакетов при этом собирается как сохранённые пользователем пакеты плюс `defaultWhitelistPackages()` из `ClientPreferences.kt` (YouTube family, браузеры, Telegram, Instagram, X, Roblox, Discord, часть игр Supercell, MEGA, ChatGPT, Gemini).

То есть белый список отвечает только на первый вопрос: "должно ли приложение вообще зайти в VPN".

### 7.2.3 Путь трафика для приложения из белого списка

Если приложение входит в белый список (`ONLY_SELECTED`), его трафик идёт так:

`App -> VpnService(TUN) -> tun2proxy -> local obfuscator SOCKS (или fallback local Xray SOCKS) -> Xray routing -> direct|proxy`

Дальше уже Xray принимает решение для каждого отдельного соединения:

- если домен относится к Google/YouTube family, соединение принудительно остаётся в `proxy`;
- если включён `bypassRu` и домен совпал с `.ru`, `.su`, `.xn--p1ai` или с локальным каталогом доменов из `ru site list.txt`, соединение уходит в `direct`;
- если домен не распознан по sniffing, включается `domainStrategy: "IPIfNonMatch"` и Xray повторно проверяет соединение по IP; RU-IP тоже идут в `direct`;
- всё остальное идёт в `proxy`, затем в VLESS/REALITY и на сервер.

Здесь `direct` означает локальный выход с телефона через обычную сеть Android, без отправки этого конкретного соединения в удалённый VPN-сервер.
Выбор между `local obfuscator SOCKS` и `local Xray SOCKS` делается один раз на старте VPN-сессии через `UDP ASSOCIATE` probe и затем действует для всех приложений внутри этой сессии.

### 7.2.4 Путь трафика для приложения не из белого списка

Если приложение не входит в белый список и активен режим `ONLY_SELECTED`, его трафик:

- вообще не попадает в TUN;
- не проходит через `tun2proxy`, obfuscator и Xray;
- открывает любые сайты напрямую через системную сеть Android;
- не использует `ru site list.txt`, `geoip:ru` и другие Xray routing rules.

Это важный момент: сайты не могут "переопределить" отсутствие приложения в белом списке. Сначала приложение должно попасть в VPN, и только потом начинают работать доменные правила.

### 7.2.5 Что происходит с сайтами внутри браузера или приложения

Сайтовая маршрутизация работает только для приложений, уже попавших в VPN.

Практически это означает:

- браузер из белого списка может отправлять разные вкладки разными путями;
- `youtube.com`, `googlevideo.com`, `ytimg.com`, `gstatic.com` и связанные домены остаются в `proxy`;
- `gosuslugi.ru`, `ya.ru` или любой домен из локального RU-каталога уйдут в `direct`, если `bypassRu=true`;
- если браузер не входит в белый список, вообще любой сайт из него откроется напрямую и Xray не будет участвовать.

### 7.2.6 Инвертированный режим `EXCLUDE_SELECTED`

Режим `EXCLUDE_SELECTED` работает зеркально:

- выбранные приложения полностью обходят VPN;
- все остальные приложения попадают в TUN;
- для этих остальных приложений сайты дальше маршрутизируются теми же Xray правилами (`google -> proxy`, `RU domains/IP -> direct`, всё остальное -> `proxy`).

## 7.3 Конфиг Xray на Android

Особенности:

- socks inbound с опциональной auth/password;
- `sniffing.destOverride`: `http`, `tls`, `quic`, поэтому Xray может распознавать `Host`, TLS SNI и QUIC destination для доменных правил;
- routing:
  - `geoip:private -> direct`;
  - `bittorrent -> direct`;
  - google-related домены форсируются в `proxy` раньше RU bypass правил;
  - при `bypassRu` добавляются `.ru`, `.su`, `.xn--p1ai` suffix rules в `direct`;
  - при `bypassRu` добавляются домены из локального ru-catalog (`ru site list.txt`) в `direct`;
  - при `bypassRu` добавляется `geoip:ru -> direct`;
  - default `tcp,udp -> proxy`.

Важно: app-level routing для Android не живёт в Xray JSON. Он применяется отдельно в `NoVpnService.applyApplicationRouting()`, а Xray получает только тот трафик, который уже был допущен в TUN.

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

## 11.6 Сценарии шифровки/маскировки (по практике)

Ниже описаны рабочие сценарии для текущих полей `traffic_strategy` и `pattern_strategy`.
По коду значения заданы в `RuntimeSelections.kt`, а фактические диапазоны паддинга/джиттера/пауз — в `SessionObfuscationPlanner.kt`.

1. `BALANCED + STEADY` (базовый production сценарий)
- Динамическая ротация fingerprint/spiderX без агрессивных всплесков.
- Низкий jitter, умеренный padding, ровный фон.
- Рекомендован как дефолт для большинства сетей.

2. `CDN_MIMIC + PULSE` (под браузерный/CDN профиль)
- Упор на CDN-подобные cover path и browser-like fingerprint pool.
- Более выраженная пульсация трафика и средний burst-профиль.
- Подходит для активного web-трафика и стриминга.

3. `FRAGMENTED + RANDOMIZED` (максимум вариативности)
- Широкие диапазоны padding/idle, заметно неровный ритм.
- Полезно в сетях с жесткой эвристикой по паттернам потока.
- Цена: выше overhead и потенциально выше latency.

4. `MOBILE_MIX + BURST_FADE` (мобильные/нестабильные сети)
- Агрессивные burst-интервалы, быстрые смены ритма.
- Хорошо маскирует короткие и нестабильные мобильные сессии.
- Цена: возможный рост расхода трафика.

5. `TLS_BLEND + QUIET_SWEEP` (тихий фон)
- Консервативный и менее шумный профиль.
- Подходит для стабильных каналов и фоновой работы.

## 11.7 Как задавать кастомную шифровку

В проекте настройка идет на двух уровнях:

1. Крипто-параметры transport (VLESS/REALITY)
- Поля профиля: `server_name`, `public_key`, `short_id`, `flow`, `fingerprint`, `spider_x`.
- Они попадают в `realitySettings` Xray outbound и определяют крипто/handshake параметры.

2. Поведенческая маскировка (obfuscator/planner)
- Поля: `obfuscation.seed`, `traffic_strategy`, `pattern_strategy`.
- Они задают session nonce, выбор fingerprint/spiderX и `pattern_tuning` (padding/jitter/burst/idle).

Пример кастомного фрагмента профиля:

```json
{
  "server": {
    "server_name": "cdn.example.com",
    "public_key": "BASE64_X25519_PUBLIC_KEY",
    "short_id": "a1b2c3d4",
    "flow": "xtls-rprx-vision",
    "fingerprint": "chrome",
    "spider_x": "/cdn-cgi/trace"
  },
  "obfuscation": {
    "seed": "team-alpha-2026-q2",
    "traffic_strategy": "cdn_mimic",
    "pattern_strategy": "pulse"
  }
}
```

Практический порядок настройки:

1. Изменить серверный профиль (`client_profile.yaml` или imported JSON/YAML).
2. Импортировать профиль на Android/desktop.
3. Выбрать нужные стратегии в UI или оставить значения из профиля.
4. Перегенерировать runtime-конфиги:
- Xray (`AndroidXrayConfigWriter` / desktop builder).
- Obfuscator (`ObfuscatorConfigWriter` / desktop builder).

Если нужна полностью новая схема:

1. Добавить новый вариант в `TrafficObfuscationStrategy` и/или `PatternMaskingStrategy`.
2. Расширить `SessionObfuscationPlanner` (pool/range/rotation).
3. Синхронно обновить Android и desktop planners.

Важно: `seed` желательно ротировать по релизным циклам, чтобы менять детерминированный профиль сессий.

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
3. Держать публичный VPN edge отдельно от control-plane сервисов.
4. На публичных redeem/handshake поверхностях делать silent fail для заведомо невалидных probing-попыток и включать rate-limit по повторяющимся ошибочным handshake.
5. Для IP reputation держать пул чистых IP и быстрый failover/rotation; это не решается одной криптографией.
6. На Android держать `android:allowBackup="false"` и выключить `android:usesCleartextTraffic`, как только bootstrap/admin client flow уйдёт с HTTP.
7. Регулярно обновлять:
- `geoip.dat`;
- `geosite.dat`;
- локальный RU каталог.
8. Развивать уже существующую поведенческую маскировку: per-session seed, `traffic_strategy`,
   `pattern_strategy`, padding/jitter и relay shaping, а также синхронизировать planner-логику Android/desktop.
9. Автоматизировать regression-tests obfuscator runtime (TCP+UDP correctness + perf budgets).
10. Для desktop production рассмотреть системный tunnel backend (Wintun/WFP) вместо только локального proxy orchestration.

Текущая база для этого уже есть в Android `client/android/app/src/main/java/com/novpn/obfs/SessionObfuscationPlanner.kt`,
`client/android/app/src/main/java/com/novpn/vpn/ObfuscatorConfigWriter.kt`, desktop
`client/desktop/python/novpn_client/session_obfuscation.py` и runtime `cmd/obfuscator/runtime.go`.

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
