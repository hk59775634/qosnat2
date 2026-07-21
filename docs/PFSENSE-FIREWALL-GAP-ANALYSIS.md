# qosnat2 vs pfSense 防火墙功能差距分析（留档）

> **用途**：对照 pfSense Firewall / NAT / Aliases 模块，记录 qosnat2 已有能力与缺失项，供逐项评审是否纳入项目。  
> **生成日期**：2026-06-01  
> **最近评审**：2026-07-21（采纳建议合并项：日志/计数、Port/GeoIP/ASN、Schedule、Output、多 WAN 健康探测、规则绑 Gateway/Shaper）  
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
| Filter 规则 | `forward` + `input` + **`output`**；`accept` / `drop` / `reject`；`iif`/`oif`、协议、四元组、端口范围/别名、备注、启用/禁用、**log/counter** | 已齐 |
| 地址族 | 规则支持 `ipv4` / `ipv6`（`ip_version`） | |
| 默认策略 | forward/input 末尾 **default deny**；`established,related` 放行；output 默认 accept | |
| 自动规则 | 管理口/VPN 入站、`auto-fwd-*` 端口转发联动、WAN 按口 drop | |
| 草稿/合规 | Pending → Apply；变更审计（暴露管理口、WAN 宽放行等） | |
| 别名 | `ipv4_addr` / **`fqdn`** / **`asn`** / **`geoip`** / **`port`** | 已齐 |
| 时间表 | `firewall.schedules` + 规则 `schedule_id` | 已齐 |
| NAT | 端口转发 + hairpin、static 1:1、prefix SNAT、共享 IP 池、策略出站（egress + `ip rule`） | |
| IPv6 NAT | NPTv6、NAT64/DNS64（Jool） | |
| 多 WAN | metric/weight + **健康探测 failover**（`monitor_*` / `/network/wan-health`） | 已齐 |
| 诊断 | conntrack、抓包、nft 预览 / `rendered`、**防火墙日志与计数 API/UI** | 已齐 |
| API | OpenAPI + 仅 `firewall/*` 范围的 API Key | |

---

## 二、缺失功能清单（逐项评审）

### P0 — 规则模型与 pfSense 核心差距（建议优先）

| # | pfSense 能力 | qosnat2 缺口 | 说明 | 决策 | 备注 |
|---|-------------|-------------|------|------|------|
| 1 | 按接口规则页（WAN/LAN/OPT 分 tab） | ~~仅有统一列表~~ | Web 已有按接口 tab + Floating 筛选 | **采纳** | 已交付 |
| 2 | Floating Rules（全局、任意 hook 顺序） | 无独立 floating 链 | UI 有「无接口」筛选；非整链 floating | **延期** | 非核心 |
| 3 | Output 链规则 | ~~无~~ | nft `output` + UI 出站 tab | **采纳** | 已交付；默认 accept |
| 4 | 规则级 Logging | ~~无~~ | `log`/`counter` + `/firewall/logs` `/counters` | **采纳** | 已交付 |
| 5 | Schedules（时间表） | ~~无~~ | schedules API/UI + 规则绑定 | **采纳** | 已交付 |
| 6 | 规则绑定 Gateway | ~~无~~ | `wan_link_id` → 自动 EgressPolicy（无 skb mark） | **采纳** | 已交付 |
| 7 | 规则绑定 Limiter/Queue | ~~无~~ | `shaper_down`/`shaper_up` → profile `fw:<id>` | **采纳** | 已交付 |
| 8 | 端口别名（Port Alias） | ~~无~~ | `type=port` + `*_port_alias` | **采纳** | 已交付 |
| 9 | 源/目的端口范围、多端口 | ~~单端口~~ | `src_ports`/`dst_ports` | **采纳** | 已交付 |
| 10 | 规则反选（invert） | 无 `!` / negate | 无法「除某别名外全部 drop」 | **延期** | 低 ROI |

### P1 — 高级匹配与状态控制（pfSense Advanced）

| # | pfSense 能力 | qosnat2 缺口 | 决策 | 备注 |
|---|-------------|-------------|------|------|
| 11 | TCP flags（SYN/FIN/RST…） | 不支持 | **延期** | |
| 12 | State type（sloppy、none、synproxy 等） | 仅固定 `ct state established,related` | **延期** | |
| 13 | Max states / Max src nodes | 不支持（有全局 per-IP session limit） | **延期** | 已有会话上限 |
| 14 | Source tracking | 不支持 | **延期** | |
| 15 | Rate limiting / Limiter per rule | 见 #7 shaper 绑定 | **采纳** | 与 #7 合并交付 |
| 16 | Reply-To / Route-To | 不支持 | **延期** | 用 egress / wan_link_id |
| 17 | MAC 源地址匹配 | 不支持 | **延期** | |
| 18 | IP options / DSCP / VLAN PCP | 不支持 | **延期** | |
| 19 | 仅 ICMP 类型/代码 | 仅 `icmp`/`icmpv6` 协议级 | **延期** | |
| 20 | 规则 tag（用于日志关联） | comment + `qosnat2:rid:` | **采纳** | 用 rid 关联日志 |

