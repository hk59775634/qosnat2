# qosnat2

单机双网卡 **QoS（HTB + fq_codel + IFB + eBPF）+ NAT（nftables）** 控制面。架构准则见 [docs/单机双网卡-QoS-NAT-开发说明.md](docs/单机双网卡-QoS-NAT-开发说明.md)。

**与旧 qosnat 差异**：无 netns / ipvlan / flowtable；WAN 留在宿主机；整形为 Shaping（禁止 `TC_ACT_SHOT` policer）。

## 仓库结构

```
cmd/qosnatd/          # 守护进程
internal/             # api, ebpf, shaper, nft, store, sysctl
bpf/classify.bpf.c    # TC clsact（P2 attach）
api/openapi.yaml      # REST 契约
deploy-qos-nat.sh     # 部署
web/                  # P0 最小静态页（P4 Vue）
docs/                 # 开发说明
reference/            # 旧项目对照，禁止部署
```

## 构建

```bash
cd /opt/qosnat2
go mod tidy
go build -o bin/qosnatd ./cmd/qosnatd
make bpf                               # 需 clang + libbpf

# Web UI (P4)
cd web && npm install && npm run build # 产出 web/dist
```

浏览器访问 `http://<host>:8080/`（Vue3 + hash 路由）。

## 部署（P0）

**必须**显式指定网卡（禁止写死 ens18/ens19）：

```bash
cd /opt/qosnat2
sudo DEV_LAN=vlan.3003 DEV_WAN=vlan.907 SHARED_IP_1=203.0.113.1 ./deploy-qos-nat.sh start
```

- 安装路径：`readlink -f` → `/usr/local/bin/qosnatd`
- 状态：`/var/lib/qosnat2/state.json`
- 配置：`/etc/qosnat2/env`
- systemd：`qosnatd.service`（常驻）、`qos-nat.service`（oneshot apply-state）

```bash
curl -s http://127.0.0.1:8080/api/v1/health
sudo nft list ruleset | head -50   # 表 inet qosnat，无 flowtable
```

首次若 `shared_ips` 为空，nft 会跳过直至 API 配置：

```bash
curl -s -c /tmp/c -X POST http://127.0.0.1:8080/api/v1/login \
  -H 'Content-Type: application/json' -d '{"user":"admin","pass":"QosNat@2026"}'
curl -s -b /tmp/c -X POST http://127.0.0.1:8080/api/v1/nat/shared-ips \
  -H 'Content-Type: application/json' -d '{"ip":"203.0.113.1"}'
```

## 最小验收环境

1. 宿主机双网卡：`DEV_LAN` 接内网（10.0.0.0/8 经 ASA/VPN），`DEV_WAN` 接公网且 **在宿主机**（非 netns）。
2. 内网路由：`10.0.0.0/8` → ASA；默认路由 → `DEV_WAN` 网关。
3. `SHARED_IP_1` 为本机 WAN 真实公网 IP；`policy_routes` 含 `10.0.0.0/8`。
4. 内网客户端 `ping 8.8.8.8` / `curl` 经 NAT 出网；`nft list chain inet qosnat forward` 可见非对称 drop 规则。

## P0/P1 状态

| 项 | P0 | P1（当前） |
|----|-----|------------|
| NAT nft | ✅ | ✅ |
| BPF Pin + Map | — | ✅ |
| TC classify + IFB redirect | — | ✅ P2 |
| 动态 HTB（ringbuf/API） | — | ✅ P2 |
| active_host Iterate | — | ✅ |
| Dashboard / GC | — | ✅ P3 |

## 已知限制
- 鉴权仅 HTTP + Cookie / `X-API-Key`

## 后续待办

- **P1** ✅ bpffs Pin、Map CRUD
- **P2** ✅ classify attach + ifb redirect + 动态 HTB + ringbuf
- **P3** ✅ Dashboard + idle GC
- **P4** ✅ Vue3 + Vite + Tailwind pfSense 风格 UI
- **P5** ✅ mark 审计 + RSS/队列监控 + `POST /ebpf/reload`
- **P6** ✅ WireGuard（密钥/Peer/应用/客户端 conf）+ tcpdump 抓包 + conntrack 列表
- **文档** ✅ `api/openapi.yaml` 全量同步；Web **开发 → API (Scalar)**；**状态 → 连接状态**

## 环境变量

| 变量 | 含义 |
|------|------|
| `DEV_LAN` | 内网接口（必填） |
| `DEV_WAN` | 外网接口（必填，宿主机） |
| `ADMIN_PORT` | API 端口，默认 8080 |
| `STATE_FILE` | 默认 `/var/lib/qosnat2/state.json` |

## 参考

- `reference/`：旧 netns + policer，**勿部署**
- 模块路径：`github.com/hk59775634/qosnat2`
