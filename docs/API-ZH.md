# qosnat2 REST API 中文说明

机器可读规范：`api/openapi.yaml`（Web UI → 开发 → API，或 `GET /openapi.yaml`）。

## 鉴权

| 方式 | 说明 |
|------|------|
| Session Cookie | `POST /api/v1/login` 成功后设置 `qosnat_sess` |
| API Key | 请求头 `X-API-Key`（实验性，`/api/v1/api-keys` 管理） |

**无需登录**：`GET /api/v1/health`、`GET /openapi.yaml`、`GET/POST /api/v1/setup/*`、`POST /api/v1/login`。

详见 [API-AUTH.md](./API-AUTH.md)。

## 通用约定

- 基路径：`/api/v1/...`
- 成功写操作常见响应：`{"ok": true}`
- 错误：`{"error": "..."}`，HTTP 4xx/5xx
- 更新类接口常用 query `id` / `name` / `domain` / `username` 指定对象
- 敏感字段（密码、RADIUS secret、伪装密钥）：**GET 不返回**；**PUT 留空表示不修改**

## 功能域索引

| Tag | 说明 | 代表路径 |
|-----|------|----------|
| health | 健康、OpenAPI | `/api/v1/health` |
| setup | 首次引导 | `/api/v1/setup/status` |
| auth | 登录、会话、API Key | `/api/v1/login` |
| network | 网卡、VLAN、VXLAN、WAN、netplan | `/api/v1/interfaces` |
| nat | SNAT/DNAT、防火墙 | `/api/v1/nat/*`、`/api/v1/firewall/*` |
| shaper | QoS 模板与租户 | `/api/v1/shaper/*` |
| stats | 仪表盘 | `/api/v1/stats/dashboard` |
| ebpf | BPF 状态与重载 | `/api/v1/ebpf/*` |
| system | 调优、HTTPS、审计 | `/api/v1/system/*` |
| vpn | WireGuard、ocserv | `/api/v1/vpn/*` |
| diagnostics | 抓包、conntrack | `/api/v1/diagnostics/*` |

Web UI 与路由完整对照见 [UI-API-ALIGNMENT.md](./UI-API-ALIGNMENT.md)。

## ocserv（OpenConnect）API 详解

### 配置读写

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/vpn/ocserv` | 配置 + 安装状态 + `install_job` |
| PUT | 同上 | 仅写 state，不重启服务 |
| POST | `/api/v1/vpn/ocserv/apply` | 写 `ocserv.conf` 并 systemctl 启停 |

GET 响应要点：

- `config`：脱敏后的全局配置（含 groups、vhosts 列表）
- `vhosts_meta`：各域名是否可管理独立用户等
- `status`：ocserv 是否已安装
- `install_job`：安装任务（见下表）
- `radius_secret_set` / `camouflage_secret_set`：是否已配置（不返回明文）

### 安装

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/vpn/ocserv/install` | 后台源码编译安装，**需 root**，返回 202 |
| POST | `/api/v1/vpn/ocserv/uninstall` | 卸载（需 root + `admin_password`） |
| GET | `/api/v1/vpn/ocserv/install/status` | 轮询任务状态 |

`POST /api/v1/vpn/ocserv/install` 请求体（可选）：`{ "version": "1.4.2" }` 指定官方源码 tag。

`install_job.state`：`idle` | `running` | `ok` | `failed`；`log_tail` 为日志末尾约 80 行。

