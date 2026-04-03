# Module 3: Split Tunneling

Цель: исключить RU-сегмент и выбранные приложения из VPN-туннеля, оставив остальной трафик на `proxy`.

## 1. Xray routing

Готовый JSON-шаблон лежит в [examples/xray/client-routing.split.json](/d:/projekt/noVPN/examples/xray/client-routing.split.json).

Ключевые моменты:

- `domainStrategy: "IPIfNonMatch"` нужен, чтобы после доменного матчинга Xray делал повторный проход по IP-правилам.
- `domain: ["ext:geosite.dat:ru"]` отправляет домены RU-сегмента в `direct`.
- `ip: ["ext:geoip.dat:ru"]` отправляет RU-IP в `direct`.
- `process: [...]` даёт desktop app-level исключения там, где локальное ядро Xray видит имя или путь процесса.
- финальное правило отправляет весь прочий `tcp,udp` трафик в `proxy`.

Для Android package-level исключения не дублируются в JSON и применяются отдельно через `VpnService.Builder.addDisallowedApplication(...)`.

Важно: `geosite.dat` и `geoip.dat` должны лежать в asset directory Xray. По официальной документации Xray ищет их рядом с бинарником, либо в каталоге из `XRAY_LOCATION_ASSET`.

## 2. Windows desktop: WFP + Wintun

Пример management-side кода лежит в [examples/windows/wfp/split_tunnel_wfp.cpp](/d:/projekt/noVPN/examples/windows/wfp/split_tunnel_wfp.cpp).

Что делает пример:

- открывает Base Filtering Engine через `FwpmEngineOpen0`;
- создаёт собственного provider и sublayer;
- для каждого `.exe` получает `appId` через `FwpmGetAppIdFromFileName0`;
- ставит app-scoped filters на `FWPM_LAYER_ALE_AUTH_CONNECT_V4/V6`.

Практическая модель для desktop:

- если UI хранит список исключений, эти `.exe` программируются как `direct`;
- если реализация клиента использует allowed-app model для Wintun, список исключений просто инвертируется перед установкой правил;
- обновление списка приложений должно быть атомарным: удалить старые filters, затем поставить новый набор внутри одной транзакции WFP.

Замечание по архитектуре: сам по себе WFP management API не заменяет policy-логику клиента. Он задаёт процессно-специфичные фильтры, а решение "вести поток в Wintun или оставить на системном NIC" принимает ваш desktop backend.

## 3. Android: `VpnService.Builder`

Готовый Kotlin пример лежит в [examples/android/SplitTunnelVpnService.kt](/d:/projekt/noVPN/examples/android/SplitTunnelVpnService.kt).

Логика:

- сервис принимает список `packageName`;
- проверяет, что пакет установлен;
- для каждого пакета вызывает `addDisallowedApplication(packageName)`;
- затем создаёт VPN interface через `establish()`.

Это соответствует официальной модели Android: disallowed-приложения используют системную сеть, а остальные идут через VPN.

## 4. Recommended rule order

Порядок правил должен быть именно таким:

1. `geoip:private` в `direct`
2. `ext:geosite.dat:ru` в `direct`
3. `ext:geoip.dat:ru` в `direct`
4. app/process exclusions в `direct`
5. default catch-all в `proxy`

Такой порядок уменьшает лишние DNS/IP lookup и делает поведение предсказуемым.

## Sources

- Xray Routing: https://xtls.github.io/en/config/routing
- Xray asset directory: https://xtls.github.io/en/config/features/env.html
- Android `VpnService.Builder.addDisallowedApplication`: https://developer.android.com/reference/android/net/VpnService.Builder.html
- Android VPN guide: https://developer.android.com/develop/connectivity/vpn
- WFP `FwpmGetAppIdFromFileName0`: https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmgetappidfromfilename0
- WFP filtering layers: https://learn.microsoft.com/en-us/windows/win32/fwp/management-filtering-layer-identifiers-
- WFP filter insertion: https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmfilteradd0
