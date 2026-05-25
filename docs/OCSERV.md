# OpenConnect (ocserv)

AnyConnect 兼容 SSL VPN，使用 **ocserv** 服务端，**从源码安装**（非 apt 包）。

## 安装

```bash
sudo /opt/qosnat2/scripts/install-ocserv.sh
```

脚本会安装 **libradcli-dev** 并编译带 RADIUS 支持的 ocserv。安装完成后可用 `ldd /usr/local/sbin/ocserv | grep radcli` 确认。

可选环境变量：

| 变量 | 默认 |
|------|------|
| `OCSERV_TAG` | `v1.3.0` |
| `OCSERV_PREFIX` | `/usr/local` |
| `OCSERV_SYSCONFDIR` | `/etc/ocserv` |

安装后二进制：`/usr/local/sbin/ocserv`，systemd 单元：`ocserv.service`。

若已在 qosnat2 启用 HTTPS，安装脚本会尝试将 `/etc/qosnat2/tls.crt` 复制为 VPN 证书。

## Web 管理

**VPN → OpenConnect**：启用、端口、地址池（默认 `10.250.0.0/24`）、**认证方式**、**高级配置**（功能开关）、保存并 Apply。

### 高级配置

通过开关启用/停用 ocserv 能力，例如：TCP/UDP、MTU 探测、DTLS 旧版兼容、Cisco 客户端兼容、隔离 worker、漫游、压缩、keepalive/DPD、rekey、occtl 等；并可调 `max-same-clients`、超时与间隔类数值。配置写入 `ocserv.conf`。

### 认证

| 方式 | 说明 |
|------|------|
| **本地用户** | `ocpasswd`，在 UI 管理用户列表 |
| **RADIUS** | 使用 radcli，Apply 时生成 `/etc/radcli/radiusclient.conf`、`servers`、`dictionary` |

RADIUS 常用参数：服务器地址、认证/计费端口（1812/1813）、共享密钥、groupconfig、NAS-Identifier、计费（acct）与上报间隔。

**FreeRADIUS 注意**：ocserv 不发送 `NAS-Port`，需在服务器 `acct_unique` 中去掉对该属性的依赖，否则计费可能异常（见 ocserv `doc/README-radius.md`）。

也可在 API 以 root 触发后台安装：`POST /api/v1/vpn/ocserv/install`（需 qosnatd 以 root 运行）。

## API

| 方法 | 路径 |
|------|------|
| GET | `/api/v1/vpn/ocserv` |
| PUT | `/api/v1/vpn/ocserv` |
| POST | `/api/v1/vpn/ocserv/apply` |
| POST | `/api/v1/vpn/ocserv/install` |
| GET/POST/DELETE | `/api/v1/vpn/ocserv/users`（仅 `auth_method=plain`） |

`PUT` 体字段：`auth_method`（`plain` \| `radius`）、`radius`（含 `server`、`auth_port`、`secret` 等）。

## 客户端

- Cisco AnyConnect 客户端，服务器填 `https://<公网IP或域名>:443`
- 或 `openconnect https://<host> --user=<name>`

## 注意

- 需开启 `net.ipv4.ip_forward`（安装脚本会写入 sysctl）
- WAN 口 nft NAT 需允许 VPN 池访问外网（默认 masquerade 通常已覆盖）
- 与 WireGuard（`10.200.0.0/24`）地址池错开，默认 ocserv 使用 `10.250.0.0/24`
- 仍不支持 IPsec / 传统 OpenVPN（不同协议）
- 若先安装 ocserv 再装 radcli，需**重新运行**安装脚本以启用 RADIUS
