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
      {"cidr": "10.0.0.0/8", "down": "8mbit", "up": "8mbit", "mask": 32}
    ]
  }
}
```

省略 `mode` 即 EDT。从旧版升级时若 `state.json` 含 `"mode": "htb"`，加载后自动清除该字段。

## BPF 对象

| 文件 | 程序 |
|------|------|
| `bpf/rate_edt.bpf.o` | `rate_limit_ingress`, `rate_limit_egress` |

安装路径：`/usr/lib/qosnat2/rate_edt.bpf.o`

Map：`profile_lpm`, `host_exact`, `throttle`, `token_bucket`（均 LRU/哈希，内核态 per-IP 状态）

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
