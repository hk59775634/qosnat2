# qosnat

高性能 **VPN NAT + 每 IP eBPF 限速** 方案，适用于 Ubuntu 22.04/24.04。宿主机负责限速，独立 `natns` 命名空间负责 Nftables SNAT 与 flowtable 快路径。

## 架构

```
VPN/ASA 客户端 ──► ens19 (宿主机)
                      ├─ eBPF TC：每 IP 令牌桶限速
                      └─ 策略路由 table 100 ──► ipvlan ──► natns
                                                    └─ ens18 SNAT ──► 公网
```

- **策略路由网段**：可配置多个内网 CIDR（如 `10.0.0.0/8` + `172.16.0.0/24`），流量送入 `natns`。
- **共享 IP 池**：上述网段内、未单独映射的地址，默认轮询 SNAT 到共享公网 IP。
- **独立 IP / 网段映射**：在共享池规则之前匹配，优先级更高。

## 仓库结构

| 路径 | 说明 |
|------|------|
| `deploy-nat-qos.sh` | 一键部署（netns、ipvlan、eBPF、systemd） |
| `nat-admin/` | Go 管理后台（Web UI + REST API，:8080） |
| `nat-qos-bpf/` | eBPF 限速程序与 `nat-qos-bpf` CLI |
| `部署说明.txt` | 安装与运维步骤 |
| `开发需求.txt` | 需求与架构说明（当前实现版） |

## 快速开始

```bash
# 1. 克隆后在仓库根目录执行（自动识别源码路径）
git clone https://github.com/hk59775634/qosnat.git
cd qosnat

# 2. 编辑 deploy-nat-qos.sh 头部变量（网卡、公网 IP 等），然后部署
#    会先 apt 安装依赖（此时 WAN 仍在宿主机），再移 WAN 进 netns
sudo FORCE_DEPLOY=1 ./deploy-nat-qos.sh start

# 3. 浏览器打开 http://<LAN_IP>:8080 登录（账号见 /etc/nat-admin/env）
```

部署后配置持久化于 `/var/lib/nat-admin/state.json`，重启由 `nat-admin.service` 自动恢复。

## API 文档

安装后访问：

- ReDoc: `http://<host>:8080/redoc.html`
- OpenAPI: `http://<host>:8080/openapi.yaml`

主要接口：

- `POST /api/policy-routes` — 添加策略路由网段
- `GET/POST/DELETE /api/wan-forwards` — 公网端口 DNAT 到宿主机
- `GET /api/policy-routes` — 列出
- `DELETE /api/policy-routes?cidr=` — 删除
- `POST /api/rate-default` — 全局默认限速
- `POST /api/shared-ips` — 共享公网 IP 池

鉴权：登录 Cookie 或 Header `X-API-Key`。

## 环境变量

| 变量 | 默认 | 含义 |
|------|------|------|
| `DEV_LAN` | ens19 | 内网网卡 |
| `DEV_WAN` | ens18 | 外网网卡（移入 natns） |
| `VPN_NET` | 10.0.0.0/8 | 初始策略路由网段 |
| `POLICY_ROUTES` | 同 VPN_NET | 部署时逗号分隔多网段 |
| `WAN_SSH_DNAT` | 1 | 公网 22 → 宿主机 SSH |
| `WAN_ADMIN_DNAT` | 1 | 公网管理端口 → nat-admin |

## 许可

GPL-2.0（eBPF 部分）。部署前请确认公网 IP 归属与 FORCE_DEPLOY 风险。
