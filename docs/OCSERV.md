# OpenConnect (ocserv)

AnyConnect 兼容 SSL VPN，使用 **ocserv** 服务端。生产环境通过 **GitHub Release 预编译包** 安装/升级（非 apt 包）；开发环境可选用源码编译。

## 安装（release，推荐）

```bash
# 指定版本（对应 GitHub tag ocserv-1.4.2）
sudo /opt/qosnat2/scripts/install-ocserv.sh --method release --version 1.4.2

# 或指定完整下载 URL
sudo /opt/qosnat2/scripts/install-ocserv.sh --method release --url 'https://github.com/.../ocserv-linux-amd64.tar.gz'
```

安装包 `ocserv-linux-amd64.tar.gz` 内含 `bin/ocserv`、`bin/occtl`、`bin/ocpasswd` 及可选 `systemd/ocserv.service`。版本标签写入 `/var/lib/qosnat2/ocserv-release-tag`。

安装后二进制：`/usr/local/sbin/ocserv`，`occtl`/`ocpasswd` 在 `/usr/local/bin/`，systemd 单元：`ocserv.service`。

若已在 qosnat2 启用 HTTPS，安装脚本会尝试将 `/etc/qosnat2/tls.crt` 复制为 VPN 证书。

## 源码安装（仅开发）

```bash
sudo /opt/qosnat2/scripts/install-ocserv.sh --method source
```

脚本会安装 **Meson/Ninja**、**libradcli-dev** 等依赖；**ocserv ≥ 1.4** 使用 Meson 构建。Web/API 的 `method=source` 仅在**非 release 构建**的 qosnatd 上可用。

可选环境变量：

| 变量 | 默认 |
|------|------|
| `OCSERV_TAG` | `1.4.2` |
| `OCSERV_PREFIX` | `/usr/local` |
| `OCSERV_SYSCONFDIR` | `/etc/ocserv` |

## 构建 release 包（维护者）

在具备编译工具链的主机上：

```bash
sudo ./scripts/build-ocserv-release.sh
# 产出 dist/ocserv-linux-amd64.tar.gz
```

