# qosnat2 最终代码审计报告（第二轮）

**审计日期**: 2026-05-31  
**审计方式**: 全项目静态审计（架构 / Bug / API / UI / 生产就绪）  
**代码基准**: `3f67d44`（Release `v2026053101`）  
**报告目录**: `/opt/qosnat2/自动代码审计/`  
**相对首轮**: 2026-05-30 审计；本轮为修复后复验

---

## 执行摘要

首轮审计 **28 项**问题中，**22 项已修复或基本达标**，**6 项仍为 PARTIAL/OPEN**。P0 全局 `flush ruleset` 与 state 原子写已解决；数据面 apply 管道（防火墙/NAT/egress）与 `nftApplyMu` 已统一；Terminal 与版本切换均增加二次密码 grant。

**当前结论**: 可在 **独占网关、Terminal 默认关闭、定期备份 state** 的前提下生产部署。剩余风险集中在 **Terminal 启用时的 root shell**、**部分 handler Save 失败仅打日志**、**state 导入无回滚**、**规模化 nft 全表 reload**。

| 等级 | 首轮 | 本轮（未关闭） | 本轮新增 |
|------|------|----------------|----------|
| **P0 致命** | 2 | 0（1 项降为 PARTIAL） | 0 |
| **P1 高危** | 6 | 3 PARTIAL + 1 OPEN | 1 |
| **P2 中危** | 12 | 5 PARTIAL/OPEN | 2 |
| **P3 优化** | 8 | 6 路线图 | 1 |

**六维度评级（本轮）**

| 维度 | 首轮 | 本轮 | 说明 |
|------|------|------|------|
| **代码** | B | **A-** | apply/revert 统一；Save 仍有 log-only 路径 |
| **架构** | B+ | **A-** | 模块清晰；Server 仍偏大 |
| **API** | B | **B+** | grant/ETag/scope；error envelope 未全覆盖 |
| **UI** | B+ | **A-** | Terminal grant、版本弹窗、Apply 告警 |
| **数据库** | N/A | N/A | 无 DB |
| **缓存** | N/A | N/A | 无 Redis |
| **部署** | B- | **B+** | delete table；运维文档已补 |

---

## 已修复项（本轮确认，不计入待办）

| ID | 项 | 证据 |
|----|-----|------|
| F-001 | 全局 nft flush → `delete table inet qosnat` | `internal/nft/nft.go:77-78`；`deploy-qos-nat.sh:365`；`scripts/uninstall.sh:186-188` |
| F-003 | NAT IPv4 check + revert | `nat_apply_helpers.go` `commitNatIPv4Change` |
| F-005 | nft apply 互斥 | `server.go` `nftApplyMu` + `withNftApply` |
| F-006 | Egress revert | `egress_handlers.go` + `reloadNftAfterEgressRevert` |
| F-007 | NAT64/NPTv6 stack commit | `commitNatStackChange` in `nat_v6_handlers.go` |
| F-008 | 版本切换 re-auth grant | `version_switch_grant.go` + General.vue 弹窗 |
| F-009/010 | state/sessions 原子写 | `store/atomicwrite.go` + `.bak` |
| F-012 | Shaper profile upsert revert | `shaper_profiles.go`（首轮） |
| F-014 | 防火墙 dry-run | UI + `?dry_run=1` |
| F-017 | API Key RBAC | `auth_scope.go` admin/readonly/firewall |
| F-018–020 | Terminal 警示、防火墙搜索、state 导入导出 | UI + handlers |
| F-021 | Terminal Origin 同源校验 | `terminal_handlers.go` |
| F-022 | OpenAPI 路由一致性 CI | `scripts/check-openapi-routes.sh` |
| F-023 | state export ETag | `system_state_handlers.go` |
| F-025 | 防火墙规则搜索 | `FirewallRules.vue` |
| F-026 | CI nft smoke | `.github/workflows/ci.yml` |
| F-027/028 | WG 文档 / URL 稳定 | `docs/WIREGUARD-SCALING.md` |
| — | DHCP/DNS 独立 + upstream | `store/dhcp.go`, `dnsmasq/` |
| — | input 链 auto 顺序、WARP reconcile、别名 409 | 首轮已确认 |

---

## 未关闭问题清单

### P0 — 致命（无 OPEN；1 项 PARTIAL）

#### F-002 Web Terminal 满权限 Shell 【ACCEPTED · 设计如此】

| 项 | 内容 |
|----|------|
| **等级** | 运维应急能力（非日常入口） |
| **设计** | **必须 root 权限** — 用于 SSH 服务异常时的补救；平时保持 `DiagnosticsTerminalEnabled=false` |
| **已做** | 默认关闭；`POST /diagnostics/terminal/grant` 二次密码；可选 `QOSNAT_TERMINAL_ALLOW_CIDRS`；UI 红色警示 |
| **不采纳** | F-002-C 降权 shell（与应急 root 需求冲突） |
| **位置** | `terminal_handlers.go` |

---

### P1 — 高危

#### F-004 applyNatStack 回滚 【PARTIAL】

| 项 | 内容 |
|----|------|
| **已做** | `lastNatStackSnapshot` + 分步 `rollbackNatStackDataplane`；`commitNatStackChange` 失败 revert |
| **残留** | 首次成功 apply 前 snapshot 为空，首次 mid-flight 失败可能回滚到「全禁用」而非上一运行态 |
| **位置** | `nat_stack_snapshot.go:26-35` |
| **修复** | 启动 `ApplyAll` 成功后 seed snapshot；或 apply 前 clone 当前 dataplane 状态 |