**qosnat2 版本号**：10 位 `YYYYMMDDNN`，清单 [`releases/qosnat2-versions.json`](https://github.com/hk59775634/qosnat2/blob/main/releases/qosnat2-versions.json)。

## 系统版本管理 API（qosnatd）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/system/version` | 当前运行版本、当前 release tag、可切换版本列表 |
| POST | `/api/v1/system/version/switch` | 切换到指定 release 版本（下载二进制并重启 `qosnatd`） |

`POST /api/v1/system/version/switch` 请求体：

```json
{
  "tag": "v1.2.3",
  "current_password": "管理员当前密码"
}
```

说明：

- 仅在 `qosnatd` 以 root 运行时可执行切换（否则 403）。
- 切换流程为下载 `qosnat2-linux-amd64.tar.gz`，覆盖 `/usr/local/bin/qosnatd`，随后重启服务。
- 一键安装写入 `/etc/qosnat2/release-tag`；版本查询优先读取该标签。

### 运行时会话（occtl）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `.../status/detail` | `occtl show status -j` |
| GET | `.../sessions` | 在线用户列表 |
| POST | `.../sessions/disconnect` | body：`username` 或 `id` |

需启用 `advanced.use_occtl`；不可用时 **503**。

### 用户与组

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST/PUT/DELETE | `.../users` | 全局 ocpasswd 用户 |
| GET | `.../users/traffic?username=&period=` | 流量汇总 + 历史曲线（5min 采样） |
| GET/POST/PUT/DELETE | `.../groups` | per-group 配置目录 |

`period`：`24h` | `7d` | `30d` | `365d`（默认 `7d`）。

## WireGuard 补充

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/vpn/wireguard/peers/traffic?name=&period=` | 按 **Peer 名称** 返回历史 `series` + `summary`；`online`/`current` 来自 `wg show IFACE dump`（transfer 计数）。后台每 5 分钟采样写入 `/var/lib/qosnat2/wireguard-peer-traffic.json`。`period` 同 ocserv。 |
| GET | `/api/v1/vpn/wireguard` | `config.peers[]` 附带 `total_rx_bytes` / `total_tx_bytes`（历史 hourly 汇总）。 |

### 虚拟主机（vhost）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST/PUT/DELETE | `.../vhosts` | 按 `domain` 管理 `[vhost:domain]` |
| GET/POST/PUT/DELETE | `.../vhosts/users?domain=` | **仅** plain + 非空 `plain_passwd_path` |

vhost 字段要点（完整见 OpenAPI `OCServVhost`）：

- `auth_method`：空=继承全局；`plain` | `radius` | `certificate`
- `plain_passwd_path`：独立密码文件路径；空则用全局 ocpasswd
- `radius`：非空且含 `server` 时写 `/etc/radcli/vhosts/<domain>.conf`
- `rx_data_per_sec` / `tx_data_per_sec`：**服务端视角**字节/秒；限制客户端上传用 `tx_data_per_sec`

新建 vhost 时服务端会用全局 OCServ 配置做默认值种子（地址池、DNS、限速等），再在高级页覆盖。

### 带宽与 UI 映射

ocserv 配置项为服务端上下行。Web UI「客户端上行/下行」映射为：

- 客户端下行 → `rx_data_per_sec`
- 客户端上行 → `tx_data_per_sec`

多连接测速工具（如默认 speedtest-cli）可能突破单连接限速；单连接或 `--single` 更接近套餐值。

## 网络补充接口

| 方法 | 路径 | 说明 |
|------|------|------|
| PUT | `/api/v1/interfaces/roles` | 设置 `dev_lan` / `dev_wan`，可选 `apply` netplan |
| GET | `/api/v1/network/netplan` | 预览合并后的 netplan YAML |
| POST | `/api/v1/network/netplan/apply` | 应用 netplan（失败回滚） |

## 防火墙补充接口

| 方法 | 路径 | 说明 |
|------|------|------|
| PUT | `/api/v1/firewall/rules/order` | body：`{"order":["id1","id2",...]}` 调整规则顺序 |
| GET/POST/DELETE | `/api/v1/firewall/aliases` | nft 别名对象组（ipv4 / asn） |

## 自动化冒烟

```bash
export BASE=https://127.0.0.1:8080
export ADMIN_USER=admin
export ADMIN_PASS='你的密码'
bash scripts/test-ui-api.sh
```

HTTPS 自签时脚本使用 `curl -k`。

## 相关文档

- [OCSERV.md](./OCSERV.md) — ocserv 部署与排错
- [UI-API-ALIGNMENT.md](./UI-API-ALIGNMENT.md) — Web 页面与 API 路径对照
