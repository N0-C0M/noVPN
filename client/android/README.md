# Android Scaffold

This Android scaffold now includes:

- a `VpnService`-based foreground runtime;
- config generation for embedded Xray;
- obfuscator sidecar config generation;
- ABI-aware embedded binary installation from `assets/bin/`;
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

2. Option A: update the bundled client profiles in this repo:

   ```bash
   go run ./cmd/client-profile-sync -input /path/to/client-profile.yaml
   ```

   This rewrites:

   - `client/common/profiles/reality/default.profile.json`
   - `client/android/app/src/main/assets/profile.default.json`

3. Option B: build the APK once, then import the server profile inside the app with the new
   `Import` button in the top-right corner. The importer accepts the server-generated
   `client-profile.yaml` directly and stores it as an internal selectable profile.

Current limitation:

- the app now supports real server profile import and validation, but the full Android
  `TUN -> Xray` traffic pipeline is still incomplete in this scaffold. The runtime starts
  `VpnService`, `xray`, and the obfuscator sidecar, but a production-ready packet forwarding
  path still needs a dedicated Android integration layer.
