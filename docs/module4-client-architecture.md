# Module 4: Client Integration

Цель: собрать desktop/mobile обёртку, которая управляет Xray, кастомным обфускатором, split tunneling и runtime-конфигом.

Текущий scaffold уже добавлен в [client/README.md](/d:/projekt/noVPN/client/README.md).

## Directory structure

```text
client/
  common/
    profiles/
      routing/
      reality/
      obfuscation/
    schema/
    telemetry/
  desktop/
    python/
      app.py
      ui/
        main_window.py
        settings_view_model.py
        split_tunnel_view_model.py
      services/
        app_catalog_service.py
        profile_store.py
        command_bus.py
    cpp/
      include/
        tunnel_controller.hpp
        xray_core_manager.hpp
        obfuscator_manager.hpp
        routing_config_builder.hpp
        split_tunnel_controller.hpp
        seed_coordinator.hpp
        wintun_adapter.hpp
        wfp_policy_store.hpp
      src/
        tunnel_controller.cpp
        xray_core_manager.cpp
        obfuscator_manager.cpp
        routing_config_builder.cpp
        split_tunnel_controller.cpp
        seed_coordinator.cpp
        wintun_adapter.cpp
        wfp_policy_store.cpp
      third_party/
        xray/
        obfuscator/
        wintun/
  android/
    app/
      src/main/java/com/novpn/
        ui/
        vpn/
        xray/
        obfs/
        split/
        data/
      src/main/res/
```

## Desktop architecture

Разделение ответственности:

- Python слой отвечает за UI, каталог приложений, настройки и orchestration команд.
- C++ слой отвечает за Wintun, WFP, lifecycle бинарников и low-level IPC.
- Xray и обфускатор запускаются как отдельные процессы под контролем desktop backend.

Ключевые интерфейсы desktop backend:

```cpp
struct TunnelRuntimePlan {
    bool bypassRu = false;
    std::vector<std::wstring> excludedExePaths;
    std::wstring xrayConfigPath;
    std::wstring obfuscatorConfigPath;
};

class ITunnelController {
  public:
    virtual ~ITunnelController() = default;
    virtual void Start(const TunnelRuntimePlan& plan) = 0;
    virtual void Stop() = 0;
    virtual void Reconfigure(const TunnelRuntimePlan& plan) = 0;
};

class IXrayCoreManager {
  public:
    virtual ~IXrayCoreManager() = default;
    virtual void WriteConfig(const std::string& json) = 0;
    virtual void Start() = 0;
    virtual void Restart() = 0;
    virtual void Stop() = 0;
};

class IObfuscatorManager {
  public:
    virtual ~IObfuscatorManager() = default;
    virtual void ConfigureSeed(const std::string& seed) = 0;
    virtual void Start() = 0;
    virtual void Stop() = 0;
};

class ISplitTunnelController {
  public:
    virtual ~ISplitTunnelController() = default;
    virtual void ApplyExcludedApps(const std::vector<std::wstring>& exePaths) = 0;
    virtual void Clear() = 0;
};

class ISeedCoordinator {
  public:
    virtual ~ISeedCoordinator() = default;
    virtual std::string LoadOrCreateSeed() = 0;
    virtual void SyncToXrayAndObfuscator() = 0;
};
```

Роль основных классов:

- `TunnelController`: верхний orchestration слой, стартует и останавливает runtime.
- `RoutingConfigBuilder`: собирает `config.json` для локального Xray на основе UI флагов.
- `SplitTunnelController`: применяет `.exe` exclusions через WFP/Wintun policy.
- `SeedCoordinator`: гарантирует одинаковый seed для клиента и серверной логики obfuscation.
- `XrayCoreManager`: следит за локальным Xray binary, логами и hot-reload.
- `ObfuscatorManager`: управляет кастомным binary из Модуля 1.

UI-флоу desktop:

1. Пользователь включает чекбокс `Не проксировать РФ`.
2. `SettingsViewModel` формирует новый `TunnelRuntimePlan`.
3. `RoutingConfigBuilder` добавляет или убирает RU rules.
4. `SplitTunnelController` обновляет список исключённых приложений.
5. `XrayCoreManager.Restart()` или hot-reload применяет новый config.

## Android architecture

Ключевые классы:

```kotlin
interface RoutingConfigWriter {
    fun writeConfig(bypassRu: Boolean, excludedPackages: List<String>): String
}

interface EmbeddedXrayRunner {
    fun start(configPath: String)
    fun restart(configPath: String)
    fun stop()
}

interface ObfuscatorBridge {
    fun applySeed(seed: String)
    fun start()
    fun stop()
}

interface SplitTunnelRepository {
    fun loadExcludedPackages(): List<String>
    fun saveExcludedPackages(value: List<String>)
}
```

Рекомендуемые concrete classes:

- `NoVpnService`: наследник `VpnService`, создаёт TUN и применяет `addDisallowedApplication`.
- `AndroidRoutingConfigWriter`: пишет локальный Xray JSON в sandbox.
- `AndroidXrayRunner`: запускает embedded Xray binary через `ProcessBuilder`.
- `AndroidObfuscatorBridge`: настраивает seed и lifecycle обфускатора.
- `InstalledAppsScanner`: собирает список APK для UI выбора.
- `TunnelViewModel`: соединяет UI и VPN runtime.

## Runtime pipeline

Общий pipeline для desktop и Android:

1. Загрузить профиль Reality/VLESS.
2. Загрузить seed обфускации.
3. Сформировать routing rules:
   `bypassRu` -> добавить `ext:geosite.dat:ru` и `ext:geoip.dat:ru`
   `excludedApps` -> добавить process/package exclusions
4. Пересобрать локальный `config.json`.
5. Перезапустить Xray.
6. Перезапустить или перенастроить обфускатор тем же seed.
7. Обновить туннельный backend.

## Packaging requirements

В клиентский пакет должны входить:

- `xray` binary;
- `geoip.dat` и `geosite.dat`;
- binary кастомного обфускатора;
- базовый Reality/VLESS client profile;
- runtime writable каталог под generated `config.json`, logs и cache.

## Operational note

Я трактую список приложений как exclusion list: выбранные в UI приложения идут `direct`, остальные по умолчанию идут в `proxy`. Если для Windows backend окажется надёжнее allowed-app model, UI-список исключений просто инвертируется в runtime перед программированием WFP/Wintun.
