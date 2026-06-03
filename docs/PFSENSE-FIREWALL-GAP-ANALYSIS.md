# qosnat2 vs pfSense 防火墙功能差距分析（留档）

> **用途**：对照 pfSense Firewall / NAT / Aliases 模块，记录 qosnat2 已有能力与缺失项，供逐项评审是否纳入项目。  
> **生成日期**：2026-06-01  
> **对照基准**：仓库代码与文档（`internal/store/firewall.go`、`internal/nft/nft.go`、`docs/待开发清单.md`、`docs/UI开发建议.md`）。  
> **产品定位**：单机双网卡 QoS + NAT 控制面，非 pfSense 完整克隆。

---

## 评审说明

- 每项可在「决策」列填写：`采纳` / `拒绝` / `延期` / `调研中`。
- 优先级 **P0–P6** 仅作建议排序，最终以产品路线为准。
- 「刻意不对标」一节为文档已声明范围，评审时可快速跳过。

---

## 一、已有能力（与 pfSense 部分对齐）

| 领域 | qosnat2 现状 | 决策 |
|------|----------------|------|
| Filter 规则 | `forward` + `input`；`accept` / `drop` / `reject`；`iif`/`oif`、协议、四元组、备注、启用/禁用 | |
| 地址族 | 规则支持 `ipv4` / `ipv6`（`ip_version`） | |
| 默认策略 | forward/input 末尾 **default deny**；`established,related` 放行 | |
| 自动规则 | 管理口/VPN 入站、`auto-fwd-*` 端口转发联动、WAN 按口 drop | |
| 草稿/合规 | Pending → Apply；变更审计（暴露管理口、WAN 宽放行等） | |
| 别名 | `ipv4_addr` 对象组（CIDR 列表） | |
| NAT | 端口转发 + hairpin、static 1:1、prefix SNAT、共享 IP 池、策略出站（egress + `ip rule`） | |
| IPv6 NAT | NPTv6、NAT64/DNS64（Jool） | |
| 诊断 | conntrack、抓包、nft 预览 / `rendered` | |
| API | OpenAPI + 仅 `firewall/*` 范围的 API Key | |

---

## 二、缺失功能清单（逐项评审）

### P0 — 规则模型与 pfSense 核心差距（建议优先）

| # | pfSense 能力 | qosnat2 缺口 | 说明 | 决策 | 备注 |
|---|-------------|-------------|------|------|------|
| 1 | 按接口规则页（WAN/LAN/OPT 分 tab） | 仅有统一列表 + `iif`/`oif` | 需接口维度视图/模板，降低误配 | | |
| 2 | Floating Rules（全局、任意 hook 顺序） | 无 | 无法实现跨接口 QoS bypass、全局 drop、IPS 前置等 | | |
| 3 | Output 链规则 | 无 `output` chain | 无法限制本机出站（除 input 间接约束） | | |
| 4 | 规则级 Logging | 无 `log` 动作/计数器 | 无防火墙拦截日志 UI、无法做「最近拦截」Dashboard | | |
| 5 | Schedules（时间表） | 无 | 无法按时段开关规则 | | |
| 6 | 规则绑定 Gateway | 出站走 egress 策略，非单条规则 `gateway` | 无法实现「仅此规则走 WAN2」类 pfSense 行为 | | |
| 7 | 规则绑定 Limiter/Queue | QoS 在 shaper，规则未挂 limiter | pfSense 可在 pass 规则上直接挂上传/下载限速 | | |
| 8 | 端口别名（Port Alias） | 仅 IP 别名 | 多端口/端口组无法在规则中引用 | | |
| 9 | 源/目的端口范围、多端口 | 单 `src_port`/`dst_port`（0=任意） | 无 `1024-65535`、端口列表 | | |
| 10 | 规则反选（invert） | 无 `!` / negate | 无法「除某别名外全部 drop」 | | |

### P1 — 高级匹配与状态控制（pfSense Advanced）

