# 单机双网卡 QoS + NAT — 新项目开发说明（qosnat2）

> 基于 qosnat 实战经验；**以本目录 `/opt/qosnat2` 为唯一开发根目录**。  
> **拓扑**：宿主机双网卡（LAN + WAN），无 netns / fastpath / ipvlan / veth。  
> **QoS（已确认）**：Per-IP **Shaping**（HTB + fq_codel/fq + IFB）+ **eBPF 控制面**（cilium/ebpf 管理 bpffs）；**禁止**令牌桶 Policing（`TC_ACT_SHOT`）。

---

## 1. 项目目标

| 目标 | 说明 |
|------|------|
| 拓扑 | Linux（Ubuntu 22.04/24.04；VM 默认，**可选 SR-IOV / ConnectX-6 Dx**） |
| NAT | Nftables：端口转发、1:1、Outbound SNAT 池 |
| QoS | `10.0.0.0/8` 内动态 **/32** 独立上下行整形（速率**可配置**，示例 8mbit） |
| 整形 | **Shaping** 排队，禁止 Policing 丢包 |
| 控制面 | Go REST API + **cilium/ebpf** 统一管理 BPF 生命周期与 Map |
| 数据面 | TC clsact 分类 + **HTB** + **fq_codel/fq** + **IFB** 上行 |
| 前端 | pfSense 风格：**TailwindCSS + Vue 3**（或 React），Widget 化 Dashboard |
| 后端 | **Go**（推荐 **Gin** 或 Fiber）+ **Netlink**（rtnetlink）操作 tc/网卡 |

**业务类比**：MikroTik / pfSense **PCQ** — 每终端 IP 动态子队列 + 带宽上限。

---

## 2. 总体架构（控制面 + 数据面）

```
┌─────────────────────────────────────────────────────────────────┐
│  Web UI (Vue/React + Tailwind, pfSense 风格)                     │
└────────────────────────────┬────────────────────────────────────┘
                             │ REST JSON
┌────────────────────────────▼────────────────────────────────────┐
│  qosnatd (Go)                                                    │
│  • Gin/Fiber 路由                                                │
│  • cilium/ebpf：Load/Attach/Pin/MapUpdate/Iterate/Delete         │
│  • netlink：HTB 类 / IFB / VLAN / 多队列                         │
│  • nftables：NAT/ACL（避免污染 skb->mark，见 §9）                 │
│  • state.json 持久化 + 启动时回放 Map                              │
└────────────┬───────────────────────────────┬────────────────────┘
             │ bpffs Map 读写                 │ tc / nft
┌────────────▼───────────────────────────────▼────────────────────┐
│  内核数据面                                                       │
│  LAN ingress: bpf(clsact) → mirred → ifb → HTB → fq (上行)         │
│  LAN egress:  bpf → tc_classid → HTB → fq_codel (下行)           │
│  forward: nft SNAT / filter → WAN                                │
└─────────────────────────────────────────────────────────────────┘
```

| 平面 | 职责 |
|------|------|
| **eBPF Map** | 速率配置、LPM 网段模板、/32 覆盖、活跃主机表（供 UI 遍历） |
| **HTB + fq** | 真实整形与多队列 pacing（执行面） |
| **BPF 程序** | 分类、选 class、首包上报；**不** `TC_ACT_SHOT` |

---

## 3. 与 qosnat（旧项目）差异

| 能力 | qosnat | qosnat2 |
|------|--------|---------|
| NAT | natns + flowtable | 宿主机 **nft** |
| 限速执行 | eBPF policer 丢包 | **HTB + fq** |
| 限速配置 | CLI / 部分 map | **REST + bpf_map_update_elem** |
| 前端 | 单页 admin | **pfSense 式多模块** |
| 后端 | 单文件 nat-admin | **qosnatd** 模块化 |

---

## 4. 网络拓扑（数据面）

