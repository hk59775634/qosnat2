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
| **VPN** | WireGuard；**ocserv**（OpenConnect，[`scripts/install-ocserv.sh`](scripts/install-ocserv.sh) 官方源码编译安装） |

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

### 开发构建（源码树 + web/dist 目录）

```bash
cd /opt/qosnat2
apt install -y golang-go clang libbpf-dev npm
go mod tidy
go build -o bin/qosnatd ./cmd/qosnatd
make bpf                               # 需 clang + libbpf → bpf/classify.bpf.o

cd web && npm install && npm run build # 产出 web/dist（部署前必须）
```

### Release 单文件（推荐：编译机一次打包，目标机免 npm/Go）

在 **Ubuntu 24.04 x86_64** 且具备 `go`、`clang`、`npm` 的机器上执行（仅构建机需要完整工具链）：

```bash
cd /opt/qosnat2
./scripts/build-release.sh
# 产出: dist/qosnatd-linux-amd64
#       dist/qosnat2-linux-amd64.tar.gz
```

该二进制通过 `-tags release` **内嵌** `web/dist` 与 `classify.bpf.o`，目标主机只需：

```bash
sudo install -m 0755 dist/qosnatd-linux-amd64 /usr/local/bin/qosnatd
sudo ./deploy-qos-nat.sh -SkipWeb start
```

无需在目标机安装 Node/npm，也无需 `web/dist` 目录。仍需要运行时依赖（nftables、iproute2、tc 等），见 `scripts/install-deps.sh`。

浏览器访问 `http://<host>:8080/`（Vue 3 + hash 路由）。

## 一键安装（curl | bash）

> **平台说明**：一键安装默认从 GitHub Release 下载 `qosnatd-linux-amd64` 并部署（目标机不编译），自动安装运行时 apt 依赖（`nftables`、`iproute2`、`dnsmasq` 等）。该流程**仅在 Ubuntu 24.04 x86_64 上完成安装验证**，**强烈推荐使用 Ubuntu 24.04**。其他版本可设置 `QOSNAT_SKIP_OS_CHECK=1` 强制继续（不保证成功）。
>
> **安装方式约定**：除开发环境外，统一使用 release 可执行文件安装/升级/切换版本；不再通过源码编译安装。
>
> **qosnat2 版本号**：`YYYYMMDD` + 每日 2 位序号（如 `2026052801`），见 [`releases/qosnat2-versions.json`](releases/qosnat2-versions.json)；Web 可从 GitHub 拉取清单并切换。**ocserv** 无版本切换，仅支持源码编译安装（官方 tag 如 `1.4.2`）。

从 GitHub 下载 release 二进制并执行部署脚本（需 **root**）：

```bash
# 默认 HTTP（管理端口自动选取未占用端口）
curl -ksSL https://raw.githubusercontent.com/hk59775634/qosnat2/main/scripts/install.sh | bash

# 启用 HTTPS：为公网 IPv4 申请 Let's Encrypt 短期 IP 证书（profile shortlived，约 6 天有效，自动续期）
# 要求：ACME_EMAIL、本机 TCP/80 从公网可达、IP 为公网地址
export ACME_EMAIL=you@example.com
curl -ksSL https://raw.githubusercontent.com/hk59775634/qosnat2/main/scripts/install.sh | bash -s -- ipssl
```

可选：`QOSNAT_RELEASE_TAG=v1.2.3`（固定版本）、`PUBLIC_IP=1.2.3.4`、`ACME_STAGING=1`（测试环境）、`QOSNAT_INSTALL_DIR=/opt/qosnat2`、`QOSNAT_SKIP_OS_CHECK=1`（非 24.04 时）。

## 版本切换（Web UI）

在 **System → General → 版本管理** 中可查看当前版本并切换 release tag。  
切换流程：下载对应版本二进制 → 覆盖 `/usr/local/bin/qosnatd` → 自动重启服务。

## 一键卸载

停止服务，清理 TC / nft / BPF，并删除配置与状态（默认保留 `/opt/qosnat2` 源码，可加 `--purge-repo` 一并删除）：

```bash
# 远程
curl -ksSL https://raw.githubusercontent.com/hk59775634/qosnat2/main/scripts/uninstall.sh | bash -s -- -y

# 本地仓库
sudo ./deploy-qos-nat.sh uninstall -y
sudo ./deploy-qos-nat.sh uninstall -y --purge-repo   # 同时删除源码目录
```

保留配置重装：`sudo ./scripts/uninstall.sh -y --keep-data`

## 部署（本地仓库）

```bash
sudo ./deploy-qos-nat.sh start
# 打开 http://<host>:<ADMIN_PORT>/ → 自动进入 /#/setup

# 本地仓库 + ipssl（需已编译 qosnatd）
export ACME_EMAIL=you@example.com
sudo IPSSL=1 ./deploy-qos-nat.sh start
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
