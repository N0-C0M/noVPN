# Client Scaffolds

This directory now contains two concrete client scaffolds:

- [desktop/python](d:/projekt/noVPN/client/desktop/python/README.md) for desktop profile management, multi-server selection, embedded runtime startup, desktop logging, Windows installer builds, and the initial Windows system-tunnel path;
- [android](d:/projekt/noVPN/client/android/README.md) for `VpnService`, foreground-service startup, and embedded runtime scaffolding;
- [common/profiles/reality/default.profile.json](d:/projekt/noVPN/client/common/profiles/reality/default.profile.json) as the shared Reality/VLESS profile.

Shared client capabilities now include:

- real process lifecycle entry points for embedded Xray and the module 1 obfuscator;
- invite-driven import of one or more server profiles;
- persisted profile metadata such as `server_id` and `api_base`;
- generated runtime/config state that can be reused in repo mode and packaged Windows builds;
- a Windows installer path via Inno Setup;
- an initial Windows system-tunnel implementation based on Xray TUN + `wintun.dll`.