```
  VPN/ASA ──► DEV_LAN
                ├─ ingress bpf → mirred → ifb0 → HTB/fq  (上行, saddr /32)
                ├─ egress  bpf → classid → HTB → fq_codel (下行, daddr /32)
                └─ forward → nft SNAT → DEV_WAN → 公网
```

### 4.1 路由与非对称回程

- `10.0.0.0/8 via <ASA> dev DEV_LAN`；`default via <GW> dev DEV_WAN`  
- nft forward 丢弃「公网源 → LAN 上 10.x」直连回程（见旧项目教训 §11.2）

### 4.2 网卡示例（须显式配置）

```bash
DEV_LAN=vlan.3003 DEV_WAN=vlan.907 ./deploy-qos-nat.sh start
```

---

## 5. 仓库结构（以 qosnat2 为核心）

```
qosnat2/
├── cmd/qosnatd/                 # 主守护进程（API + BPF + shaper + nft）
├── internal/
│   ├── ebpf/                    # cilium/ebpf：加载、Map、Attach、Iterate
│   ├── shaper/                  # netlink：HTB 类 CRUD、ifb、GC
│   ├── nft/                     # nftables 生成/应用
│   ├── sysctl/                  # 可调内核参数
│   └── store/                   # state.json 持久化
├── bpf/
│   ├── classify.bpf.c           # TC clsact：分类 + 读 Map（无 SHOT）
│   └── headers/
├── web/                         # Vue3 + Vite + Tailwind（pfSense 皮肤）
│   ├── src/views/               # 按 §10 菜单分模块
│   └── ...
├── deploy-qos-nat.sh
├── api/openapi.yaml             # REST 契约
├── docs/
│   └── 单机双网卡-QoS-NAT-开发说明.md
├── reference/                   # 旧 qosnat，勿部署
└── README.md
```

**`nat-admin/`、`nat-qos-bpf/`（旧）**：仅作参考；新代码迁入 `cmd/qosnatd` + `bpf/classify.bpf.c`，勿双轨维护。

---

## 6. eBPF Map 与 BPF 程序规范

### 6.1 库与生命周期（强制）

- 语言：**Go 1.22+**  
- 库：**github.com/cilium/ebpf**（Pin 到 `/sys/fs/bpf/qosnat2/`）  
- 后端 **唯一** 负责：`LoadCollectionSpec` → `RewriteConstants` → `Load` → `Attach` → `Pin` → 进程退出后由 bpffs 保持  

禁止 Web 或 shell 直接 `bpftool map update`（除调试）；生产变更 **必须** 走 REST API。

### 6.2 Map 定义（建议）

| Map 名 | 类型 | Key | Value | 用途 |
|--------|------|-----|-------|------|
| `profile_lpm` | **LPM trie** | `struct lpm_v4_key` | `struct rate_val` | PCQ 网段模板（如 10.0.0.0/8→8M） |
| `host_exact` | **Hash** | `__u32` IPv4 BE | `struct rate_val` | VIP /32 覆盖（50M 等） |
| `active_host` | **LRU hash** | `__u32` host IP | `struct active_val` | 当前活跃主机（UI 状态页遍历） |
| `classid_map` | Hash | `__u32` host IP | `__u32` classid | 内核侧 classid 缓存 |

```c
/* rate_val：字节/秒，与 tc HTB 一致；API 接收 mbit 后换算 */
struct rate_val {
    __u64 down_bps;   /* 下行（LAN egress / 客户下载） */
    __u64 up_bps;     /* 上行（LAN ingress / 客户上传） */
    __u32 class_minor;
    __u8  pad[4];
};

struct active_val {
    __u64 bytes_down;
    __u64 bytes_up;
    __u64 last_seen_ns;
    __u32 class_minor;
    __u32 flags;
};
```

**查找优先级（BPF 内）**：`host_exact`（/32）> `profile_lpm`（最长前缀）> `default`（Map 中 0.0.0.0/0 或常量）。