### P2 — Aliases / 威胁情报（pfSense Aliases + 插件生态）

| # | pfSense 能力 | qosnat2 缺口 | 决策 | 备注 |
|---|-------------|-------------|------|------|
| 21 | Host / Network | ✅ `ipv4_addr` | **采纳** | 已有 |
| 22 | Port | ✅ `type=port` | **采纳** | 已交付 |
| 23 | URL Table（动态列表） | ✅ `url` 字段 + 5min 刷新 | **采纳** | 已有 |
| 24 | GeoIP | ✅ `type=geoip`（ipdeny） | **采纳** | 已交付 |
| 25 | ASN | ✅ `type=asn` + members/url | **采纳** | 已交付；无 live Whois |
| 26 | MAC / FQDN | FQDN ✅；MAC ❌ | **部分采纳** | FQDN 已有；MAC 延期 |
| 27 | pfBlockerNG 类（IP/域名黑名单、DNSBL） | ❌ | **拒绝** | 偏离定位 |
| 28 | 别名自动更新（BGP/Whois/Redis） | ❌ | **延期** | GeoIP/URL 已够用 |

### P3 — NAT 子模块（pfSense Firewall → NAT）

| # | pfSense 能力 | qosnat2 现状 / 缺口 | 决策 | 备注 |
|---|-------------|-------------------|------|------|
| 29 | Outbound 模式：Automatic / Hybrid / Manual / Disable | 由 `policy_routes` + egress 自动生成，无显式模式 | **延期** | |
| 30 | Outbound 规则独立排序/UI | SNAT 与策略路由耦合，无逐条 outbound 表 | **延期** | |
| 31 | Port Forward 独立 filter 关联 | 自动生成 `auto-fwd-*`，用户不可单独编辑 | **延期** | |
| 32 | Disable NAT 某网段 | 有 egress `NoSNAT` | **采纳** | 已有 |
| 33 | Pure NAT / NAT + Proxy ARP（1:1 VIP） | 有 static mapping + IP Alias；无 CARP | **部分采纳** | CARP 拒绝 |
| 34 | FTP / PPTP / GRE 等 NAT 辅助 | ❌ | **延期** | |
| 35 | IPv6 防火墙规则与 NPT 联动 UI | NPTv6 有；IPv6 filter 可用 | **延期** | 体验项 |

### P4 — 虚拟 IP / HA / 状态同步

| # | pfSense 能力 | qosnat2 缺口 | 决策 | 备注 |
|---|-------------|-------------|------|------|
| 36 | Virtual IPs（CARP、IP Alias、Proxy ARP） | IP Alias ✅；CARP/Proxy ARP ❌ | **部分采纳** | CARP **拒绝** |
| 37 | pfsync 连接状态同步 | ❌ | **拒绝** | 见 HA-DEPLOYMENT 冷备 |
| 38 | 配置段同步到备机 | 仅有 state 导入导出 | **延期** | |
| 39 | Gateway Groups + 监控（dpinger、故障切换） | ✅ WanLink monitor + failover | **采纳** | 已交付 |

### P5 — 安全服务（pfSense Packages，防火墙周边）

| # | pfSense 能力 | qosnat2 缺口 | 决策 | 备注 |
|---|-------------|-------------|------|------|
| 40 | IDS/IPS（Snort/Suricata） | ❌ | **拒绝** | |
| 41 | Captive Portal | ❌ | **拒绝** | |
| 42 | Block private / bogon 向导 | ❌ | **延期** | 可用别名手工 |
| 43 | Unbound DNS 策略与防火墙联动 | dnsmasq 为主；Unbound 仅 DNS64 | **拒绝** | 完整 Unbound 不做 |
| 44 | HAProxy 反向代理与 ACL | ❌ | **拒绝** | 远期可选，非本期 |

### P6 — 运维、审计、权限（防火墙运维体验）

