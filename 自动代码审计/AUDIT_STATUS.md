# 审计修复追踪（第二轮复验 + 第三轮迭代）

**更新日期**: 2026-05-31  
**代码基准**: 已发布 **`v2026053108`**

---

## 第十轮（版本切换卡住 · v2026053108）

| 项 | 状态 |
|----|------|
| 多次切换后任务永久 running | **FIXED** — 重启前先写入 ok；进程重启后 reconcile 陈旧 running |
| 无法再次切换 | **FIXED** — stale running 自动判定 ok/failed + `switch/reset` 清除 |

---

## 第九轮（版本切换弹窗 · v2026053107）

| 项 | 状态 |
|----|------|
| 确认切换后弹窗不关闭 | **FIXED** — 成功时直接 reset 弹窗，不再被 submitting  guard 拦截 |
| 任务状态与弹窗不同步 | **FIXED** — 关闭后立即 `applyVersionSwitchJob` + 防重复提交 |

---

## 第八轮（防火墙 Apply UX · v2026053106）

| 项 | 状态 |
|----|------|
| 应用变更需点两次 | **FIXED** — apply 响应返回最新 `changes`，一次 Apply 即清除 pending |
| 错误提示英文/原始 | **FIXED** — 合规 issue 与 API 错误 i18n 通俗文案 |
| 流程像三步 | **FIXED** — 去掉 Apply 确认框与重复 stagedOk 提示 |

---

## 第七轮（pfSense 两步应用 · v2026053105）

| 项 | 状态 |
|----|------|
| 防火墙两步走 | **FIXED** — pending 草稿 + 合规审核 + Apply/Discard |
| 405 envelope | **FIXED**（第六轮） |
| Terminal checkbox / 搜索扩展 | **FIXED**（第六轮） |

**代码基准**: 已发布 **`v2026053105`**

| 项 | 状态 |
|----|------|
| 405 → JSON envelope | **FIXED** — 全 handler `writeMethodNotAllowed` |
| Terminal 风险 checkbox | **FIXED** — grant 弹窗需勾选 |
| 防火墙搜索扩展 | **FIXED** — auto + builtin 规则纳入搜索 |

---

## 第五轮收尾（v2026053103）

| 项 | 状态 |
|----|------|
| tuning PUT 缺 Save | **FIXED** — `putSystemTuning` + `persistState` |
| ACME 错误 envelope | **FIXED** — `renewErrorResponse` 含 `code` |
| 审计文档同步 | **FIXED** — FINAL / BUG / API / PRODUCTION |

---

## 汇总

| 状态 | 数量 |
|------|------|
| **FIXED / ACCEPTED** | 30+ |
| **PARTIAL** | 0 |
| **OPEN (路线图)** | 0 |

---

## 第三轮迭代（本次）

| ID | Item | 状态 |
|----|------|------|
| F-002 | Terminal root shell | **ACCEPTED** — SSH 应急必须 root；默认关 + grant |
| F-004 | Nat stack rollback | **FIXED** — `applyNatStackWithRollback` + boot baseline |
| F-011 | Save 错误 | **FIXED** — mutating handler 统一 `persistState` |
| F-029 | State import | **FIXED** — `commitStateImport` 失败回滚 |
| F-022 | OpenAPI | **FIXED** — export ETag、import PUT/raw、version verify |

---

## 第四轮迭代（本次）

| ID | Item | 状态 |
|----|------|------|
| F-015/016 | error `code` envelope | **FIXED** — `writeAPIError` + helpers；handler 全量迁移 |
| F-030 | Shaper wizard revert | **FIXED** — `captureShaperWizardBackup` / `revertShaperWizard` |
| F-024 | Server 拆分 | **FIXED** — `server_boot.go` / `server_nft.go` |
| F-031 | nft 规模化 | **FIXED** — PATCH 增量 `ReplaceFilterRuleByID`；`docs/NFT-SCALING.md` |

---

## PARTIAL（剩余）

_无_

---

## OPEN（路线图）

_本轮计划项已全部落地；后续可按 HA/多节点等路线图另开项。_

---

## F-002 设计说明

Web Terminal ** deliberately ** 以 qosnatd 权限（通常 root）运行，用于 **SSH 不可用时的应急排障**，不作为日常管理入口。缓解措施：默认禁用、二次密码 grant、可选 `QOSNAT_TERMINAL_ALLOW_CIDRS`。**不实施**降权 shell（F-002-C）。

---

## 验证

```bash
go test ./internal/api/... ./internal/store/...
bash scripts/check-openapi-routes.sh
cd web && npm run build   # 或 sudo npm run build
```