### 6.3 TC 程序行为（`classify.bpf.c`）

| 钩子 | 方向 | 行为 |
|------|------|------|
| `tc/ingress` | 上行 | 查 Map 得 rate；写 `tc_classid`；`mirred` → ifb；更新 `active_host` |
| `tc/egress` | 下行 | 查 Map；写 `tc_classid` → 宿主机 HTB 树 |

**禁止**：`TC_ACT_SHOT`、令牌桶丢包。

首包若 `active_host` 无条目：写 Map 占位 + **ringbuf** 事件 `NEW_HOST` → Go 创建 HTB 类（与 Map 中 rate 一致）。

### 6.4 速率单位换算（API 层）

```
用户输入:  "8mbit" / "50mbit"
API 存储:  state.json 保留字符串
写入 Map:  bps = mbit * 125000   /* 字节/秒，与 tc/htb 一致 */
```

---

## 7. 后端 REST API 与 eBPF 交互规范（强制）

### 7.1 通用约定

- 前缀：`/api/v1/`  
- 鉴权：Session Cookie + `X-API-Key`（与旧 nat-admin 一致）  
- 写操作：**先** 持久化 `state.json`，**再** 写 bpffs，**再** netlink（tc），任一步失败则回滚  

### 7.2 流量整形 — 添加/更新

**场景**：Web「QoS 策略」添加/修改网段模板或 `/32`（向导）。

| 操作 | REST | 后端必须执行 |
|------|------|----------------|
| 添加/更新网段 | `PUT /api/v1/shaper/profiles` body `{cidr,down,up,mask,device?}` | ① `bpf_map_update_elem(profile_lpm)` ② mirred CIDR 列表重建 ③ HTB/u32 子网类 |
| PCQ 向导提交 | `POST /api/v1/shaper/wizard` | 写 `profile_lpm` + mask /32 + 默认 rate + 绑定网卡 |
| **/32 单 IP** | 同上（CIDR 写 `/32`）或活跃池首包 + ringbuf | `host_exact` 由 BPF 首包创建；长期覆盖建议 wizard/profile |

> **已移除**（2026-05-22）：`PUT/DELETE /api/v1/shaper/hosts/{ip}` 与独立 VIP 页；请用 profile `/32` 或向导。

**关键**：【添加/更新】必须调用 **`bpf_map_update_elem`**，将 **IP/前缀（Key）** 与 **速率 Value（字节/秒）** 写入对应 Map。

### 7.3 流量整形 — 删除

| 操作 | REST | 后端必须执行 |
|------|------|----------------|
| 删除网段模板 | `DELETE /api/v1/shaper/profiles?cidr=` | ① **`bpf_map_delete_elem(profile_lpm)`** ② 移除 mirred/u32 ② 空闲 GC `host_exact` |

**关键**：【删除】必须 **`bpf_map_delete_elem`** 实时擦除，不得仅删 JSON 不删 Map。

### 7.4 状态页 — 遍历 Map

**场景**：Dashboard /「状态 → eBPF 限速池」。

| API | 行为 |
|-----|------|
| `GET /api/v1/shaper/active` | **`Map.Iterate(active_host)`** → JSON 数组：`{ip, down_bps, up_bps, class_minor, bytes_down, bytes_up, last_seen}` |
| `GET /api/v1/shaper/profiles` | `state.json` profiles + 可选 map 对账 |
| `GET /api/v1/interfaces` | 网卡列表 + `traffic` + `traffic_history` + **`link_speed_mbps`** |
| `GET /api/v1/stats/dashboard` | 聚合：活跃数、总吞吐、RSS 队列、CPU、conntrack（§10.1） |

**关键**：状态 API **必须** 通过 **Iterate** 导出当前 Map 条目，供前端表格实时渲染（可配合 2s 轮询或 SSE）。

