# Android Scaffold

Начальный Android-каркас находится в этой директории.

Что уже есть:

- `VpnService` с `addDisallowedApplication(...)`;
- `ViewModel` для флага `Не проксировать РФ`;
- генерация локального `filesDir/xray/config.json`;
- загрузка базового Reality-профиля из `assets/profile.default.json`;
- сканер установленных APK для будущего UI split tunneling.

Следующий инженерный шаг после scaffold:

- подключить embedded Xray binary;
- добавить реальный экран выбора APK;
- связать `TunnelViewModel` и `NoVpnService` через foreground-service flow.
