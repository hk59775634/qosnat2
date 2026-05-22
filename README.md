# qosnat2

单机双网卡 **QoS（HTB + fq_codel + IFB + eBPF）+ NAT（nftables）** 控制面。架构准则见 [docs/单机双网卡-QoS-NAT-开发说明.md](docs/单机双网卡-QoS-NAT-开发说明.md)。

**与旧 qosnat 差异**：无 netns / ipvlan / flowtable；WAN 留在宿主机；整形为 Shaping（禁止 `TC_ACT_SHOT` policer）。

## 功能概览

| 模块 | 说明 |
|------|------|
| **初始设置向导** | 安装后仅启动 Web；浏览器完成管理员、LAN/WAN、策略路由等后再加载数据面（类似 AdGuard Home） |
| **高级设置 · 系统优化** | sysctl / conntrack / TCP / 网卡 txqueuelen & RPS / QoS 叶子队列；按 **CPU+内存** 自动推荐，可手动覆盖 |
| **防火墙规则** | 自定义 forward/input nft 规则（Web 编辑） |
| **常规设置 / 审计** | hostname/密码、**HTTPS 开关与证书**、审计、API 密钥 |
| **NAT** | Outbound SNAT 池、1:1、前缀映射、WAN 端口转发、策略路由 |
| **QoS** | Per-IP HTB + eBPF `profile_lpm`；网段与单 IP（`/32`）均在 QoS 策略页，`POST /shaper/wizard` |
| **网络** | 接口 4h 流量、实时速率（线速占比/手动基准）、ethtool、**netplan** IPv4/VLAN、路由、DHCP |
| **可观测** | Dashboard、eBPF Maps、Mark 审计、conntrack、tcpdump 抓包 |
| **VPN** | WireGuard 密钥 / Peer / `wg-quick` 应用 |

## 仓库结构

```
cmd/qosnatd/              # 守护进程（REST + 静态 Web）
internal/
  api/                    # HTTP 处理器（setup、nat、shaper、tuning…）
  ebpf/ shaper/ nft/      # 数据面
  store/ sysctl/ tuning/  # 持久化与内核调优
  netif/ route/ dnsmasq/  # 网卡、路由、DHCP
bpf/classify.bpf.c        # TC clsact
api/openapi.yaml          # REST 契约
deploy-qos-nat.sh         # 部署脚本
web/                      # Vue 3 + Vite + Tailwind（构建产物 web/dist）
scripts/test-ui-api.sh    # API 冒烟测试
docs/                     # 开发说明、UI 规划
reference/                # 旧项目对照（勿部署）
```

## 构建

```bash
cd /opt/qosnat2
go mod tidy
go build -o bin/qosnatd ./cmd/qosnatd
make bpf                               # 需 clang + libbpf → bpf/classify.bpf.o

cd web && npm install && npm run build # 产出 web/dist（部署前必须）
```

浏览器访问 `http://<host>:8080/`（Vue 3 + hash 路由）。

## 部署

```bash
sudo ./deploy-qos-nat.sh start
# 打开 http://<host>:8080/ → 自动进入 /#/setup
```

| 路径 | 说明 |
|------|------|
| `/usr/local/bin/qosnatd` | 控制面二进制 |
| `/var/lib/qosnat2/state.json` | 持久化（`setup_complete`、NAT/QoS/调优） |
| `/etc/qosnat2/env` | 运行时 LAN/WAN（向导写入） |
| `/etc/sysctl.d/99-qosnat2.conf` | 系统优化 sysctl |
| `qosnatd.service` | 常驻 Web/API |
| `qos-nat.service` | 向导完成后 enable，回放 TC/nft |

安装后**不**自动加载 TC/nft；完成向导并应用数据面后生效。

预置网卡（可选）：

```bash
DEV_LAN=ens19 DEV_WAN=ens18 ADMIN_USER=admin ADMIN_PASS='your-pass' \
  sudo ./deploy-qos-nat.sh start
```

健康检查：