### 7.5 NAT 与 QoS 联动 — `skb->mark` 隔离（强制）

| 字段 | 用途 | 分配 |
|------|------|------|
| `skb->tc_classid` | **仅** HTB 选类（高 16 minor / 低 16 major） | BPF 分类程序写入 |
| `skb->mark` | **仅** nft 策略路由 / connmark（若启用） | nft 使用区间 **`0x00000000–0x0FFFFFFF`** |
| `skb->cb[]` / `tc_index` | IFB 重定向辅助（优先 **mirred**，不用 mark 做 IFB） | 避免与 nft 重叠 |

**规则**：

1. **IFB 上行** 使用 `TC_ACT_REDIRECT` / `mirred egress` 到 `ifb0`，**不** 用 mark 选 IFB。  
2. nft 规则 **禁止** `meta mark set` 覆盖 BPF 已设置的 `tc_classid`；如需打标，使用 `ct mark` 或限定 bit 掩码 `mark & 0x0FFFFFFF`。  
3. Outbound NAT（postrouting SNAT）**不得** 依赖与 QoS 相同的 mark 位。  
4. 文档化常量：`QOS_MARK_MASK = 0xF0000000` 保留给 QoS（若将来必须用 mark 时），nft **不得** 写入该位。

### 7.6 BPF 生命周期 API（运维）

| API | 说明 |
|-----|------|
| `POST /api/v1/ebpf/reload` | 重新 Load+Attach（维护窗口） |
| `GET /api/v1/ebpf/maps` | 列出 Pin 路径与 Map 统计 |
| `GET /api/v1/ebpf/programs` | 程序 attach 状态 |

---

## 8. 数据面：HTB + fq + IFB（执行面，与 Map 同步）

- Go 在 `bpf_map_update_elem` 之后，**同步或异步** 调用 netlink：  
  - `tc class add/change/del dev DEV_LAN`  
  - `tc class add/change/del dev ifb0`  
  - leaf：`fq_codel`（默认）或 `cake`  
- **idle_timeout**（如 300s）：GC 线程删除无流量 HTB 类，并 `bpf_map_delete_elem(active_host)`（保留 `host_exact` 配置）。  
- 规模：活跃 /32 数千～数万；**不** 预建全 10/8。

---

## 9. Nftables（NAT / 防火墙）

表 `inet qosnat`：prerouting DNAT、postrouting SNAT（static → prefix → policy 池）、forward、input。  
**无 flowtable**。  
ACL：forward/input 增加 drop/reject 规则（国家码、黑名单等二期）。

与 §7.5 mark 隔离协同测试。

---

## 10. Web UI — pfSense 风格菜单蓝图

**技术栈**：Vue 3 + Vite + TailwindCSS（灰蓝扁平 / 可选复古顶栏）；API 对接 `qosnatd`。

```
├── 仪表大厅 (Dashboard)
├── 常规设置 (System)
│   ├── General Setup
│   └── System Tunables (sysctl / fq 全局参数)
├── 接口 (Interfaces)
│   ├── 物理接口 / 多队列 / SR-IOV 状态
│   ├── 虚拟接口 (VLAN / IFB / tun / wg)
│   └── 接口分配 (LAN / WAN)
├── 防火墙 (Firewall)
│   ├── Aliases
│   ├── NAT / Port Forward / Outbound NAT
│   └── Rules (nft ACL)
├── 流量整形 (Traffic Shaper)          ← 核心
│   ├── QoS 策略（网段 + /32 向导）
│   └── 活跃 Per-IP 池
├── VPN
│   └── WireGuard（仅此；不支持 IPsec / OpenVPN）
└── 状态 (Status)
    ├── eBPF Map 监视器（Iterate）
    ├── 连接状态 (conntrack)
    ├── 接口 / 队列统计
    └── 抓包 (tcpdump)
```

### 10.1 仪表大厅 Widgets

