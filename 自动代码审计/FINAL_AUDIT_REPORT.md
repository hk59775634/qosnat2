# qosnat2 最终代码审计报告（第十三轮复验）

**审计日期**: 2026-06-01  
**审计方式**: 全项目静态审计（架构 / Bug / API / UI / 生产就绪）+ 关键路径代码走读 + `go test` / OpenAPI 路由检查  
**代码基准**: 工作区当前 HEAD（catalog 最新 `v2026060102`）  
**相对上轮**: 2026-05-31 第十二轮（`v2026053110`）

---

## 执行摘要

上轮计划内 **P0/P1 后端项仍保持有效**（scoped nft delete、原子写、`nftApplyMu`、NAT revert、state import 回滚等）。第十三轮：**Terminal 相关按产品决策不修改**；**F-042/045/046 已修复**。

| 等级 | 上轮未关闭 | 本轮新增 | 本轮合计待办 |
|------|------------|----------|--------------|
| **P0 致命** | 0 | 0 | **0** |
| **P1 高危** | 0 | 0（Terminal 已 ACCEPTED） | **0** |
| **P2 中危** | 0 | 0（F-042 FIXED；F-043 ACCEPTED） | **0** |
| **P3 优化** | 路线图若干 | 0（F-045/046 FIXED） | 0 |

**六维度评级（本轮）**

| 维度 | 上轮 | 本轮 | 说明 |
|------|------|------|------|
| **代码** | A | **A-** | 核心 apply/revert 仍健全；Terminal 开关未接线 |
| **架构** | A- | **A-** | 单机网关模型未变；无 DB/Redis |
| **API** | A- | **B+** | OpenAPI 缺 1 路由；health 与 terminal 状态不一致 |
| **UI** | A- | **B+** | Terminal 侧栏常驻、无启用开关与后端联动 |
| **数据库** | N/A | N/A | 无 DB |
| **缓存** | N/A | N/A | 无 Redis（外部 RADIUS 独立） |
| **部署** | A- | **A-** | `deploy-qos-nat.sh` 已无 `flush ruleset`；需 TLS + 备份清单 |

**生产结论**: 在 **TLS、强密码、定期异地备份 state** 前提下可部署。Web Terminal 为 **已认证管理员的 root shell**（设计如此）；可选 `QOSNAT_TERMINAL_ALLOW_CIDRS` 收紧来源 IP。

---

## 本轮新发现问题

### F-041 — Web Terminal 未强制执行 `diagnostics_terminal_enabled`（P1 → **ACCEPTED**）

> **产品决策（2026-06-01）**：保持 root shell，不实施此项修复。

### F-041（原描述，归档）

| 项 | 内容 |
|----|------|
| **等级** | P1 |
| **影响** | 配置项无法真正关闭 Terminal；任意持有会话/API Key 的管理员可随时获得 root shell，与 REMEDIATION「默认关」及 `store.System.DiagnosticsTerminalEnabled` 语义冲突 |
| **证据** | `internal/api/terminal_handlers.go` 仅检查 `requestAuthorized` + 可选 CIDR；**未读** `st.System.DiagnosticsTerminalEnabled`。`internal/api/server.go` `handleHealth` **硬编码** `"diagnostics_terminal_enabled": true`。`web/src/layouts/AppLayout.vue` 侧栏 **固定** Terminal 入口，无状态判断 |
| **修复位置** | `terminal_handlers.go`、`server.go`（health）、`AppLayout.vue` / `router/index.js`（可选路由守卫） |
| **修复方案** | 1) WS 握手前：`if !st.System.DiagnosticsTerminalEnabled { 403 }`；2) health/general 返回真实布尔值；3) 前端根据 health 或 general 隐藏入口；4) 高级设置恢复「启用 Web Terminal」开关（改开关需当前密码，与 TLS 同级） |

### F-042 — OpenAPI 与实现路由不一致（P2 → **FIXED**）

| 项 | 内容 |
|----|------|
| **等级** | P2 |
| **影响** | API 文档与 CI 检查漂移；集成方漏文档 |
| **证据** | `bash scripts/check-openapi-routes.sh` 报告 `MISSING: /api/v1/network/warp/license/apply` |
| **修复位置** | `api/openapi.yaml` |
| **修复方案** | 补充该 path 的 method、request/response schema |