#### F-011 Save 错误处理 【PARTIAL】

| 项 | 内容 |
|----|------|
| **已做** | `_ = srv.store.Save()` **0 处**；防火墙/NAT/egress/DHCP 等关键路径 `persistState`/`saveState` |
| **残留** | **~62 处** `log.Printf("save state: …")` 后仍返回 200（ocserv、WG、routes、system、certs 等 **~14 个 handler 文件**） |
| **修复** |  mutating handler 统一 `if !srv.persistState(w) { return }` |

#### F-029 state 导入无 dataplane 回滚 【OPEN · 新】

| 项 | 内容 |
|----|------|
| **影响** | import 写盘成功但 `reloadNft`/`applyNatStack` 失败时 state 与内核不一致 |
| **位置** | `system_state_handlers.go:72-81`, `122-131` |
| **修复** | import 前 backup；apply 失败 revert state + 再 apply |

---

### P2 — 中危

#### F-015/016 统一 API 错误 envelope 【PARTIAL】

| 项 | 内容 |
|----|------|
| **已做** | `writeAPIError` + `code`；auth/state/nft 部分路径 |
| **残留** | `writeAPIError` **~10 处** vs 原始 `writeJSON` error **400+ 处** |
| **修复** | 按模块逐步迁移；OpenAPI 同步 `code` 枚举 |

#### F-022 OpenAPI 规范准确性 【PARTIAL】

| 项 | 内容 |
|----|------|
| **已做** | 路由脚本 91 条无 obvious gap |
| **残留** | `state/import/raw` 方法、version verify 描述、export ETag 等与实现不一致 |
| **修复** | 对照 `server.go` 手工修 openapi.yaml |

#### F-024 Server 上帝对象 【OPEN】

| 项 | 内容 |
|----|------|
| **影响** | 维护成本；非直接生产故障 |
| **位置** | `server.go` ~730 行；`internal/api/` ~170+ `(srv *Server)` 方法 |
| **修复** | 按子系统拆 package（路线图） |

#### F-030 Shaper wizard reload 无 revert 【OPEN · 新】

| 项 | 内容 |
|----|------|
| **位置** | `shaper_handlers.go` wizard 路径 `reloadNft()` 失败仅 log |
| **修复** | 对齐 `reloadNftWith*Revert` |

#### F-031 全表 nft reload 规模 【OPEN · 设计】

| 项 | 内容 |
|----|------|
| **说明** | 已 scoped delete table，但仍每次重建整表；`QOSNAT_NFT_INCREMENTAL` 仅 filter 增删 |
| **修复** | 压测 + 增量扩展（见 PRODUCTION_READINESS） |

---

### P3 — 优化 / 路线图

| ID | 项 | 状态 |
|----|-----|------|
| F-002-C | Terminal 降权 shell | 未做 |
| F-017+ | Session/UI RBAC、nav `v-if` | API Key 已覆盖自动化 |
| UI-P2 | nft diff 预览、备份向导 UI、端口转发统计、别名 bulk | 未做 |
| F-028 | REST 路径重命名 | 刻意稳定 |

---

## 六维度交叉矩阵（本轮）

| 问题 | 代码 | 架构 | API | UI | DB | 缓存 | 部署 |
|------|:----:|:----:|:---:|:--:|:--:|:----:|:----:|
| F-001 flush | ✓ | ✓ | | | | | ✓ |
| F-002 Terminal | △ | △ | ✓ | ✓ | | | ✓ |
| F-004 NatStack | △ | △ | ✓ | | | | |
| F-011 Save | △ | | △ | △ | ✓ | | ✓ |
| F-029 import | △ | △ | △ | | ✓ | | ✓ |
| F-015 envelope | | | △ | △ | | | |
| F-031 nft 规模 | ✓ | ✓ | | | | | ✓ |

图例：✓ 已缓解　△ 部分　空 不适用

---

## 修复路线图（剩余）

### 下一迭代（1–2 周）

1. F-011 — 剩余 handler `persistState`
2. F-029 — state import 事务化
3. F-004 — 启动 seed nat stack snapshot
4. F-022 — OpenAPI 与实现同步

### 按需

5. F-015 — error `code` 全覆盖  
6. F-002-C + Terminal CIDR 默认 deny  
7. UI：state 备份向导、Terminal 风险 checkbox  

---

## 子报告索引

| 文件 | 内容 |
|------|------|
| [ARCHITECTURE_AUDIT.md](./ARCHITECTURE_AUDIT.md) | 架构 / 逻辑 / 稳定性 |
| [BUG_REPORT.md](./BUG_REPORT.md) | Bug 编号与复验状态 |
| [API_REVIEW.md](./API_REVIEW.md) | REST / 错误码 / 认证 |
| [UI_REVIEW.md](./UI_REVIEW.md) | UX / 运维 / 竞品差距 |
| [PRODUCTION_READINESS.md](./PRODUCTION_READINESS.md) | 容量 / 瓶颈 / HA |
| [AUDIT_STATUS.md](./AUDIT_STATUS.md) | 修复追踪表 |
| [REMEDIATION_PLAN.md](./REMEDIATION_PLAN.md) | 分阶段修复建议 |

---

## 审计声明

- 静态审计；未做渗透测试与满载压测。
- Redis/MySQL 仅外部 RADIUS 文档，不计入运行时风险。
- 本轮 **更新** 审计报告以反映 `v2026053101` 源码；模板文件（`第一阶段`…`终极提示词.md`）未改。

**审计人**: 自动化代码审计（Cursor Agent）  
**版本基准**: `3f67d44` / `v2026053101`
