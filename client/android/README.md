# Android Scaffold

This Android scaffold now includes:

- a `VpnService`-based foreground runtime;
- config generation for embedded Xray;
- obfuscator sidecar config generation;
- embedded binary installation from `assets/bin/`;
- start/stop controls in `MainActivity`;
- package exclusions through `addDisallowedApplication(...)`.

Expected asset layout:

```text
app/src/main/assets/bin/xray
app/src/main/assets/bin/obfuscator
```

Current launcher assumptions:

- Xray CLI: `xray run -config <path>`
- obfuscator CLI: `obfuscator --config <path>`
