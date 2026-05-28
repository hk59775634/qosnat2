# OpenConnect (ocserv)

AnyConnect 兼容 SSL VPN，使用 **ocserv** 服务端，通过 **官方源码编译** 安装（非 apt 包、不提供 GitHub 预编译包切换）。

## 安装

```bash
sudo /opt/qosnat2/scripts/install-ocserv.sh
sudo /opt/qosnat2/scripts/install-ocserv.sh --version 1.4.2
```

脚本会安装 **Meson/Ninja**、**libradcli-dev** 等依赖；**ocserv ≥ 1.4** 使用 Meson 构建。

可选环境变量：

| 变量 | 默认 |
|------|------|
| `OCSERV_TAG` / `OCSERV_VERSION` | `1.4.2` |
| `OCSERV_PREFIX` | `/usr/local` |
| `OCSERV_SYSCONFDIR` | `/etc/ocserv` |

安装后二进制：`/usr/local/sbin/ocserv`，`occtl`/`ocpasswd` 在 `/usr/local/bin/`，systemd 单元：`ocserv.service`。

若已在 qosnat2 启用 HTTPS，安装脚本会尝试将 `/etc/qosnat2/tls.crt` 复制为 VPN 证书。

## Web 管理

**VPN → OpenConnect**：标签页按运维与配置分组：

| 运维 | 配置 |
|------|------|
| **概览**、**在线会话** | **服务器**、**组**、**虚拟主机**、**用户**、**证书**、**高级** |

- **概览**：安装/运行状态、版本、在线人数及 occtl `show status` 统计。
- **在线会话**：已连接客户端列表，可断开；约每 8 秒自动刷新。
- **服务器**：启用、端口、地址池（默认 `10.250.0.0/24`）、认证、DNS/路由、保存并 Apply。
- 其余标签见界面说明。

### 运维面板与 occtl

实时会话与统计依赖 **occtl** 控制套接字，需同时满足：

1. 高级配置中开启 **occtl**，保存并 **Apply**。
2. **socket-file** 与配置一致（默认 `/var/run/ocserv-socket`）。
3. ocserv 运行且 socket 可访问。
4. 主机已安装 `occtl`。

**Apply 与重启**：运行中优先 `systemctl reload`；不可 reload 的项须 `restart`。

### 认证

| 方式 | 说明 |
|------|------|
| **本地用户** | `ocpasswd`，在 UI 管理 |
| **RADIUS** | radcli；保存/应用时生成 `/etc/radcli/*` |

也可在 API 以 root 触发后台安装：`POST /api/v1/vpn/ocserv/install`（可选 `version` 指定官方 tag）。

## API

| 方法 | 路径 |
|------|------|
| GET | `/api/v1/vpn/ocserv` |
| PUT | `/api/v1/vpn/ocserv` |
| POST | `/api/v1/vpn/ocserv/apply` |
| POST | `/api/v1/vpn/ocserv/install` |
| POST | `/api/v1/vpn/ocserv/uninstall` |
| GET | `/api/v1/vpn/ocserv/install/status` |

更多路径见 [API-ZH.md](./API-ZH.md)。

## 排错

- 与 WireGuard（`10.200.0.0/24`）地址池错开，默认 ocserv 使用 `10.250.0.0/24`
- 编译安装需 root，且目标机具备足够磁盘与编译依赖
