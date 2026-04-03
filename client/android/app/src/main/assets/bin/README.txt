Place embedded Android binaries here before packaging:

- bin/xray
- bin/obfuscator

Current launcher assumptions:

- xray is launched as: xray run -config <path>
- obfuscator is launched as: obfuscator --config <path>

If your module 1 binary uses a different CLI, update
com.novpn.vpn.EmbeddedRuntimeManager.
