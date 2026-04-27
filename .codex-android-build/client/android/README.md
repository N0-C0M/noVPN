# Android Scaffold

This Android scaffold now includes:

- a `VpnService`-based foreground runtime;
- config generation for embedded Xray;
- obfuscator sidecar config generation;
- ABI-aware embedded binary installation from `assets/bin/`;
- runtime preflight diagnostics for profile completeness and embedded assets;
- protected local SOCKS bridges with per-launch credentials and random loopback host/port;
- start/stop controls in `MainActivity`;
- application routing through `addAllowedApplication(...)` / `addDisallowedApplication(...)`.

Expected asset layout:

```text
app/src/main/assets/bin/geoip.dat
app/src/main/assets/bin/geosite.dat
app/src/main/assets/bin/arm64-v8a/xray
app/src/main/assets/bin/arm64-v8a/obfuscator
app/src/main/assets/bin/armeabi-v7a/xray
app/src/main/assets/bin/armeabi-v7a/obfuscator
app/src/main/assets/bin/x86_64/xray
app/src/main/assets/bin/x86_64/obfuscator
app/src/main/assets/bin/x86/xray
app/src/main/assets/bin/x86/obfuscator
```

The runtime picks binaries from `Build.SUPPORTED_ABIS` order and falls back to the legacy flat
layout (`bin/xray`, `bin/obfuscator`) if needed.

Current launcher assumptions:

- Xray CLI: `xray run -config <path>`
- obfuscator CLI: `obfuscator --config <path>`

Profile workflow:

1. Generate the server profile on the VPS:

   ```bash
   sudo cat /var/lib/novpn/reality/client-profile.yaml
   ```

2. Update the bundled desktop profile and Android bootstrap asset in this repo:

   ```bash
   go run ./cmd/client-profile-sync \
     -input /path/to/client-profile.yaml \
     -bootstrap-address 2.26.85.47 \
     -bootstrap-api-base http://2.26.85.47/admin
   ```

   This rewrites:

   - `client/common/profiles/reality/default.profile.json`
   - `client/android/app/src/main/secure/bootstrap.json`

3. Build the APK and distribute activation codes from the admin control plane. The mobile client is
   now centered around end-user flows: select a server, enter a code, run the quick test, and tap
   connect. Direct profile import is no longer the main path in the home screen UI.

Build workflow:

1. Build locally with the checked-in Gradle wrapper:

   ```bash
   cd client/android
   ./gradlew assembleDebug
   ```

   On Windows:

   ```powershell
   cd client/android
   .\gradlew.bat assembleDebug
   ```

2. GitHub Actions now builds the debug APK automatically from
   `.github/workflows/android-apk.yml` and uploads it as an artifact.

Manifest security flags:

- `android:allowBackup` is disabled (`false`) in `AndroidManifest.xml`.
- `android:usesCleartextTraffic` remains enabled for now because bootstrap and admin-facing client
  flows still accept `http://.../admin` (`bootstrap.json`, `InviteRedeemer`,
  `GatewayPolicyService`, `NetworkDiagnosticsRunner`).
- When bootstrap and client control-plane calls move to HTTPS-only, flip
  `android:usesCleartextTraffic` to `false` and update the bootstrap/profile examples accordingly.

Current runtime behavior:

- the preferred Android datapath is `TUN -> tun2proxy -> local obfuscator SOCKS -> local Xray
  SOCKS -> VLESS/REALITY`;
- the service creates both local SOCKS bridges with UDP enabled and probes `UDP ASSOCIATE`
  support on the local obfuscator bridge during startup;
- if that UDP probe passes, the obfuscator bridge is used for the whole VPN session;
- if the UDP probe fails, the whole VPN session falls back to `TUN -> tun2proxy -> local Xray
  SOCKS (UDP enabled) -> VLESS/REALITY`;
- this fallback is session-wide and is not limited to YouTube or any other single package;
- local loopback SOCKS remains a hardening boundary, not an absolute one. Current defenses are
  per-launch credentials plus random loopback host/port.

Runtime preflight:

- the home screen now surfaces a preflight panel for:
  - real imported profile validation;
  - ABI-compatible embedded `xray` and `obfuscator` binaries;
  - required `geoip.dat` and `geosite.dat` assets.
- config generation and service startup both refuse to proceed when preflight is not ready.
- the preflight panel reflects actual runtime prerequisites only; UDP bridge selection happens
  later at service startup through capability probing.
