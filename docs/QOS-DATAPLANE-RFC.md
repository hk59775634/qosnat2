# QoS 数据面重构（EDT + Per-IP Token Bucket）

**状态**：已实现（2026-06-11）  
**默认模式**：`shaper.mode = "edt"`  
**旧模式**：`shaper.mode = "htb"`（IFB + HTB + ringbuf，仅兼容保留）

## 目标

- Per-IP 8M/15M 语义：首包即生效，不依赖 userspace 建 HTB 类
- 去掉 IFB mirred 双跳，避免万人共享兜底队列
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
    "mode": "edt",
    "profiles": [
      {"cidr": "10.0.0.0/8", "down": "8mbit", "up": "8mbit", "mask": 32}
    ]
  }
}
```

- 省略 `mode` 或 `mode: "edt"`：新数据面
- `mode: "htb"`：恢复旧 IFB+HTB（不推荐生产）

## BPF 对象

| 文件 | 程序 |
|------|------|
| `bpf/rate_edt.bpf.o` | `rate_limit_ingress`, `rate_limit_egress` |
| `bpf/classify.bpf.o` | HTB 模式专用 |

安装路径：`/usr/lib/qosnat2/rate_edt.bpf.o`

Map：`profile_lpm`, `host_exact`, `throttle`, `token_bucket`（均 LRU/哈希，内核态 per-IP 状态）

## 构建

```bash
cd bpf && make && sudo make install
go test ./internal/store/ ./internal/shaper/ ./internal/ebpf/ ...
```

## 迁移

1. 现有生产若需 **保持旧行为**：在 `state.json` 写 `"mode": "htb"`
2. 默认升级至 edt 后：关闭 QoS 再开启，或 `systemctl restart qosnatd`，使 TC 拓扑重建
3. 验证：`bpftool map dump pinned /sys/fs/bpf/qosnat2/throttle | head`；`tc qdisc show dev ens19` 应为 `fq` 而非 `htb`

## 验收

- 1k+ 源 IP 同时活跃：无 `ifb0` 丢包累积、无 `htb: too many events`
- 单 IP iperf 达到 profile 速率 ±5%
- ping p99 不随 active IP 线性恶化

## 与 qosnat-vpp 关系

本改造使 qosnat2 在 Linux 上可承载高压 Per-IP QoS；VPP 路线仍可作为长期上限方案。
