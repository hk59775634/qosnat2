# qosnat2 架构审计报告

**首轮日期**: 2026-05-30  
**第二轮复验**: 2026-05-31 · 基准 `3f67d44` / `v2026053101`

> **复验摘要**: P0-1（全局 flush）**已修复**；P1-1 NAT/egress revert **已修复**；P1-2 applyNatStack **部分修复**（首 snapshot 空）；P0-2 Terminal **部分缓解**（默认关 + grant + CIDR）。详见 [FINAL_AUDIT_REPORT.md](./FINAL_AUDIT_REPORT.md)。

**审计范围**: 全项目源码、配置、脚本、文档（只读）  
**项目**: qosnat2 — Linux 网关 NAT/QoS/VPN 控制面

---

## 一、项目概览

| 层级 | 路径 | 职责 |
|------|------|------|
| 守护进程 | `cmd/qosnatd/` | 进程入口、信号、环境变量 |
| HTTP API | `internal/api/` | REST 控制面、编排 nft/tc/BPF/ocserv/WG |
| 状态存储 | `internal/store/` | `state.json` 单一真相源 |
| nft 渲染 | `internal/nft/` | ruleset 生成与 `nft -f` 应用 |
| QoS | `internal/shaper/`, `internal/ebpf/` | tc + BPF 限速 |
| VPN | `internal/ocserv/`, `internal/wg/` | OpenConnect / WireGuard |
| WARP | `internal/warpnetns/` | netns 隔离与 NAT 回补 |
| 前端 | `web/` | Vue 3 + Vite 管理 UI |
| 部署 | `deploy-qos-nat.sh`, `scripts/` | 安装/卸载/验收 |

**数据流**: UI/API → `store.Update/Save` → `nft.Apply` / `jool.Apply` / `unbound.Apply` / `dnsmasq.Apply` / `shaper` / `policyroute` → 内核/用户态服务。

**外部依赖**: 本项目核心**不依赖 Redis/MySQL**；RADIUS Challenge 文档中提及 Redis 为外部 FreeRADIUS 体系建议，非 qosnat2 运行时组件。

---

## 二、架构评价

### 优点

1. **分层清晰**: `store`（模型+校验）与 `nft`（渲染）分离，API 层做编排，符合网关产品常见模式。
2. **单一状态文件**: `state.json` 便于备份/迁移/版本切换。
3. **自动规则机制**: `SyncAutoFilterRules` 统一管理 admin/VPN/WAN-drop/端口转发联动规则，减少手工配置错误。
4. **近期改进**: 防火墙/转发/别名 handler 已引入 `checkNftForState` + revert 模式；WARP 在 `reloadNft`/`applyNatStack` 后调用 `reconcileWarpAfterNft`；input chain 顺序已修正。

### 问题

---

## 三、按严重程度分类

### P0 — 致命

#### P0-1: 全局 `flush ruleset` 清空主机全部 nftables — **✅ 已修复（2026-05-31 复验）**

| 项 | 内容 |
|----|------|
| **位置** | `internal/nft/nft.go:77-78`；`deploy-qos-nat.sh:365`；`scripts/uninstall.sh:186-188` |
| **描述** | ~~每次 `nft.Apply` 执行 `flush ruleset`~~ 现为 `delete table inet qosnat`；卸载/stop 脚本已同步。 |
| **残留** | `deploy-qos-nat.sh` usage 文案仍写 “flush”（文档）；`warpnetns.go` 注释过时。 |
| **修复建议** | 更新 help 注释；压测 scoped delete 窗口。 |

#### P0-2: Web Terminal 授予完整 Shell — **△ 部分缓解**

| 项 | 内容 |
|----|------|
| **位置** | `terminal_handlers.go`；`terminal_grant_handlers.go`；`server.go:209-210` |
| **描述** | 默认关闭；连接前须 `POST /diagnostics/terminal/grant` 验证密码；可选 `QOSNAT_TERMINAL_ALLOW_CIDRS`。启用后仍为 PTY 全 shell。 |
| **影响** | 启用 + 凭据泄露时整机风险仍在。 |
| **修复建议** | 生产保持关闭；启用时强制 CIDR；F-002-C 降权 shell。 |

