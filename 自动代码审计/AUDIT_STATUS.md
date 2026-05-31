# 审计修复追踪（第二轮复验 + 第三轮迭代）

**更新日期**: 2026-05-31  
**代码基准**: 迭代完成后待发布

---

## 汇总

| 状态 | 数量 |
|------|------|
| **FIXED / ACCEPTED** | 26+ |
| **PARTIAL** | 2 |
| **OPEN (路线图)** | 3 |

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

## PARTIAL（剩余）

| ID | Item |
|----|------|
| F-015/016 | error `code` 未全覆盖所有 handler |
| F-030 | Shaper wizard reload 无 revert |

---

## OPEN（路线图）

| ID | Item |
|----|------|
| F-024 | Server 拆分 |
| F-031 | nft 全表 reload 规模化 |

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
