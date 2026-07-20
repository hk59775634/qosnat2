# QoS 数据面（EDT + Per-IP Token Bucket）

**状态**：已实现（2026-06-11）  
**唯一数据面**：EDT（`shaper.mode` 省略或 `"edt"`；旧 `"htb"` 在加载 state 时自动清除）

## 目标

- Per-IP 8M/15M 语义：首包即生效，内核态 per-IP 状态
- 无 IFB mirred 双跳，避免万人共享兜底队列
- **不创建 ifb0**；升级时自动清除 HTB 遗留 mirred 并删除 ifb0
- 突发大量 VPN 用户时不崩溃

## 数据路径

| 方向 | Hook | 机制 |
|------|------|------|
| 上行（客户端 → 公网） | 接口 **ingress** clsact | **Token bucket**（按源 IP，`up_bps`） |
| 下行（公网 → 客户端） | 接口 **egress** clsact | **EDT** + root **fq**（按目的 IP，`down_bps`） |

涉及接口：`DEV_LAN`、profile 绑定网卡、WireGuard `wg*`、ocserv `vpns*`（启用时自动挂接）。

## 配置

```json
{
  "shaper": {
    "enabled": true,
    "profiles": [
      {"cidr": "10.0.0.0/8", "down": "8mbit", "up": "8mbit", "mask": 32},
      {"cidr": "10.1.0.0/30", "down": "8mbit", "up": "8mbit", "mask": 30}
    ]
  }
}
```

省略 `mode` 即 EDT。从旧版升级时若 `state.json` 含 `"mode": "htb"`，加载后自动清除该字段。

**`mask`（host_mask）语义**：

| mask | 限速桶键 | 含义 |
|------|----------|------|
| `32` 或省略/`0` | 完整主机 IP | 每 IP 独立限速（默认） |
| `1`–`31` | `ip & mask` 网段地址 | 同前缀主机共享 `down`/`up` 配额 |

`cidr` 仍只决定 **谁匹配该策略**（LPM）；`mask` 决定匹配后 **如何聚合共享桶**。例如 `cidr=10.1.0.0/30` 且 `mask=30` 时，该 /30 内四台主机共用 8M；`mask=32` 时各自 8M。

下行 fq 的 `queue_mapping` 仍按 **原始主机 IP** 散列，保证同网段内流间公平。

## BPF 对象

| 文件 | 程序 |
|------|------|
| `bpf/rate_edt.bpf.o` | `rate_limit_ingress`, `rate_limit_egress` |

安装路径：`/usr/lib/qosnat2/rate_edt.bpf.o`

Map：`profile_lpm`, `host_exact`, `throttle`, `token_bucket`

- `profile_lpm` / `host_exact` 的 `rate_val` 含 `host_mask`（偏移 20）
- `throttle` / `token_bucket` 的键在 `host_mask<32` 时为聚合后的网段地址；值含累计 `bytes` 供观测采样
- `host_flow`：按原始主机 IP 记账，共享桶可展开成员；`ReplayState` 会清空 throttle/token_bucket/host_flow

**QoS 限速仅以 `profiles` 列表为准**；`default_profile` 不再参与 BPF。`policy_cidr` 仅用于 NAT/路由语义，不写入限速 map。

## 构建

```bash
cd bpf && make && sudo make install
go test ./...
```

## 迁移

1. 升级后重启 `qosnatd`；TC 拓扑重建为 fq + clsact BPF
2. `ifb0` 会被自动删除
3. 验证：`bpftool map dump pinned /sys/fs/bpf/qosnat2/throttle | head`；`tc qdisc show dev ens19` 应为 `fq`；`ip link show ifb0` 应不存在

## 验收

- 1k+ 源 IP 同时活跃：无 `ifb0` 丢包累积
- 单 IP iperf 达到 profile 速率 ±5%
- ping p99 不随 active IP 线性恶化

## 与 qosnat-vpp 关系

本改造使 qosnat2 在 Linux 上可承载高压 Per-IP QoS；VPP 路线仍可作为长期上限方案。