---

### P1 — 高危

#### P1-1: NAT/出口策略 handler 缺少 nft 预检与回滚 — **✅ 已修复**

| 项 | 内容 |
|----|------|
| **位置** | `nat_apply_helpers.go`、`egress_handlers.go`、`nat_v6_handlers.go` |
| **描述** | `commitNatIPv4Change`、`commitNatStackChange`、`commitEgressChange` 已统一 check → save → apply → revert。 |

#### P1-2: `applyNatStack` 部分失败无整体回滚 — **△ 部分修复**

| 项 | 内容 |
|----|------|
| **位置** | `nat_translation.go`；`nat_stack_snapshot.go` |
| **描述** | 分步 progress + `rollbackNatStackDataplane`；`commitNatStackChange` 失败 revert state。首次成功前 snapshot 为空。 |
| **修复建议** | 启动 seed snapshot；import 路径增加 backup/revert（F-029）。 |

#### P1-3: 并发 `nft.Apply` 无全局互斥 — **✅ 已修复**

| 项 | 内容 |
|----|------|
| **位置** | `server.go` `nftApplyMu`；`withNftApply` |
| **描述** | 所有 reload/applyNatStack 路径已串行化。 |

#### P1-4: `Server` 职责过重（上帝对象）

| 项 | 内容 |
|----|------|
| **位置** | `internal/api/server.go` + 35 个 `*_handlers.go` |
| **描述** | 单 `Server` 承载认证、nft、QoS、VPN、WARP、证书、诊断等全部编排，handler 间共享 mutable state。 |
| **影响** | 新功能易引入交叉副作用；测试与代码审查成本高；长期技术债务。 |
| **修复建议** | 按域拆 `FirewallService` / `NatService` / `VpnService` 等，Apply 管道统一入口（非紧急，但应规划）。 |

---

### P2 — 中危

#### P2-1: `state.json` 非原子写入

| 项 | 内容 |
|----|------|
| **位置** | `internal/store/store.go:233-244`；`internal/api/auth.go:41-46`（sessions.json 同理） |
| **描述** | 直接 `os.WriteFile`，进程崩溃或磁盘满时可能产生截断文件。 |
| **影响** | 配置丢失，服务无法启动或回退到空状态。 |
| **修复建议** | write temp + `fsync` + `rename`；可选定期备份 `.bak`。 |

#### P2-2: Shaper 先 Save 后应用 BPF/tc

| 项 | 内容 |
|----|------|
| **位置** | `internal/api/shaper_handlers.go` |
| **描述** | 与 NAT handler 类似，持久化先于 dataplane 成功确认。 |
| **影响** | QoS 配置与 tc 实际 class 不一致。 |
| **修复建议** | 应用成功后再 Save，或失败 revert。 |

#### P2-3: 错误处理不一致：`_ = srv.store.Save()`

| 项 | 内容 |
|----|------|
| **位置** | 多个 handler（如 `nat_handlers.go`）忽略 Save 错误 |
| **描述** | Save 失败仍继续 reloadNft，加剧 state/内核漂移。 |
| **修复建议** | Save 失败应 abort 并返回 500，不触发 apply。 |

#### P2-4: 模块间隐式耦合 WARP ↔ nft flush

| 项 | 内容 |
|----|------|
| **位置** | `internal/warpnetns/warpnetns.go` 注释；`reconcileWarpAfterNft` |
| **描述** | 架构上依赖「flush 后回补」而非避免 flush，耦合脆弱。 |
| **修复建议** | 见 P0-1；WARP 回补保留为防御性二次保障。 |

#### P2-5: Legacy 迁移路径复杂

