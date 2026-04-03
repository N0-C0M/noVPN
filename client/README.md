# Client Scaffolds

This directory now contains two concrete client scaffolds:

- [desktop/python](d:/projekt/noVPN/client/desktop/python/README.md) for desktop config generation and embedded runtime startup;
- [android](d:/projekt/noVPN/client/android/README.md) for `VpnService`, foreground-service startup, and embedded runtime scaffolding;
- [common/profiles/reality/default.profile.json](d:/projekt/noVPN/client/common/profiles/reality/default.profile.json) as the shared Reality/VLESS profile.

Both scaffolds are still intentionally minimal, but they now include real process lifecycle entry points for embedded Xray and the module 1 obfuscator.
