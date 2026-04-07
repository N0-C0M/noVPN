Place embedded Android binaries here before packaging.

Shared Xray assets:

- bin/geoip.dat
- bin/geosite.dat

ABI-aware binary layout:

- bin/arm64-v8a/xray
- bin/arm64-v8a/obfuscator
- bin/armeabi-v7a/xray
- bin/armeabi-v7a/obfuscator
- bin/x86_64/xray
- bin/x86_64/obfuscator
- bin/x86/xray
- bin/x86/obfuscator

The runtime selects the first matching entry from `Build.SUPPORTED_ABIS`
and falls back to the legacy flat layout (`bin/xray`, `bin/obfuscator`).

Current launcher assumptions:

- xray is launched as: xray run -config <path>
- obfuscator is launched as: obfuscator --config <path>

If your module 1 binary uses a different CLI, update
com.novpn.vpn.EmbeddedRuntimeManager.