| Widget | 数据来源 |
|--------|----------|
| **网卡 RSS 队列** | `/proc/interrupts`、`ethtool -S`、各队列 pps/bps |
| **软中断 (Softirq)** | `/proc/softirqs`、per-CPU |
| **eBPF 限速池** | `GET /api/v1/shaper/active` → 活跃 Per-IP 数量 + Top N |
| **系统信息** | CPU、内存、**Hugepages**（若启用 DPDK/大页）、uptime |
| **WAN/LAN 吞吐** | netlink stats / 历史环状图 |

### 10.2 常规设置 → System Tunables

可调项示例：`net.core.rmem_max`、`net.core.wmem_max`、`net.netfilter.nf_conntrack_max`、fq `flows`/`quantum`（通过 tc 全局或 sysctl 文档化）。  
UI 写 `state.json` + `sysctl -w` + 可选重启提示。

### 10.3 接口管理

| 功能 | 实现 |
|------|------|
| 物理口 / 多队列 | `ethtool -l/-L`、Netlink |
| SR-IOV / switchdev | 展示模式（能读则读）；100G ConnectX-6 Dx 调优说明 |
| **IFB** | 创建/删除 `ifb0`、关联「上行整形」开关 |
| VLAN / tun / wg | 基础配置入口（二期可深集成） |

### 10.4 防火墙

| pfSense 功能 | qosnat2 |
|--------------|---------|
| Port Forward | nft prerouting DNAT |
| 1:1 NAT | static_mappings |
| Outbound NAT | shared_ips + policy_routes + numgen |
| Rules | nft forward/input 链 |

### 10.5 流量整形（核心）

| 页面 | 说明 |
|------|------|
| **PCQ 向导** | 主网段、掩码 `/32`、默认 8mbit → `POST /shaper/wizard` → **`profile_lpm` + mirred CIDR** |
| **/32 覆盖** | profile CIDR `/32` 或活跃池 `host_exact`（最长前缀优先） |
| 列表删改 | 对应 §7.2 / §7.3 Map 操作 |

### 10.6 VPN

**WireGuard**：服务端、Peer、Conf 导出；**Peer 流量**（`wg show` transfer 每 5 分钟采样 + Web 曲线/实时）。不支持 IPsec / OpenVPN。

### 10.7 状态与诊断

| 功能 | 实现 |
|------|------|
| **eBPF Map 监视器** | Iterate `active_host` + `tc -s class`；展示 IP、rate、pacing、队列深度（若可取） |
| **States** | `conntrack -L` / netlink conntrack 计数 |
| **抓包** | 后端调 `tcpdump -i DEV_LAN -w /tmp/cap.pcap`，Web 下载 |

---

## 11. state.json（持久化，与 Map 对账）

```json
{
  "policy_routes": ["10.0.0.0/8"],
  "shared_ips": ["63.70.2.197"],
  "static_mappings": {},
  "prefix_mappings": {},
  "shaper": {
    "policy_cidr": "10.0.0.0/8",
    "default_profile": { "down": "8mbit", "up": "8mbit", "host_mask": 32 },
    "profiles": [],
    "hosts": { "10.0.18.83": { "down": "50mbit", "up": "50mbit" } },
    "leaf": "fq_codel",
    "idle_timeout_sec": 300
  },
  "firewall": { "wan_port_forwards": [], "rules": [] },
  "system": { "sysctl": {}, "hostname": "qosnat" },
  "api_keys": []
}
```

启动 `qosnatd`：**先** 回放 state → **批量 bpf_map_update_elem** → **tc 重建** → **nft 加载**。

---

## 12. 部署脚本（deploy-qos-nat.sh）

1. 依赖：iproute2、nftables、clang、llvm、libbpf-dev、**go**  
2. `modprobe ifb sch_htb sch_fq_codel cls_bpf act_bpf act_mirred`  
3. 创建 `ifb0`、HTB 根、`clsact`、Pin bpffs `/sys/fs/bpf/qosnat2/`  
4. 安装 `qosnatd`、静态资源 `web/dist`  
5. systemd：`qos-nat.service`（oneshot）、`qosnatd.service`（常驻）  