| 项 | 内容 |
|----|------|
| **位置** | `internal/store/store.go` Load；`MigrateNatFromLegacy` |
| **描述** | 磁盘 JSON 多版本字段并存，`_ = json.Unmarshal` 忽略部分错误。 |
| **影响** | 升级后个别字段静默丢失。 |
| **修复建议** | 迁移单元测试 + 启动时校验报告。 |

---

### P3 — 优化建议

| 编号 | 建议 |
|------|------|
| P3-1 | 引入 API 版本前缀策略（当前 `/api/v1` 已有，OpenAPI 可再细化 deprecation） |
| P3-2 | `internal/nft` 与 `internal/store` 间 VPN 端口类型重复（`AutoInputVPN` vs `VPNFirewall`），可 code gen 或共享 struct |
| P3-3 | 验收脚本多但 CI 未全覆盖 dataplane（需 root/nft），建议 mock + 集成分层 |
| P3-4 | 前端与后端校验重复（防火墙表单），保持同步测试或 JSON Schema 共享 |
| P3-5 | 文档化「单节点边界」：非 HA 集群产品，避免用户误以为可水平扩展控制面 |

---

## 四、逻辑层专项

| 域 | 结论 |
|----|------|
| **NAT** | IPv4 静态/前缀映射、端口转发、hairpin、NPTv6 逻辑完整；NAT64 依赖 Jool+Unbound+dnsmasq 多组件，失败面大（见 P1-2） |
| **QoS** | eBPF + tc + IFB 架构合理；租户/档位模型在 store 中清晰 |
| **nftables** | 渲染与 Apply 分离良好；input 顺序（user fwd → auto fwd → auto input accept → user input → wan-drop）已修正 |
| **tc** | shaper 与 nft mark 隔离设计明确（注释禁止 meta mark set） |
| **WireGuard** | 多实例 + 流量采样；与防火墙 auto 规则联动 |
| **API/配置** | 单文件 state + session 文件；无 DB 事务但足够单节点场景 |

---

## 五、稳定性 / 性能 / 安全 / 部署摘要

- **Panic/nil**: handler 层多数有 JSON 边界；pty/websocket 有 defer 清理；未见明显 goroutine 泄漏（terminal 有 wg.Wait）。
- **性能**: 瓶颈在 nft 全量 reload 与 tc 重建，非 Redis/MySQL（未使用）；API 为单进程，适合单网关。
- **安全**: 无 SQL；命令执行多通过 `exec.Command` 固定路径；需关注 Terminal（P0-2）、session 固定 30 天 TTL、API Key 哈希存储（良好）。
- **部署**: `deploy-qos-nat.sh` stop 时 `flush ruleset` 风险同 P0-1；升级依赖 release catalog + state 迁移。

---

## 六、已修复项（本分支相对历史问题）

以下问题在审计前已有修复，**不再列为待办 P0**，但建议在变更说明中保留：

1. Input chain 规则顺序（`firewall_auto.go` / `SyncAutoFilterRules`）
2. WARP `reconcileWarpAfterNft` 挂接 `reloadNft` 与 `applyNatStack`
3. 防火墙/转发/别名 nft 预检 + revert（`nft_apply_helpers.go`）
4. 别名/端口/CIDR 校验增强；别名删除引用检查
5. UI：wan-block 警告、chain 深链、端口转发 ↔ 防火墙联动

---

## 七、总结

qosnat2 作为**单节点 Linux 网关控制面**，模块划分总体合理，store/nft 分离是正确架构选择。当前最大架构风险来自 **全局 nft flush** 与 **Web Terminal 满权限 Shell**；其次为 **dataplane 应用缺乏统一事务/互斥** 以及 **部分 handler 未对齐 revert 模式**。建议优先 P0/P1 项后再考虑大规模用户场景（见 `PRODUCTION_READINESS.md`）。

**审计结论**: 可用于受控生产环境（单网关、独占主机），需在部署规范中明确 nft 独占与 Terminal 策略，并继续统一 apply 管道。
