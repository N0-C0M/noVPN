# Desktop Scaffold

This desktop scaffold now includes:

- profile loading from `client/common/profiles/reality/default.profile.json`;
- local Xray config generation;
- runtime startup for embedded `xray.exe` and `obfuscator.exe`;
- Tkinter UI with start/stop controls;
- headless CLI mode for automation.

Expected binary layout:

```text
client/desktop/runtime/bin/xray.exe
client/desktop/runtime/bin/obfuscator.exe
```

Quick start:

```bash
python client/desktop/python/app.py --headless --bypass-ru --start-runtime
```
