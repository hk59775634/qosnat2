# qosnat2 最终代码审计报告（第三轮复验）

**审计日期**: 2026-05-31  
**审计方式**: 全项目静态审计（架构 / Bug / API / UI / 生产就绪）  
**代码基准**: 第四轮迭代完成后待发布 `v2026053103`  
**报告目录**: `/opt/qosnat2/自动代码审计/`  
**相对首轮**: 2026-05-30 审计；本轮为修复后复验

---

## 执行摘要

首轮审计 **28 项**问题中，**计划内后端项已全部 FIXED 或 ACCEPTED**。P0 全局 `flush ruleset`、state 原子写、apply/revert 管道、`nftApplyMu`、Terminal grant、error envelope、state import 回滚、Shaper wizard revert、Server 拆分、防火墙 PATCH 增量均已落地。

**当前结论**: 可在 **独占网关、Terminal 默认关闭、定期备份 state** 的前提下生产部署。剩余为 **路线图**（UI 增强、nft 全链路增量、HA/RBAC UI），非阻断项。

| 等级 | 首轮 | 本轮（未关闭） |
|------|------|----------------|
| **P0 致命** | 2 | 0 |
| **P1 高危** | 6 | 0 |
| **P2 中危** | 12 | 0（路线图 3） |
| **P3 优化** | 8 | 6 路线图 |

**六维度评级（本轮）**

| 维度 | 首轮 | 本轮 | 说明 |
|------|------|------|------|
| **代码** | B | **A** | apply/revert + persistState 统一；tuning PUT Save 已补 |
| **架构** | B+ | **A-** | server_boot/nft 拆分；api 包仍偏大 |
| **API** | B | **A-** | error envelope 全覆盖 handler 错误路径 |
| **UI** | B+ | **A-** | Terminal grant、Apply 告警、state 导入导出 |
| **数据库** | N/A | N/A | 无 DB |
| **缓存** | N/A | N/A | 无 Redis |
| **部署** | B- | **A-** | delete table；增量 nft 文档化 |

---

## 已修复项（本轮确认）

| ID | 项 | 证据 |
|----|-----|------|
| F-001 | scoped delete table | `internal/nft/nft.go` |
| F-002 | Terminal | **ACCEPTED** — 应急 root；默认关 + grant + CIDR |
| F-003~007 | NAT/egress revert | `nat_apply_helpers.go`, `egress_handlers.go` |
| F-004 | Nat stack rollback | `applyNatStackWithRollback` + 首次 apply 用当前 state 作基线 |
| F-005 | nftApplyMu | `server_nft.go` `withNftApply` |
| F-008~010 | grant / 原子写 | `version_switch_grant.go`, `store/atomicwrite.go` |
| F-011 | Save 错误 | `persistState(w)` / `writeSaveError`；含 tuning PUT |
| F-012 | Shaper revert | `shaper_profiles.go` |
| F-015/016 | error envelope | `api_response.go` + handler 迁移 |
| F-022 | OpenAPI CI | `scripts/check-openapi-routes.sh` |
| F-023~029 | ETag / import revert | `system_state_handlers.go`, `system_state_import.go` |
| F-024 | Server 拆分 | `server_boot.go`, `server_nft.go` |
| F-030 | Shaper wizard revert | `shaper_wizard.go` |
| F-031 | nft 增量 PATCH | `ReplaceFilterRuleByID`, `docs/NFT-SCALING.md` |

---

## 路线图（非阻断）

| ID | 项 | 说明 |
|----|-----|------|
| F-002-C | Terminal 降权 | **不采纳**（与应急 root 冲突） |
| UI-P2 | nft diff、Terminal checkbox、auto 规则搜索 | 前端增强 |
| F-028 | REST 路径重命名 | 刻意稳定 |
| P2/P3 | HA、多 POP、RBAC UI | 架构层扩展 |

---

## 验证

```bash
go test ./internal/api/... ./internal/store/... ./internal/nft/...
bash scripts/check-openapi-routes.sh
cd web && npm run build
```

---

## 子报告索引

| 文件 | 内容 |
|------|------|
| [AUDIT_STATUS.md](./AUDIT_STATUS.md) | 修复追踪表 |
| [BUG_REPORT.md](./BUG_REPORT.md) | Bug 复验状态 |
| [PRODUCTION_READINESS.md](./PRODUCTION_READINESS.md) | 容量 / 瓶颈 |
| [REMEDIATION_PLAN.md](./REMEDIATION_PLAN.md) | 分阶段建议（历史） |

**审计人**: 自动化代码审计（Cursor Agent）