**禁止**：WAN 移入 netns；`deploy` 脚本 **必须** `readlink -f` 安装到 `/usr/local/bin/`。

---

## 13. 开发阶段（历史顺序，均已交付）

> 功能里程碑见 [`待开发清单.md`](待开发清单.md)（P1–P4 已完成）。下表保留作架构演进记录。

| 阶段 | 内容 | 验收依据 |
|------|------|----------|
| **P0** | `qosnatd` 骨架 + nft SNAT + forward | `acceptance-auto.sh`、`acceptance-check.sh` |
| **P1** | bpf Pin + `profile_lpm` / `host_exact` Map CRUD API | `GET /ebpf/maps`、bpftool pinned |
| **P2** | classify.bpf + ifb + HTB/fq + ringbuf 建类 | `acceptance-p2-mirred.sh`、`acceptance-p2-iperf.sh` |
| **P3** | Iterate `active_host` + Dashboard、VLAN 回滚、DHCPv6 | `acceptance-p3-smoke.sh`、`P3-dashboard.md` |
| **P4** | Vue 全模块 + 租户/VXLAN/ASN | `acceptance-p4-smoke.sh`、`P4-overlay.md` |
| **P5** | mark 隔离 + 多队列/RSS/ethtool | `GET /system/mark-policy`、`/interfaces/queues` |
| **P6** | WireGuard + tcpdump 抓包 | Web VPN/诊断页、`/api/v1/vpn/wireguard` |

---

## 14. 测试清单

> **与 [`待开发清单.md`](待开发清单.md) §14 同步**。自动化报告：[`验收报告-auto.md`](验收报告-auto.md)、[`验收报告-p2-iperf.md`](验收报告-p2-iperf.md)。

### 14.1 API / eBPF（已验收）

- [x] `POST /shaper/wizard` 或 profile `/32` 后 `host_exact` / `profile_lpm` 有对应 key（`acceptance-auto.sh`、P2 iperf）
- [x] `DELETE` 后 key 消失（wizard 清理用例）
- [x] `GET /shaper/active` 与 Map Iterate 一致（状态页 + API 冒烟）
- [x] 重启 `qosnatd` 后 Map 与 state 稳定（auto：`restart qosnatd profile_lpm stable`）

### 14.2 整形（已验收 / 部分人工）

- [x] 单 `/32` iperf 下行/上行 ≈配置值（±约 5%，见 `验收报告-p2-iperf.md`）
- [x] `/32` profile 覆盖默认网段速率（LPM + `host_exact`，P2 环境 20mbit 对账）
- [x] `100.64.0.0/24` profile + mirred 与 ~8M（`acceptance-p2-mirred.sh` + iperf）
- [ ] HTB `overlimits` 极少（Shaping 非 Policing）— 高负载时人工看 `tc -s class`

### 14.3 NAT / mark

- [x] `tc_classid` 与 `mark` 隔离（auto：LAN ingress 无 BPF、mirred→ifb0、Mark 策略页）
- [ ] Outbound NAT + 限速同时开启 **24h** — 见 [`P3-stability.md`](P3-stability.md)，需人工值守

### 14.4 旧项目回归 / 环境

- [x] 非对称回程 drop（nft `forward` 规则，auto：`nft table qosnat`）
- [x] 显式 `DEV_LAN`/`DEV_WAN`（env + 引导写入，禁止写死网卡名）
- [x] 前端危险 `onclick` 引号约定（Vue 组件，见 §7 编码约束）

### 14.5 仍须专项（未勾 = 待验收，非缺代码）

