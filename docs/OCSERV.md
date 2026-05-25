# OpenConnect (ocserv)

AnyConnect 兼容 SSL VPN，使用 **ocserv** 服务端，**从源码安装**（非 apt 包）。

## 安装

```bash
sudo /opt/qosnat2/scripts/install-ocserv.sh
```

可选环境变量：

| 变量 | 默认 |
|------|------|
| `OCSERV_TAG` | `v1.3.0` |
| `OCSERV_PREFIX` | `/usr/local` |
| `OCSERV_SYSCONFDIR` | `/etc/ocserv` |

安装后二进制：`/usr/local/sbin/ocserv`，systemd 单元：`ocserv.service`。

若已在 qosnat2 启用 HTTPS，安装脚本会尝试将 `/etc/qosnat2/tls.crt` 复制为 VPN 证书。

## Web 管理

**VPN → OpenConnect**：启用、端口、地址池（默认 `10.250.0.0/24`）、用户、保存并 Apply。

也可在 API 以 root 触发后台安装：`POST /api/v1/vpn/ocserv/install`（需 qosnatd 以 root 运行）。

## API

| 方法 | 路径 |
|------|------|
| GET | `/api/v1/vpn/ocserv` |
| PUT | `/api/v1/vpn/ocserv` |
| POST | `/api/v1/vpn/ocserv/apply` |
| POST | `/api/v1/vpn/ocserv/install` |
| GET/POST/DELETE | `/api/v1/vpn/ocserv/users` |

## 客户端

- Cisco AnyConnect 客户端，服务器填 `https://<公网IP或域名>:443`
- 或 `openconnect https://<host> --user=<name>`

## 注意

- 需开启 `net.ipv4.ip_forward`（安装脚本会写入 sysctl）
- WAN 口 nft NAT 需允许 VPN 池访问外网（默认 masquerade 通常已覆盖）
- 与 WireGuard（`10.200.0.0/24`）地址池错开，默认 ocserv 使用 `10.250.0.0/24`
- 仍不支持 IPsec / 传统 OpenVPN（不同协议）