| # | pfSense 能力 | qosnat2 缺口 | 决策 | 备注 |
|---|-------------|-------------|------|------|
| 11 | TCP flags（SYN/FIN/RST…） | 不支持 | | |
| 12 | State type（sloppy、none、synproxy 等） | 仅固定 `ct state established,related` | | |
| 13 | Max states / Max src nodes | 不支持 | | |
| 14 | Source tracking | 不支持 | | |
| 15 | Rate limiting / Limiter per rule | 不支持（与 shaper 分离） | | |
| 16 | Reply-To / Route-To | 不支持 | | |
| 17 | MAC 源地址匹配 | 不支持 | | |
| 18 | IP options / DSCP / VLAN PCP | 不支持 | | |
| 19 | 仅 ICMP 类型/代码 | 仅 `icmp`/`icmpv6` 协议级 | | |
| 20 | 规则 tag（用于日志关联） | 有 comment，无独立 tag/规则号体系 | | |

### P2 — Aliases / 威胁情报（pfSense Aliases + 插件生态）

| # | pfSense 能力 | qosnat2 缺口 | 决策 | 备注 |
|---|-------------|-------------|------|------|
| 21 | Host / Network | ✅ `ipv4_addr` | | 已有 |
| 22 | Port | ❌ | | |
| 23 | URL Table（动态列表） | ❌ | | |
| 24 | GeoIP | ❌（开发说明 §9 标为二期） | | |
| 25 | ASN | 文档/P4 有规划，`NormalizeAlias` 仍拒绝 `asn` | | |
| 26 | MAC / FQDN | ❌ | | |
| 27 | pfBlockerNG 类（IP/域名黑名单、DNSBL） | ❌ | | |
| 28 | 别名自动更新（BGP/Whois/Redis） | ❌ | | |

### P3 — NAT 子模块（pfSense Firewall → NAT）

| # | pfSense 能力 | qosnat2 现状 / 缺口 | 决策 | 备注 |
|---|-------------|-------------------|------|------|
| 29 | Outbound 模式：Automatic / Hybrid / Manual / Disable | 由 `policy_routes` + egress 自动生成，无显式模式 | | |
| 30 | Outbound 规则独立排序/UI | SNAT 与策略路由耦合，无逐条 outbound 表 | | |
| 31 | Port Forward 独立 filter 关联 | 自动生成 `auto-fwd-*`，用户不可单独编辑 | | |
| 32 | Disable NAT 某网段 | 无细粒度「不 NAT」开关 | | |
| 33 | Pure NAT / NAT + Proxy ARP（1:1 VIP） | 有 static mapping，无 Virtual IP / CARP | | |
| 34 | FTP / PPTP / GRE 等 NAT 辅助 | ❌ | | |
| 35 | IPv6 防火墙规则与 NPT 联动 UI | NPTv6 有；IPv6 filter 与 v4 别名不一致 | | |

### P4 — 虚拟 IP / HA / 状态同步

| # | pfSense 能力 | qosnat2 缺口 | 决策 | 备注 |
|---|-------------|-------------|------|------|
| 36 | Virtual IPs（CARP、IP Alias、Proxy ARP） | ❌ | | |
| 37 | pfsync 连接状态同步 | ❌（见 `docs/HA-DEPLOYMENT.md`） | | |
| 38 | 配置段同步到备机 | 仅有 state 导入导出，无实时集群 | | |
| 39 | Gateway Groups + 监控（dpinger、故障切换） | 多 WAN 有 metric/weight，无 RTT/丢包探测与自动 failover | | |

### P5 — 安全服务（pfSense Packages，防火墙周边）