```bash
curl -s http://127.0.0.1:8080/api/v1/health
curl -s http://127.0.0.1:8080/api/v1/setup/status
```

## Web UI 菜单

- **Dashboard** — 吞吐、会话、WAN/LAN 速率
- **Network** — 接口（netplan）、VLAN、多 WAN、路由、DHCP、RSS/多队列
- **Security** — Outbound NAT、端口转发
- **Traffic** — QoS 策略、活跃 Per-IP
- **System** — 常规设置、高级设置、API 密钥、审计日志、OpenAPI
- **Security** — 防火墙规则（forward/input）
- **Observability / VPN / Diagnostics** — eBPF、Mark、conntrack、抓包、WireGuard

> `/32` 限速在 **QoS 策略** 配置；**HTTPS** 在 **常规设置** 页启用并上传/粘贴证书。

### 系统优化（高级设置）

- 首次引导或首次打开页面时，按宿主机 **CPU 核数 + 内存** 写入推荐档位（低 / 中 / 高）。
- 可调：`nf_conntrack_*`、TCP/backlog、邻居表、RPS、`txqueuelen`、HTB 叶子（`fq_codel` / `fq`）、Per-IP 空闲回收等。
- API：`GET/PUT /api/v1/system/tuning`（`apply_recommended` 按当前硬件重算）。

## API 与测试

- OpenAPI：`api/openapi.yaml`，运行时 `GET /openapi.yaml`
- 鉴权：见 [docs/API-AUTH.md](docs/API-AUTH.md)（Session Cookie / `QOSNAT_API_KEY`）
- 冒烟：`scripts/test-ui-api.sh`
- HTTPS 验收：`sudo QOSNAT_PASS=… scripts/acceptance-https.sh`
- P2 iperf 对账：`sudo bash -c 'source /etc/qosnat2/env; scripts/acceptance-p2-iperf.sh'`（需 SSH `root@100.64.0.254`）
- P3 冒烟：`sudo bash -c 'source /etc/qosnat2/env; scripts/acceptance-p3-smoke.sh'`
- CI：GitHub Actions `.github/workflows/ci.yml`（`go test` + `npm run build`）

## 部署选项

```bash
sudo ./deploy-qos-nat.sh start           # 默认：无 dist 时才 npm build
sudo ./deploy-qos-nat.sh -BuildWeb start # 强制重建 web/dist
sudo ./deploy-qos-nat.sh -SkipWeb start  # 跳过前端构建
```

## 最小验收环境

1. 宿主机双网卡：`DEV_LAN` 内网（如 `10.0.0.0/8`），`DEV_WAN` 公网且在宿主机。
2. 内网默认经 ASA/VPN；默认路由走 WAN 网关。
3. 完成向导并应用数据面；内网客户端可 `ping` / `curl` 出网。
4. `nft list chain inet qosnat forward` 可见非对称 drop 规则；Dashboard 有速率。

## 实现阶段（已完成）

| 阶段 | 内容 |
|------|------|
| P0–P1 | nft NAT、bpffs、eBPF Map |
| P2 | TC classify + IFB + 动态 HTB |
| P3 | Dashboard、idle GC |
| P4 | Vue 3 Web UI |
| P5 | Mark 审计、RSS、ebpf reload |
| P6 | WireGuard、抓包、conntrack |
| — | 引导向导、接口/DHCP/路由 API、系统优化与硬件推荐 |

## 环境变量

| 变量 | 含义 |
|------|------|
| `DEV_LAN` | 内网接口 |
| `DEV_WAN` | 外网接口（宿主机） |
| `ADMIN_PORT` | API 端口，默认 8080 |
| `STATE_FILE` | 默认 `/var/lib/qosnat2/state.json` |
| `ADMIN_USER` / `ADMIN_PASS` | 可选，部署时预置管理员 |

## 参考

- `reference/`：旧 netns + policer 实现，**仅对照，勿部署**
- Go 模块：`github.com/hk59775634/qosnat2`
- 上游：<https://github.com/hk59775634/qosnat2>
