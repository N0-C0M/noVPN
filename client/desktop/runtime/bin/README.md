# Embedded Desktop Binaries

Place the embedded desktop binaries in this directory:

- `xray.exe`
- `obfuscator.exe`

Current launcher assumptions:

- Xray CLI: `xray.exe run -config <path>`
- obfuscator CLI: `obfuscator.exe --config <path>`

If your module 1 obfuscator uses different arguments, update
`client/desktop/python/novpn_client/runtime_manager.py`.
