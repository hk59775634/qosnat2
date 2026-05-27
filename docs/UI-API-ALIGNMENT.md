# Web UI 与 REST API 对齐说明

以 **Web UI 实际调用** 为准，与 `internal/api/server.go` 路由对照。辅助参考：`web/src/api/client.js`（结构化封装）、各 `web/src/views/**/*.vue`（`api.*` 或 `api.get/post/put/del`）。

## 路由一览（UI 已使用）

| 功能域 | 方法 | 路径 | 主要 UI |
|--------|------|------|---------|
| 健康 / 引导 | GET | `/api/v1/health` | 路由守卫、Capture |
| | GET | `/api/v1/setup/status` | Setup、AppLayout |
| | GET | `/api/v1/setup/interfaces` | Setup |
| | POST | `/api/v1/setup/complete` | Setup |
| 会话 | POST | `/api/v1/login` | Login |
| | GET | `/api/v1/session` | 路由守卫 |
| | POST | `/api/v1/logout` | AppLayout |
| | * | `/api/v1/api-keys` | ApiKeys |
| 统计 | GET | `/api/v1/stats/dashboard` | Dashboard、client |
| NAT | * | `/api/v1/nat/policy-routes` | Outbound |
| | * | `/api/v1/nat/shared-ips` | Outbound |
| | * | `/api/v1/nat/static-mappings` | Outbound |
| | * | `/api/v1/nat/prefix-mappings` | Outbound |
| | * | `/api/v1/nat/wan-forwards` | PortForwards |
| 限速 | * | `/api/v1/shaper/profiles` | Profiles |
| | * | `/api/v1/shaper/profiles/order` | Profiles |
| | POST | `/api/v1/shaper/wizard` | Profiles |
| | * | `/api/v1/shaper/tenants` | Tenants |
| | GET | `/api/v1/shaper/active` | ActiveHosts |
| | PUT | `/api/v1/shaper/tc` | Profiles |
| eBPF / 系统 | GET | `/api/v1/ebpf/maps` | EbpfMaps、MarkPolicy |
| | GET | `/api/v1/ebpf/programs` | EbpfMaps |
| | POST | `/api/v1/ebpf/reload` | EbpfMaps |
| | GET | `/api/v1/system/mark-policy` | MarkPolicy |
| | * | `/api/v1/system/tuning` | Advanced |
| | * | `/api/v1/system/general` | General |
| | POST | `/api/v1/system/tls/acme` | General |
| | GET | `/api/v1/system/audit` | Audit |
| 防火墙 | * | `/api/v1/firewall/rules` | FirewallRules |
| | PUT | `/api/v1/firewall/rules/order` | FirewallRules |
| | * | `/api/v1/firewall/aliases` | Aliases |
| 网络 | * | `/api/v1/interfaces` | Interfaces |
| | * | `/api/v1/interfaces/roles` | Interfaces |
| | * | `/api/v1/interfaces/ethtool` | Interfaces |
| | GET | `/api/v1/interfaces/queues` | Queues |
| | * | `/api/v1/network/vlans` | Vlans |
| | * | `/api/v1/network/vxlan` | Vxlan |
| | * | `/api/v1/network/wan-links` | WanLinks |
| | GET/POST | `/api/v1/network/netplan` | Interfaces |
| | POST | `/api/v1/network/netplan/apply` | Interfaces |
| 路由 / DHCP | * | `/api/v1/routes` | Routes |
| | POST | `/api/v1/routes/apply` | Routes |
| | * | `/api/v1/dhcp` | Dhcp、Dashboard |
| | POST | `/api/v1/dhcp/apply` | Dhcp |
| VPN | * | `/api/v1/vpn/wireguard` | WireGuard、Dashboard |
| | POST | `/api/v1/vpn/wireguard/keys` | WireGuard |
| | POST | `/api/v1/vpn/wireguard/apply` | WireGuard |
| | * | `/api/v1/vpn/wireguard/peers` | WireGuard |
| | GET | `/api/v1/vpn/wireguard/peers/traffic` | WireGuard Peer 流量历史/实时 |
| | GET | `/api/v1/vpn/wireguard/peers/{name}/conf` | WireGuard（新窗口下载） |
| | * | `/api/v1/vpn/ocserv` | OCServ、VhostAdvanced |
| | * | `/api/v1/vpn/ocserv/install` | OCServ |
| | GET | `/api/v1/vpn/ocserv/install/status` | OCServ |
| | POST | `/api/v1/vpn/ocserv/apply` | OCServ |
| | GET | `/api/v1/vpn/ocserv/status/detail` | OCServ |
| | GET | `/api/v1/vpn/ocserv/sessions` | OCServ |
| | POST | `/api/v1/vpn/ocserv/sessions/disconnect` | OCServ |
| | * | `/api/v1/vpn/ocserv/users` | OCServ |
| | GET | `/api/v1/vpn/ocserv/users/traffic` | OCServ |
| | * | `/api/v1/vpn/ocserv/groups` | OCServ |
| | * | `/api/v1/vpn/ocserv/vhosts` | OCServ、VhostAdvanced |
| | * | `/api/v1/vpn/ocserv/vhosts/users` | VhostPlainUsers |
| 诊断 | * | `/api/v1/diagnostics/captures` | Capture |
| | GET | `/api/v1/diagnostics/captures/{id}/download` | Capture |
| | GET | `/api/v1/diagnostics/conntrack` | Conntrack |
| 文档 | GET | `/openapi.yaml` | ApiDocs |

`*` 表示 GET/POST/PUT/DELETE 中 UI 会用到的一种或多种，详见 `server.go`。

## OpenAPI

规范文件：`api/openapi.yaml`；中文索引与 ocserv 说明：`docs/API-ZH.md`。

已纳入 OpenAPI（与 UI / `server.go` 一致）的条目包括：

- `PUT /api/v1/interfaces/roles`、`GET/POST /api/v1/network/netplan`、`POST .../netplan/apply`
- `PUT /api/v1/firewall/rules/order`、`/api/v1/firewall/aliases`
- `GET /api/v1/vpn/ocserv/install/status`、`/api/v1/vpn/ocserv/vhosts/users`（CRUD）
- 扩展 `OCServVhost`、`OCServInstallJobStatus` 等 schema 与各接口 `description`

## 自动化验证

```bash
export BASE=https://127.0.0.1:8080   # 或 http://...
export ADMIN_USER=admin
export ADMIN_PASS='你的密码'
bash scripts/test-ui-api.sh
```

脚本会：登录 → 对 **各只读 GET** 断言 200 → 对 ocserv `status/detail` / `sessions` 允许 200 或 503 → 校验 `vhosts/users` 不存在域名返回 404 → 对首个网卡请求 `ethtool` → 写入一条 NAT 冒烟数据。

**说明**：HTTPS 自签时脚本会自动加 `curl -k`（与验收脚本习惯一致）。

## 已知差异 / 注意

- **写操作**：全量 POST/PUT 破坏性测试未在 `test-ui-api.sh` 中覆盖，避免污染生产 state；以 UI 手工或专用环境为准。
- **ACME**：`POST /api/v1/system/tls/acme` 需 root 运行 `qosnatd`，UI 与 API 字段 `action`、`current_password` 一致。
- **ocserv 安装**：`POST /api/v1/vpn/ocserv/install` 返回 202，不在只读冒烟列表中。
