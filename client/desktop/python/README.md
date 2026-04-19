# Desktop Client

The desktop client now includes:

- bundled default profile support, so the app can start with a built-in profile before any manual import;
- multi-server profile handling for imported JSON/YAML files and invite responses that return several server profiles;
- desktop-side logging (`desktop-client.log`) plus runtime logs for `xray.exe` and `obfuscator.exe`;
- activation and disconnect flows that honor per-profile `api_base` values;
- scrollable main and settings windows with mouse-wheel support;
- headless config generation and embedded runtime startup;
- a Windows system-tunnel mode based on Xray TUN + `wintun.dll`, with admin-gated route switching;
- an initial WFP helper source for future app-scoped filter work.

## Local Run

```powershell
python client/desktop/python/app.py
```

Headless example:

```powershell
python client/desktop/python/app.py --headless --bypass-ru --start-runtime
```

Headless system-tunnel example:

```powershell
python client/desktop/python/app.py --headless --system-tunnel --start-runtime
```

Notes:

- system-tunnel mode currently requires launching the desktop client as Administrator;
- the runtime still keeps local SOCKS/HTTP inbounds, so diagnostics and local proxy usage continue to work;
- route switching is currently IPv4-only.

## Logs And State

Repository mode writes generated files to:

```text
client/desktop/python/generated/
```

Important paths:

- client state: `client/desktop/python/generated/client_state.json`
- imported profiles: `client/desktop/python/generated/profiles/`
- desktop app log: `client/desktop/python/generated/logs/desktop-client.log`
- runtime logs: `client/desktop/python/generated/runtime/logs/xray.log` and `client/desktop/python/generated/runtime/logs/obfuscator.log`

Packaged Windows builds write the same data under:

```text
%LOCALAPPDATA%\NoVPN Desktop\generated
```

## Multi-Server Behavior

- The bundled profile is always available as a fallback entry.
- Invite activation can import more than one profile from the control plane.
- Imported files keep unique names even when several servers share the same display name.
- Desktop now preserves `server_id` and `api_base`, so activation and disconnect requests can target the correct control-plane endpoint per server.

## Windows Build

Requirements:

- Windows x64
- Python 3.10+ in `PATH`
- optional: Inno Setup 6+ in `PATH` or under `.tools/InnoSetup/` for `setup.exe`

Build command:

```powershell
cd client/desktop/python
.\build_windows.ps1 -Version 0.1.0
```

Output:

- portable build: `client/desktop/build/dist/NoVPN Desktop/NoVPN Desktop.exe`
- installer, when `ISCC.exe` is available: `client/desktop/build/installer/NoVPN-Desktop-Setup-<version>.exe`

Portable-only build:

```powershell
.\build_windows.ps1 -SkipInstaller
```

The build script now also detects a repo-local Inno Setup compiler at:

```text
.tools/InnoSetup/ISCC.exe
```

The build script now accepts `bootstrap.json` from either:

```text
client/android/app/src/main/secure/bootstrap.json
client/android/app/src/main/assets/bootstrap.json
```

## Runtime Assets

The Windows build packages files from:

```text
client/desktop/runtime/bin/xray.exe
client/desktop/runtime/bin/obfuscator.exe
client/desktop/runtime/bin/geoip.dat
client/desktop/runtime/bin/geosite.dat
```

## Current Scope

The desktop client manages local runtime processes and local proxy endpoints.
It now has an initial Windows system-tunnel path, but it is still not at Android parity yet.

Current limits:

- the Windows system tunnel requires Administrator rights;
- route switching is IPv4-only for now;
- the WFP helper is source-only scaffolding at `client/desktop/windows/wfp/novpn_wfp_helper.cpp` and is not yet packaged into the desktop build by default.