### F-043 — Terminal IP 白名单默认放行全部客户端（P2 → **ACCEPTED**）

> **产品决策**：未设置 `QOSNAT_TERMINAL_ALLOW_CIDRS` 时继续不限制客户端 IP。

### F-043（原描述，归档）

| 项 | 内容 |
|----|------|
| **等级** | P2 |
| **影响** | 未设置 `QOSNAT_TERMINAL_ALLOW_CIDRS` 时，任意来源 IP（能连上管理口）在认证通过后均可开 Terminal |
| **证据** | `terminal_handlers.go` `terminalClientAllowed`：`raw == ""` 时 `return true` |
| **修复方案** | 生产文档强制设置 CIDR；或改为 **默认拒绝**、仅 localhost/管理网段白名单（破坏性变更需版本说明） |

### F-044 — `handleHealth` 误导前端与监控（P3 → **ACCEPTED**）

| 项 | 内容 |
|----|------|
| **等级** | P3 |
| **影响** | 依赖 health 的 UI/脚本误认为 Terminal 已启用 |
| **证据** | `server.go:273` 硬编码 `true` |
| **修复方案** | 与 F-041 一并改为 `st.System.DiagnosticsTerminalEnabled` |

### F-045 — warpnetns 单元测试环境敏感失败（P3 → **FIXED**）

| 项 | 内容 |
|----|------|
| **等级** | P3 |
| **影响** | `go test ./...` 在已安装 WARP 的机器上失败，CI 可能红 |
| **证据** | `TestEnsureNetnsResolvFileUsesCloudflareDNS` 期望 Cloudflare DNS，实际为 warp 生成的 `127.0.2.2` |
| **修复位置** | `internal/warpnetns/resolv_test.go` |
| **修复方案** | 测试使用 temp dir + mock 文件，或 `t.Skip` 当检测到 warp resolv 模板 |

### F-046 — 损坏 state 无启动时自动 `.bak` 恢复（P3 → **FIXED**）

| 项 | 内容 |
|----|------|
| **等级** | P3 |
| **影响** | `state.json` 损坏时需人工 `cp state.json.bak` |
| **证据** | `store.Save` 写 `.bak`；`Load` 未尝试恢复 |
| **修复方案** | `Load` 失败时尝试 `path+".bak"` 并打 audit 日志（见 REMEDIATION F-009 扩展） |

---

## 已确认仍有效的历史修复（抽样）

| ID | 项 | 证据 |
|----|-----|------|
| F-001 | scoped delete table | `internal/nft/nft.go` 无 `flush ruleset` |
| F-005 | nftApplyMu | `server_nft.go` `withNftApply` |
| F-008~010 | 会话原子写、登录限速 | `atomicwrite.go`、`security.go` loginLimiter |
| F-011 | persistState 统一 | `store_persist.go` |
| F-023~029 | state import 回滚 | `system_state_import.go` |
| F-031 | 防火墙 PATCH 增量 | `nft/incremental.go`、`QOSNAT_NFT_INCREMENTAL` |
| — | 抓包设备/过滤器校验 | `validateCaptureDevice`、`sanitizeTcpdumpFilter` |
| — | state 每次 Save 写 `.bak` | `store.go:254-255` |

---

## 数据库 / Redis

**N/A** — 产品核心无 SQL/Redis。OpenConnect RADIUS、外部 Redis 为认证侧独立组件，不在 qosnatd 进程内。

---

## 验证命令（本轮已执行）

```bash
go test ./internal/api/... ./internal/store/... ./internal/nft/...   # ok
go test ./internal/store/... ./internal/warpnetns/...                 # ok（含 .bak 恢复测试）
bash scripts/check-openapi-routes.sh                                  # ok
```

---

## 子报告索引

| 文件 | 内容 |
|------|------|
| [AUDIT_STATUS.md](./AUDIT_STATUS.md) | 修复追踪（含第十三轮） |
| [BUG_REPORT.md](./BUG_REPORT.md) | Bug 清单更新 |
| [PRODUCTION_READINESS.md](./PRODUCTION_READINESS.md) | 容量与部署（仍适用） |
| [ARCHITECTURE_AUDIT.md](./ARCHITECTURE_AUDIT.md) | 架构项历史记录 |

**审计人**: 自动化代码审计（Cursor Agent）