- [ ] `10.0.0.0/8` policy 与 10.x 客户端 iperf 对账
- [ ] 多 WAN **failover**（两条 `wan_links` default，断 WAN 恢复）
- [ ] 24h 长稳（conntrack、活跃池、定时 iperf 子集）

```bash
# 推荐一键复验
set -a; source /etc/qosnat2/env; set +a; export QOSNAT_PASS="${ADMIN_PASS:-password}"
/opt/qosnat2/scripts/acceptance-auto.sh
/opt/qosnat2/scripts/acceptance-p2-iperf.sh    # 需 SSH 测试机
/opt/qosnat2/scripts/acceptance-p4-smoke.sh
```

---

## 15. 内核参数（默认）

```ini
net.ipv4.ip_forward = 1
net.ipv4.conf.all.rp_filter = 0
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.netfilter.nf_conntrack_max = 2097152
```

100G / ConnectX 场景按 §10.2 在 UI 调大。

---

## 16. 风险与决策

| 决策 | 理由 |
|------|------|
| Map 配置 + HTB 执行 | Map 供 API/UI；HTB 保证真 Shaping |
| cilium/ebpf 统一 BPF | 类型安全、Iterate、Pin 生命周期 |
| 禁止 policer SHOT | 需求明确 |
| mark 隔离 | 避免 nft 与 QoS 打架 |
| pfSense 式 UI | 运维习惯、模块清晰 |

---

## 17. 开发前检查清单（自检，2026-05-22 更新）

| # | 项 | 状态 | 说明 |
|---|-----|------|------|
| 1 | 控制面（Map）与数据面（HTB）职责分离 | ✅ | §2、§6、§8 |
| 2 | REST 强制 `update/delete/iterate` Map | ✅ | §7、`internal/ebpf` |
| 3 | mark / tc_classid 隔离 | ✅ | §7.5、验收 auto |
| 4 | IFB 上行 + egress 下行 | ✅ | §4、§8、mirred 验收 |
| 5 | LPM 网段 + /32 优先级 | ✅ | QoS 策略页、`profile_lpm` |
| 6 | pfSense 式菜单与 Dashboard | ✅ | `web/src/views/*` |
| 7 | RSS / ethtool / 多队列 | ✅ | 接口页、高级调优 |
| 8 | WireGuard / 抓包 | ✅ | P6 已交付 |
| 9 | state.json 与 Map 启动对账 | ✅ | §11、启动 `ApplyAll` |
| 10 | 旧 policer / netns 废弃 | ✅ | §3、`reference/` 只读 |
| 11 | OpenAPI 契约 | ✅ | [`api/openapi.yaml`](../api/openapi.yaml)、`GET /openapi.yaml` |
| 12 | 前端 `web/` | ✅ | Vue3 + Vite，部署需 `npm run build` |
| 13 | 与旧 `nat-admin` 关系 | ✅ | 已迁入 `cmd/qosnatd`，勿双轨 |
| 14 | 认证 / HTTPS | ✅ | Session、API Key 哈希、常规设置 TLS；见 [`API-AUTH.md`](API-AUTH.md) |
| 15 | 审计日志 | ✅ | `GET /api/v1/system/audit`、Web 审计页 |

**结论（现行）**：P0–P6 与 P1–P4 路线图**已交付**；新工作以 [`待开发清单.md`](待开发清单.md) 为准（§14 三项实机验收、远期 Unbound/HAProxy 等）。**勿**按本说明从零新建仓库；在现有 `/opt/qosnat2` 上增量开发，并先读 AGENT_PROMPT 维护版。

---

## 18. 参考

- 旧仓库：`https://github.com/hk59775634/qosnat`  
- `reference/`：旧部署与 policer  
- cilium/ebpf、Linux HTB、IFB、nftables  

---

**文档版本**：2026-05-22 rev3（§13–§17 与验收状态同步）  
**维护**：架构准则 + 历史阶段记录；**任务优先级**以 [`待开发清单.md`](待开发清单.md) 为准。
