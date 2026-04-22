# NoVPN 完整技术文档（中文）

本文档说明当前仓库中的实际实现：服务端、Android/桌面客户端、数据链路、加密与混淆、邀请码/促销码生命周期、客户端策略控制与安全建议。

## 1. 项目范围

核心组件：

- Go 网关：`cmd/gateway`, `internal/*`
- Reality 引导与配置：`cmd/reality-bootstrap`, `internal/core/reality/*`
- 管理后台与 API：`internal/server/admin.go`
- Android 客户端：`client/android/app/src/main/java/com/novpn/*`
- 桌面客户端（Python/Tkinter）：`client/desktop/python/novpn_client/*`
- 混淆运行时：`cmd/obfuscator/*`

当前状态：

- 网关默认安全模式已加固（`source_ip_allowlist` + `policy`）。
- Obfuscator 支持 SOCKS5 `CONNECT` 和 `UDP ASSOCIATE`。
- Android 具备完整 TUN 路径；桌面端既支持本地运行时代理编排，也支持一个初始版 Windows system-tunnel 路径（Xray TUN + `wintun.dll`）。

## 2. 总体架构

### 2.1 服务端

1. `reality-bootstrap`
- 生成并持久化状态（UUID、X25519 密钥、short IDs）
- 渲染 Xray 配置
- 维护 registry（clients/invites/promos）
- 导出客户端 profile YAML

2. `gateway`
- 启动 TCP/UDP 监听
- 执行认证与 ACL
- 代理到上游
- 暴露 health/readiness/metrics

3. `admin`
- 邀请码、促销码、设备生命周期管理
- 公共兑换/断开/诊断接口
- 客户端站点/应用封禁列表与强制通知管理

### 2.2 客户端

Android：

- 生成 Xray/obfuscator 运行时配置
- 启动内置二进制
- 使用 `VpnService` 建立 TUN
- 通过 `tun2proxy` 接入本地代理链

桌面端：

- 导入并保存 profile
- 生成运行时配置
- 启停 `xray.exe` 与 `obfuscator.exe`
- 提供激活、促销码、诊断、路由、设置等 UI
- 支持 Windows 安装包流程（Inno Setup）

## 3. 加密与混淆

### 3.1 加密位置

外部通道加密由 Xray（`vless` + `reality`）完成。  
Obfuscator 不替代加密，而是改变流量时序与分片行为。

相关代码：

- Android：`client/android/.../AndroidXrayConfigWriter.kt`
- Desktop：`client/desktop/python/novpn_client/config_builder.py`
- Server：`internal/core/reality/provision.go`

### 3.2 Obfuscator 行为

`cmd/obfuscator/runtime.go`：

- 处理 SOCKS5 请求
- 支持 `CONNECT` 与 `UDP ASSOCIATE`
- 按 relay plan 做时序/分片调节
- 通过上游 SOCKS 转发

## 4. 数据传输路径

### 4.1 Android

`应用流量 -> VpnService(TUN) -> tun2proxy -> 本地 obfuscator SOCKS -> 本地 Xray SOCKS -> VLESS/REALITY -> 服务端`

服务启动时会先对本地 obfuscator bridge 做 `UDP ASSOCIATE` 探测。探测成功则整条 VPN 会话使用
`obfuscator -> Xray` 链路；探测失败则整条会话回退到
`TUN -> tun2proxy -> 本地 Xray SOCKS (UDP enabled) -> VLESS/REALITY`。这个回退是会话级的，
不是针对某个单独站点或 YouTube 的特殊规则。

### 4.2 Desktop

桌面端支持两种模式：

- 本地运行时模式：运行本地代理链并由 UI 编排；
- Windows system-tunnel 模式：使用 Xray `tun` inbound 和 `wintun.dll`。

当前 system-tunnel 路径：

- 需要 Administrator 权限；
- 通过临时 IPv4 路由切换接管当前会话；
- 仍保留本地 SOCKS/HTTP inbound 供诊断和显式代理使用；
- 默认安装包中仍未集成 WFP helper。

### 4.3 服务端

- TCP 代理：`internal/transport/tcp/proxy.go`
- UDP 代理：`internal/transport/udp/proxy.go`
- 流量统计与限额：`internal/core/reality/traffic.go`

## 5. 邀请码/促销码/设备生命周期

Registry 数据结构：`internal/core/reality/registry.go`

- `InviteRecord`
- `PromoRecord`
- `ClientRecord`

公共兑换接口：

- `POST /admin/redeem/{code}`

行为：

- 邀请码兑换：创建或更新设备绑定客户端
- 促销码兑换：给已绑定设备增加流量额度

断开设备接口：

- `POST /admin/disconnect`

## 6. 自定义与临时促销码

新能力：

- 自定义促销码：`code`
- 使用次数限制：`max_uses`（`0` 表示不限次）
- 临时促销码：`expires_minutes`（到期失效）

规则：

- `code` 为空时自动生成
- 自定义码会转为小写并做格式校验
- 与 invite/promo 代码空间做唯一性校验
- 达到 `max_uses` 或过期后自动失效

后台入口：

- `POST /admin/api/promos`
- 管理面板表单支持 `code`、`bonus_gb`、`max_uses`、`expires_minutes`

## 7. Android 默认白名单模式

Android 支持“默认白名单模式”，可在设置中开关。  
默认包含：YouTube 系列、Telegram、Instagram、X、部分 Supercell 游戏、MEGA、ChatGPT、Gemini（详见 `ClientPreferences.kt`、`SettingsActivity.kt`）。

## 8. 管理后台策略控制

支持：

- 站点/域名封禁列表
- 应用包名封禁列表
- 面向用户的强制通知（可设置过期）

公共读取接口：

- `GET /admin/client/policy`
- `GET /admin/client/notices`

后台更新接口：

- `POST /admin/api/policy/blocklist`
- `POST /admin/api/policy/notices`
- `POST /admin/api/policy/notices/{id}/deactivate`

## 9. 构建与部署

基础构建：

```bash
go build -o gateway ./cmd/gateway
go build -o reality-bootstrap ./cmd/reality-bootstrap
```

仅渲染与校验：

```bash
./reality-bootstrap -config deploy/config.example.yaml -render-only
```

Windows 客户端与安装包：

- 构建脚本：`client/desktop/python/build_windows.ps1`
- 安装脚本：`client/desktop/installer/novpn-desktop.iss`

## 10. 安全建议

1. 保持网关安全模式为 `source_ip_allowlist` + `policy`。
2. 管理后台建议仅通过 SSH 隧道/VPN/反向代理 TLS 暴露。
3. 公共 redeem/disconnect 接口应配合边界防护与限流。
4. 定期轮换 admin token，不建议将 admin 端口直接公网开放。
5. 定期更新 geo 数据与应用/站点目录。

## 11. 关键代码索引

服务端：

- `cmd/gateway/main.go`
- `internal/server/gateway.go`
- `internal/server/admin.go`
- `internal/core/reality/registry.go`
- `internal/core/reality/traffic.go`
- `deploy/config.example.yaml`

Android：

- `client/android/.../NoVpnService.kt`
- `client/android/.../AndroidXrayConfigWriter.kt`
- `client/android/.../ObfuscatorConfigWriter.kt`
- `client/android/.../ClientPreferences.kt`
- `client/android/.../SettingsActivity.kt`

Desktop：

- `client/desktop/python/novpn_client/app.py`
- `client/desktop/python/novpn_client/runtime_manager.py`
- `client/desktop/python/novpn_client/config_builder.py`
- `client/desktop/python/novpn_client/obfuscator_config_builder.py`
- `client/desktop/python/build_windows.ps1`