| # | pfSense 能力 | qosnat2 缺口 | 决策 | 备注 |
|---|-------------|-------------|------|------|
| 45 | Filter 日志查看器（按接口/动作/源 IP） | 有最近日志页；筛选能力弱 | **部分采纳** | 基础已交付 |
| 46 | pfTop / 实时规则命中 | 有 counters API/UI | **部分采纳** | |
| 47 | 规则命中计数器 | ✅ `counter` | **采纳** | 已交付 |
| 48 | RBAC（按菜单/只读防火墙编辑） | 仅 admin / readonly / firewall API Key | **延期** | |
| 49 | 规则版本 / diff / 回滚 | 有 pending apply，无历史版本 | **延期** | |
| 50 | 与手写 `nftables-qosnat.nft` 共存 | Apply 会覆盖 | **延期** | 已知限制 |

---

## 三、建议实施路线图（参考，非承诺）

```
已交付（2026-07-21）
    → P0：规则日志/计数、Port 别名、端口范围、Output、Schedule、按接口 UI
    → P0/P2：Gateway/Shaper 绑定、GeoIP/ASN/FQDN、多 WAN failover
后续可选
    → Floating 真链、规则 invert、高级 TCP/状态字段
    → 日志筛选增强、规则版本
明确拒绝
    → IPsec/OpenVPN、IDS、Captive、CARP/pfsync、完整 Unbound、HAProxy（本期）
```

| 阶段 | 内容 | 状态 |
|------|------|------|
| P0 | 规则 `log` + counter → 日志页；Port alias；端口范围；Output；Schedule | **已交付** |
| P1 核心 | 单规则 gateway/shaper；多 WAN 健康探测 | **已交付** |
| P2 别名 | GeoIP/ASN/FQDN | **已交付** |
| 延期 | Floating 真链、invert、TCP flags… | 待产品决策 |
| 拒绝 | IDS、Captive、CARP/pfsync、IPsec/OpenVPN、完整 Unbound、HAProxy | 定位边界 |

---

## 四、刻意不在 pfSense 对标范围内

以下在 `docs/待开发清单.md` 等文档中已声明，**一般不视为防火墙漏做**：

| 项 | 说明 |
|----|------|
| IPsec / OpenVPN | 仅 WireGuard + ocserv |
| 完整 SD-WAN | 单宿主机双网卡；有 FRR BGP/OSPF 可选 |
| Package 市场、RRD 报表、NetFlow | 可观测性另线（Prometheus 已有） |
| 多节点 SDN / EVPN 控制器 | 远期 |
| Suricata / Captive Portal / CARP | 2026-07-21 评审 **拒绝** |

---

## 五、评审汇总（填写）

| 统计 | 数量 |
|------|------|
| 缺失项合计（#1–#50） | 50 |
| 已采纳 / 已交付 | 约 22（含部分采纳） |
| 已拒绝 | 约 8（IDS/Captive/CARP/pfsync/Unbound/HAProxy/pfBlocker 等） |
| 延期 | 其余高级匹配与运维增强 |
| 调研中 | 0 |

**评审人**：qosnat2 产品线  
**评审日期**：2026-07-21  
**下次复审**：按需（Floating / invert / 高级匹配）

---

## 六、相关文档

- [待开发清单.md](待开发清单.md)
- [UI开发建议.md](UI开发建议.md) — pfSense UI 对照
- [API-ZH.md](API-ZH.md) — 防火墙 REST
- [单机双网卡-QoS-NAT-开发说明.md](单机双网卡-QoS-NAT-开发说明.md) — §9 nft ACL 二期
- [HA-DEPLOYMENT.md](HA-DEPLOYMENT.md)
- [P4-overlay.md](P4-overlay.md) — ASN 别名规划
- OpenAPI：`/api/v1/firewall/schedules|logs|counters`、`/api/v1/network/wan-health`

---

## 七、结论摘要

qosnat2 在 **基础状态防火墙 + NAT + 端口转发 + 默认拒绝** 上已具备网关能力。相对 2026-06 差距清单，**运维刚需与别名表达力**已对齐一批：

- **已补**：Output、日志/计数、Schedule、Port/GeoIP/ASN/FQDN、按接口视图、规则绑 Gateway/Shaper、多 WAN 健康探测 failover  
- **仍延期**：真 Floating、规则 invert、高级 TCP/状态字段、规则版本  
- **明确不做**：IDS、Captive Portal、CARP/pfsync、IPsec/OpenVPN、完整 Unbound、HAProxy  

核心壁垒仍是 **Per-IP EDT/eBPF QoS + ocserv/WARP/LVS**，不以完整 pfSense 克隆为目标。