发布：CI 自动生成 10 位版本号（如 `2026052801`），创建 Git tag `ocserv-2026052801`，并更新 [`releases/ocserv-versions.json`](https://github.com/hk59775634/qosnat2/blob/main/releases/ocserv-versions.json)（仅保留最新 5 个）。

## Web 管理

**VPN → OpenConnect**：标签页按运维与配置分组：

| 运维 | 配置 |
|------|------|
| **概览**、**在线会话** | **服务器**、**组**、**虚拟主机**、**用户**、**证书**、**高级** |

- **概览**：安装/运行状态、版本与 release 切换、在线人数及 occtl `show status` 统计。
- **在线会话**：已连接客户端列表，可断开；约每 8 秒自动刷新；支持搜索与分页。
- **服务器**：启用、端口、地址池（默认 `10.250.0.0/24`）、**认证方式**、DNS/路由、保存并 Apply。
- **组**：`config-per-group` 目录、默认组模板、`auto-select-group`；为每组生成独立配置文件（DNS/路由/地址池/限速等）；主配置写入 `select-group`（需 **保存并应用**）。
- **虚拟主机**：`[vhost:域名]` 段（证书、网段、DNS/路由等）；修改后需 **保存并应用** 写入 `ocserv.conf`。
- **用户**：本地 `ocpasswd` 用户增删改；组从已定义组列表选择；**流量** 打开悬浮窗查看统计与 SNMP 风格曲线（需 occtl，后台每 5 分钟采样，历史保留 1 年）。
- **证书**：TLS 路径、qosnat2 证书复用、cert-user-oid、tls-priorities。
- **高级**：功能开关（TCP/UDP、occtl、限速等）、伪装、带宽与超时参数（`config-per-group` 已移至 **组** 标签）。

### 运维面板与 occtl

实时会话与统计依赖 **occtl** 控制套接字，需同时满足：

1. 高级配置中开启 **occtl**（`use-occtl = true`），保存并 **Apply**。
2. **socket-file** 与配置一致（默认 `/var/run/ocserv-socket`）。启用 **isolate-workers** 时进程可能创建带哈希后缀的 socket 文件，occtl 由程序自动发现（勿强行 `-s` 连接 worker socket）。
3. ocserv 运行且 socket 可访问。
4. 主机已安装 `occtl`（release 包或源码安装均会装到 `/usr/local/bin/occtl`）。

**Apply 与重启**：ocserv 已在运行时，Apply 优先 `systemctl reload`（热加载配置，不断开已有 VPN）。仅首次启动或 reload 失败时才 `restart`。**添加/删除本地用户**会立即写入 `/etc/ocserv/ocpasswd`，无需再点 Apply。

未启用 occtl 或 ocserv 未运行时，概览/在线会话 API 返回 503 及说明文字。RADIUS 认证不影响查看在线会话，但 **用户** 标签仅管理 plain 本地用户。

### 高级配置

通过开关启用/停用 ocserv 能力，例如：TCP/UDP、MTU 探测、DTLS/Cisco 兼容、伪装站点（camouflage）、压缩、keepalive/DPD、rekey、occtl 等；并可调限速、封禁、日志、带宽（高级配置中按 **M（Mbps）** 填写**客户端**上下行：下行→`tx-data-per-sec`，上行→`rx-data-per-sec`，单位字节/秒）、`max-same-clients` 与各类超时。RADIUS 认证页提供「RADIUS 属性说明」悬浮窗（ocserv 可下发属性与示例）。

基础区另支持：**DNS**、**route / no-route**（多行）、**证书路径**（`server-cert`/`server-key`/`ca-cert`）、**socket-file**、**tls-priorities**、**cert-user-oid**、**default-domain**。

**组（config-per-group）**：仅 **本地 (plain)** 认证时写入 `config-per-group` 与每组 `.conf` 文件。**RADIUS** 模式使用 `radius[groupconfig=true]` 从 RADIUS 拉取组/用户配置，不可同时写 `config-per-group`（否则 ocserv 拒绝启动）；仍可配置 `select-group` 供登录时选择组名发给 RADIUS。

**虚拟主机（vhost）**：在 `ocserv.conf` 末尾追加 `[vhost:example.com]`，可为不同域名指定证书、认证方式与地址池。

### 认证

| 方式 | 说明 |
|------|------|
| **本地用户** | `ocpasswd`，在 UI 管理用户列表 |
| **RADIUS** | 使用 radcli；**保存或应用** OCServ 配置时生成 `/etc/radcli/radiusclient.conf`、`servers`、`dictionary`。`servers` 仅写主机/IP（无端口），端口在 `authserver`/`acctserver` 行。更新 `qosnatd` 后需 `systemctl restart qosnatd`。 |

RADIUS 常用参数：服务器地址、认证/计费端口（1812/1813）、共享密钥、groupconfig、NAS-Identifier、计费（acct）与上报间隔。

**FreeRADIUS 注意**：ocserv 不发送 `NAS-Port`，需在服务器 `acct_unique` 中去掉对该属性的依赖，否则计费可能异常（见 ocserv `doc/README-radius.md`）。

### RADIUS Access-Challenge 选路（独立 FreeRADIUS）

AnyConnect 不支持「先登录再下拉选组」。在**独立 FreeRADIUS** 上可用 **Access-Challenge** 实现纯键盘二次选路；qosnat2 的 ocserv 经 radcli **原生支持** Challenge/`State` 往返，不在控制面实现状态机。

| 阶段 | RADIUS 响应 | 要点 |
|------|-------------|------|
| 1 | Access-Request（账号密码） | ocserv 转发至 FreeRADIUS |
| 2 | Access-Challenge | `Reply-Message`（菜单文案）+ `State`（会话令牌，建议 Redis TTL ~120s） |
| 3 | 客户端 | AnyConnect 弹窗，用户输入选路码 |
| 4 | Access-Accept | 携带同一 `State`；下发 `Framed-IPv6-Prefix` 等（可与 `groupconfig` 一并返回 Class、RP 限速） |

**qosnat2 职责**：生成 `/etc/radcli/*` 与字典（含 `State`、`Framed-IPv6-Prefix`）；Web **VPN → OpenConnect → RADIUS** 区提供「RADIUS 属性说明」与并列的「二次挑战选路说明」悬浮窗。

**FreeRADIUS 侧（自行部署）**：Python/Redis 状态机、`authorize` 顶置模块、有 `State` 时勿再 PAP 校验选路码等，见 UI 说明与示例；NPTv6/NAT64/路由撤销不在 qosnat2 范围内。

也可在 API 以 root 触发后台安装：`POST /api/v1/vpn/ocserv/install`（body 可选 `method`、`version`；需 qosnatd 以 root 运行）。

版本切换：`POST /api/v1/vpn/ocserv/version/switch`（`version` + `admin_password`）。

## API

| 方法 | 路径 |
|------|------|
| GET | `/api/v1/vpn/ocserv` |
| PUT | `/api/v1/vpn/ocserv` |
| POST | `/api/v1/vpn/ocserv/apply` |
| GET | `/api/v1/vpn/ocserv/version` |
| POST | `/api/v1/vpn/ocserv/version/switch` |
| POST | `/api/v1/vpn/ocserv/install` |
| POST | `/api/v1/vpn/ocserv/uninstall` |
| GET/POST/DELETE | `/api/v1/vpn/ocserv/users`（仅 `auth_method=plain`） |
| GET | `/api/v1/vpn/ocserv/users/traffic?username=&period=24h\|7d\|30d\|365d` |
| GET/POST/PUT/DELETE | `/api/v1/vpn/ocserv/groups`（组 CRUD；GET 含全局路径；RADIUS 下 POST/PUT/DELETE 返回 400） |
| GET/POST/PUT/DELETE | `/api/v1/vpn/ocserv/vhosts`（vhost CRUD；写入后重写 ocserv.conf） |
| GET | `/api/v1/vpn/ocserv/status/detail`（occtl 统计，需 `use_occtl`） |
| GET | `/api/v1/vpn/ocserv/sessions`（在线会话列表） |
| POST | `/api/v1/vpn/ocserv/sessions/disconnect`（body：`username` 或 `id`） |

## 排错

- 与 WireGuard（`10.200.0.0/24`）地址池错开，默认 ocserv 使用 `10.250.0.0/24`
- 若先安装 ocserv 再装 radcli，需**重新运行**安装脚本以启用 RADIUS
- release 列表拉取失败时 Web 仍提供默认版本 `1.4.2` 供安装
