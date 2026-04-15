# Android Scaffold

This Android scaffold now includes:

- a `VpnService`-based foreground runtime;
- config generation for embedded Xray;
- obfuscator sidecar config generation;
- ABI-aware embedded binary installation from `assets/bin/`;
- runtime preflight diagnostics for profile completeness and embedded assets;
- hardening for the local Xray SOCKS bridge: per-launch password auth, random loopback port,
  and UDP disabled;
- start/stop controls in `MainActivity`;
- package exclusions through `addDisallowedApplication(...)`.

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

Current limitation:

- the runtime already uses an Android packet path (`TUN -> tun2proxy -> local obfuscator SOCKS ->
  local Xray SOCKS -> VLESS/REALITY`), so the basic datapath is live.
- the obfuscator sidecar currently handles SOCKS `CONNECT` (TCP) only. Full end-to-end UDP/QUIC
  forwarding is not implemented in this Android chain yet; some apps that strongly prefer QUIC
  (for example YouTube) may degrade or fail until a UDP-capable sidecar path is shipped.
- when YouTube is routed through VPN, runtime switches to a simplified in-VPN path for that
  session (`TUN -> tun2proxy -> local Xray SOCKS with UDP -> VLESS/REALITY`) so YouTube traffic
  stays encrypted/protected while avoiding the TCP-only obfuscator bottleneck.
- local loopback SOCKS remains a hardening boundary, not an absolute one. Current defenses are
  per-launch credentials, random loopback host/port, and UDP disabled on the local Xray inbound.

Runtime preflight:

- the home screen now surfaces a preflight panel for:
  - real imported profile validation;
  - ABI-compatible embedded `xray` and `obfuscator` binaries;
  - required `geoip.dat` and `geosite.dat` assets.
- config generation and service startup both refuse to proceed when preflight is not ready.
- the preflight panel keeps implementation boundaries explicit (notably current UDP/QUIC limits in
  the Android sidecar chain).
