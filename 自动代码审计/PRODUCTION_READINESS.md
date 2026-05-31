# qosnat2 生产环境就绪度审计

**审计日期**: 2026-05-30  
**假设硬件**: 8核16G / 32核64G / 100G 网卡  
**假设场景**: 大规模 VPN 用户（10万级）  
**部署模型**: 单节点网关（非 K8s 控制面集群）

---

## 一、架构边界声明

qosnat2 设计为 **单机 Linux 路由器/NAT 网关**：

- 状态：`state.json` 单文件，无 Redis/MySQL 集群
- 控制面：单进程 `qosnatd`
- 数据面：内核 nft + tc + BPF + ocserv/WG

**10 万 VPN 用户** 不是 10 万 API 用户，而是 **10 万并发/累计 VPN 会话** 压在同一网关的数据面上。

---

## 二、当前可承载用户数估算

| 维度 | 8C16G + 10G | 32C64G + 100G | 限制因素 |
|------|-------------|---------------|----------|
| **OpenConnect 用户** | ~2k–8k 并发* | ~5k–20k 并发* | CPU 加解密、conntrack、单进程 ocserv |
| **WireGuard 用户** | ~5k–15k 并发* | ~20k–50k 并发* | 内核 WG 效率优于 SSL VPN |
| **混合 10 万「注册用户」** | 需 **多网关分流** | 单台难承载 10 万并发 | 产品定位应为边缘 POP 每节点上限 |
| **防火墙规则数** | ~500 规则舒适区 | ~2000+ 需实测 | 每次全量 nft reload O(n) |
| **QoS 租户/档位数** | ~1k 租户级* | ~5k+* | BPF map / tc class 数量 |
| **API 管理操作** | 数十 QPS 足够 | 同左 | 非瓶颈 |

\* **粗算**，强烈依赖 cipher、流量模型、是否 NAT64、是否 full tunnel；**必须现场压测**（iperf + 真实客户端）。

### 结论性估算

| 场景 | 单节点建议上限 |
|------|----------------|
| 中小企业 SSL VPN | 500–3000 并发 |
| 运营商 WG 汇聚 | 5000–15000 并发（高配 + 调优） |
| **10 万 VPN 用户（并发）** | **需 5–20+ 台边缘网关** + 负载分担，**非单 qosnat2 实例目标** |

---

## 三、性能瓶颈分析

### 3.1 nftables 规模

| 问题 | 说明 |
|------|------|
| 全量 reload | 每次 `delete table inet qosnat` + 重载整表，延迟仍随规则线性增长 |
| 增量 update | **`QOSNAT_NFT_INCREMENTAL=1`** 时防火墙 forward/input 单条增删改可走 nft CLI 增量；失败回退全表 reload（见 `docs/NFT-SCALING.md`） |
| 10 万用户相关规则 | 若每用户一条 filter，不可行；应使用 ipset/聚合 alias |

**优化方向**: alias 聚合、减少 auto 规则膨胀；NAT/排序等仍全表 apply。

### 3.2 tc / QoS 规模

- eBPF + tc 适合 **租户/档位** 模型，不适合 per-user 千条 class
- 100G 网卡需确认 CPU 软中断、RPS/XPS、IFB 瓶颈

### 3.3 conntrack

- 10 万流表需调 `nf_conntrack_max`（`internal/tuning/conntrack.go` 有涉及）
- 内存：每连接 ~300B+，10 万 ≈ 30MB+ 仅表项，实际更高

### 3.4 API 规模

- 单 Go 进程，JSON 全量读写 state
- 管理 API 非高频；**不是 10 万用户场景的主瓶颈**

### 3.5 数据库 / Redis 压力

**N/A** — 核心产品不使用。外部 FreeRADIUS + Redis 为认证侧独立部署。

---

## 四、单点故障

| 组件 | SPOF | 缓解 |
|------|------|------|
| qosnatd | 是 | systemd restart；state 备份 |
| state.json | 是 | 原子写 + 异地备份 |
| 单网关数据面 | 是 | Anycast / DNS 分流 / 多 POP |
| ocserv | 是 | 双机冷备或 WG 为主 |
| WARP netns | 可选链路 | reconcile 逻辑已有 |

**无内置 HA 集群** — 生产需架构层多节点，非软件内建。

---

## 五、100G 网卡注意点

1. IRQ 亲和、多队列驱动
2. 避免 userspace 每包路径（ocserv 加密仍占 CPU）
3. nft 规则数保持精简；flow offload 视内核/驱动而定
4. 抓包诊断在高 PPS 下慎用

---

## 六、需要优化的项目（优先级）

### P0 — 生产阻断

| 项 | 内容 |
|----|------|
| P0-1 | 消除全局 `flush ruleset`（见 ARCHITECTURE P0-1） |
| P0-2 | 生产禁用或硬化 Web Terminal |
| P0-3 | state.json 原子写 + 自动备份 |

### P1 — 高负载前必须

| 项 | 内容 |
|----|------|
| P1-1 | `nftApplyMu` 串行化 apply |
| P1-2 | NAT/egress apply 与 firewall 同级 revert |
| P1-3 | conntrack/sysctl 调优文档化 + 启动自检 |
| P1-4 | 压测手册：ocserv/WG 并发、规则数阶梯 |

### P2 — 规模增长

| 项 | 内容 |
|----|------|
| P2-1 | nft 增量更新 | **PARTIAL** — filter CRUD 增量（`QOSNAT_NFT_INCREMENTAL`）；NAT/排序仍全表 |
| P2-2 | 规则/alias 数量监控与告警 |
| P2-3 | metrics 导出（Prometheus）：nft reload 耗时、conntrack usage |
| P2-4 | 读-only API replica（可选，仅读 state 副本） |

### P3 — 运营商化

| 项 | 内容 |
|----|------|
| P3-1 | 多节点配置同步（git/etcd）超出当前范围 |
| P3-2 | RBAC + 审计日志外送 SIEM |
| P3-3 | 租户级报表与配额 |

---

## 七、部署清单建议

- [ ] 独占主机或明确 nft 不与其他栈共存
- [ ] TLS + 强 admin 密码 + API Key 轮换
- [ ] Terminal 关闭或 IP 白名单
- [ ] `/var/lib/qosnat2` 定时备份
- [ ] conntrack / file descriptor limits 调优
- [ ] 变更窗口：nft reload 亚秒~数秒中断评估
- [ ] 10 万用户：**水平扩展 POP**，非垂直堆单实例

---

## 八、总结

| 问题 | 答案 |
|------|------|
| 能否单台支撑 10 万 VPN 并发？ | **不能**（现实预期：数千至万余取决于协议与硬件） |
| 当前最适合场景？ | 单站点/单 POP 边缘网关，千级并发，全功能 NAT+QoS+VPN |
| 最大生产风险？ | nft 全表 reload 规模（已 scoped delete + 部分增量）、Terminal 启用时 root |
| 达标路径？ | 先 P0/P1 稳定性 → 压测定容量 → 多节点分流达 10 万 |
