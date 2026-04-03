# Client Scaffolds

В этой директории лежат стартовые каркасы клиентов:

- [desktop/python](d:/projekt/noVPN/client/desktop/python/README.md) для desktop orchestration и генерации локального `config.json`;
- [android](d:/projekt/noVPN/client/android/README.md) для Android `VpnService`, split tunneling и генерации локального `config.json`;
- [common/profiles/reality/default.profile.json](d:/projekt/noVPN/client/common/profiles/reality/default.profile.json) как общий базовый профиль Reality/VLESS.

Оба каркаса пока intentionally minimal: они дают точку входа для UI, routing и lifecycle, но ещё не включают embedded Xray binary, полноценный TUN backend на desktop и foreground-service orchestration на Android.
