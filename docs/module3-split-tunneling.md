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

Для Android package-level исключения не дублируются в JSON и применяются отдельно через `VpnService.Builder.addAllowedApplication(...)` / `addDisallowedApplication(...)`.

Важно: на Android это первый уровень маршрутизации. Сначала `VpnService` решает, попадёт ли трафик конкретного приложения в TUN вообще, и только потом Xray видит этот поток и применяет доменные/IP-правила. Если приложение не попало в TUN, правила из JSON для его соединений вообще не участвуют.

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
- в режиме `EXCLUDE_SELECTED` вызывает `addDisallowedApplication(packageName)` и эти приложения идут мимо VPN;
- в режиме `ONLY_SELECTED` вызывает `addAllowedApplication(packageName)` и только эти приложения попадают в VPN;
- затем создаёт VPN interface через `establish()`.

Это соответствует официальной модели Android: сначала происходит package-level отбор приложений, а уже потом для трафика внутри VPN применяются Xray rules.

## 4. Как реально идёт трафик на Android

### 4.1 Шаг 1: отбор приложений на уровне `VpnService`

Точка принятия первого решения:

- `ONLY_SELECTED` = белый список. В TUN попадают только выбранные пакеты.
- `EXCLUDE_SELECTED` = список исключений. Выбранные пакеты не попадают в TUN, остальные попадают.

Следствие: сайтовые правила сами по себе не могут "затащить" приложение в VPN. Если приложение не прошло package-level отбор, его трафик идёт напрямую через системную сеть и Xray его не видит.

### 4.2 Шаг 2: маршрутизация уже внутри VPN

Если приложение попало в TUN, дальше путь такой:

`App -> VpnService(TUN) -> tun2proxy -> local obfuscator SOCKS (или fallback local Xray SOCKS) -> Xray routing -> direct|proxy`

Здесь `direct` означает локальный выход с устройства через обычную сеть Android, без отправки этого соединения в удалённый VLESS/REALITY туннель. `proxy` означает отправку через Xray outbound `proxy`, дальше в VLESS/REALITY и на сервер.

Актуальный порядок правил в Android-конфиге:

1. `geoip:private` в `direct`
2. `bittorrent` в `direct`
3. `geosite:google` и дополнительные Google/YouTube домены в `proxy`
4. при `bypassRu=true`: `.ru`, `.su`, `.xn--p1ai` в `direct`
5. при `bypassRu=true`: домены из локального каталога `ru site list.txt` в `direct`
6. при `bypassRu=true`: `geoip:ru` в `direct`
7. default `tcp,udp` в `proxy`

### 4.3 Что происходит с приложением из белого списка

Когда включён default whitelist mode, клиент переводит routing в `ONLY_SELECTED`. Для приложения из белого списка это значит:

- процесс попадает в TUN;
- дальше каждое соединение внутри этого приложения оценивается отдельно правилами Xray;
- один и тот же браузер из белого списка может отправить `youtube.com` в `proxy`, `gosuslugi.ru` в `direct`, а `example.org` снова в `proxy`.

То есть белый список отвечает только на вопрос "зайдёт ли приложение в VPN", а не "все ли сайты из этого приложения обязательно пойдут через сервер".

### 4.4 Что происходит с приложением не из белого списка

В режиме `ONLY_SELECTED` приложение вне белого списка:

- не попадает в TUN;
- не проходит через `tun2proxy`, obfuscator и Xray;
- открывает любые сайты напрямую через системную сеть Android;
- не использует ни `ru site list.txt`, ни `geoip:ru`, ни другие Xray routing rules.

Это самый частый источник путаницы: доменные правила работают только для приложений, которые уже были допущены в VPN.

### 4.5 Что происходит с сайтами

Для сайтов итог зависит от двух уровней сразу:

- если браузер или приложение не попали в VPN, любой сайт идёт напрямую;
- если браузер или приложение попали в VPN, Xray пытается распознать домен через sniffing (`http`, `tls`, `quic`);
- если домен распознан и совпал с Google/YouTube правилами, соединение остаётся в `proxy`;
- если включён `bypassRu` и домен попал в RU suffix rules или в локальный каталог сайтов, соединение идёт в `direct`;
- если домен не распознан, `domainStrategy: "IPIfNonMatch"` даёт повторную проверку по IP, и RU IP-адреса тоже идут в `direct`;
- всё остальное идёт через `proxy`.

## 5. Recommended rule order

На Android package-level фильтрация приложений должна происходить до Xray routing. Внутри самого Xray порядок правил должен оставаться таким, как описано выше: сначала private/special-case direct, затем явные proxy-исключения для Google/YouTube, затем RU bypass, затем catch-all `proxy`.

## 6. Sources

- Xray Routing: https://xtls.github.io/en/config/routing
- Xray asset directory: https://xtls.github.io/en/config/features/env.html
- Android `VpnService.Builder.addDisallowedApplication`: https://developer.android.com/reference/android/net/VpnService.Builder.html
- Android VPN guide: https://developer.android.com/develop/connectivity/vpn
- WFP `FwpmGetAppIdFromFileName0`: https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmgetappidfromfilename0
- WFP filtering layers: https://learn.microsoft.com/en-us/windows/win32/fwp/management-filtering-layer-identifiers-
- WFP filter insertion: https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmfilteradd0
