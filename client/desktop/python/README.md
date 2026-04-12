# Desktop Client

ПК-клиент теперь включает:

- активацию кода / промокода и отвязку устройства;
- импорт профиля (JSON/YAML), выбор серверов и предпросмотр конфига;
- встроенный runtime (`xray.exe` + `obfuscator.exe`) с диагностикой;
- live-настройки маршрутизации/маскировки;
- автоподбор RU-приложений в настройках (аналог Android-каталога);
- прокрутку в главном окне и окне настроек;
- headless-режим для автоматизации.

## Локальный запуск из репозитория

```powershell
python client/desktop/python/app.py
```

Headless-пример:

```powershell
python client/desktop/python/app.py --headless --bypass-ru --start-runtime
```

## Сборка Windows `.exe`

Требования:

- Windows x64;
- Python 3.10+ в `PATH`;
- (опционально) Inno Setup 6+ в `PATH` для сборки `setup.exe`.

Команда сборки:

```powershell
cd client/desktop/python
.\build_windows.ps1 -Version 0.1.0
```

Результат:

- portable-сборка: `client/desktop/build/dist/NoVPN Desktop/NoVPN Desktop.exe`
- установщик (если найден `ISCC.exe`): `client/desktop/build/installer/NoVPN-Desktop-Setup-<version>.exe`

Если нужен только portable `.exe` без установщика:

```powershell
.\build_windows.ps1 -SkipInstaller
```

## Runtime-ассеты

В build включаются файлы из:

```text
client/desktop/runtime/bin/xray.exe
client/desktop/runtime/bin/obfuscator.exe
client/desktop/runtime/bin/geoip.dat
client/desktop/runtime/bin/geosite.dat
```

Во время установки пользовательские данные и логи пишутся в `%LOCALAPPDATA%\NoVPN Desktop\generated`, поэтому приложение корректно работает из `Program Files`.