| # | pfSense 能力 | qosnat2 缺口 | 决策 | 备注 |
|---|-------------|-------------|------|------|
| 40 | IDS/IPS（Snort/Suricata） | ❌ | | |
| 41 | Captive Portal | ❌（待开发清单「pfSense 类扩展」远期） | | |
| 42 | Block private / bogon 向导 | ❌ | | |
| 43 | Unbound DNS 策略与防火墙联动 | dnsmasq 为主 | | |
| 44 | HAProxy 反向代理与 ACL | ❌ | | |

### P6 — 运维、审计、权限（防火墙运维体验）

| # | pfSense 能力 | qosnat2 缺口 | 决策 | 备注 |
|---|-------------|-------------|------|------|
| 45 | Filter 日志查看器（按接口/动作/源 IP） | 仅审计 API 操作，无数据面日志 | | |
| 46 | pfTop / 实时规则命中 | ❌ | | |
| 47 | 规则命中计数器 | ❌ | | |
| 48 | RBAC（按菜单/只读防火墙编辑） | 仅 admin / readonly / firewall API Key | | |
| 49 | 规则版本 / diff / 回滚 | 有 pending apply，无历史版本 | | |
| 50 | 与手写 `nftables-qosnat.nft` 共存 | 已知限制：Apply 会覆盖 | | |

---

## 三、建议实施路线图（参考，非承诺）

```
已有能力
    → P0：规则日志、Port 别名、端口范围、按接口 UI、Schedules
    → P1：Floating/Output、单规则 Gateway、TCP flags/状态限制
    → P2：GeoIP/ASN/FQDN、IDS/黑名单
    → P3+：Captive Portal、CARP/pfsync（需单独产品决策）
```

| 阶段 | 内容 | 粗估 |
|------|------|------|
| P0 | 规则 `log` + nft counter → 日志页；Port alias；端口范围；按 WAN/LAN 分组 UI | 3–4 周量级（参考） |
| P1 | Floating + Output；单规则 gateway/egress mark；Schedule | |
| P2 | GeoIP/ASN 别名；规则挂 shaper profile | |
| P3+ | IDS、Captive Portal、CARP/pfsync | 与「单机 QoS 网关」定位需决策 |

---

## 四、刻意不在 pfSense 对标范围内

以下在 `docs/待开发清单.md` 等文档中已声明，**一般不视为防火墙漏做**：

| 项 | 说明 |
|----|------|
| IPsec / OpenVPN | 仅 WireGuard + ocserv |
| 完整 SD-WAN / BGP / OSPF | 单宿主机双网卡 |
| Package 市场、RRD 报表、NetFlow | 可观测性另线 |
| 多节点 SDN / EVPN 控制器 | 远期 |

---

## 五、评审汇总（填写）

| 统计 | 数量 |
|------|------|
| 缺失项合计（#1–#50） | 50 |
| 已采纳 | |
| 已拒绝 | |
| 延期 | |
| 调研中 | |

**评审人**：________________  
**评审日期**：________________  
**下次复审**：________________

---

## 六、相关文档

- [待开发清单.md](待开发清单.md)
- [UI开发建议.md](UI开发建议.md) — pfSense UI 对照
- [API-ZH.md](API-ZH.md) — 防火墙 REST
- [单机双网卡-QoS-NAT-开发说明.md](单机双网卡-QoS-NAT-开发说明.md) — §9 nft ACL 二期
- [HA-DEPLOYMENT.md](HA-DEPLOYMENT.md)
- [P4-overlay.md](P4-overlay.md) — ASN 别名规划

---

## 七、结论摘要

qosnat2 在 **基础状态防火墙 + NAT + 端口转发 + 默认拒绝** 上已具备网关能力。相对 pfSense **Firewall** 模块，主要缺口集中在：

- **规则能力**：Floating/Output、日志与命中统计、Schedule、Port/GeoIP/ASN 别名、规则级 Gateway/Limiter、高级 TCP/状态/限速字段
- **高可用**：Virtual IP/CARP、网关健康探测与 failover
- **安全扩展**：IDS/黑名单/Captive Portal

逐项决策请使用上文 **「决策」「备注」** 列。
