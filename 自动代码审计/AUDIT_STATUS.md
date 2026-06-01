# 审计修复追踪（第十三轮复验）

**更新日期**: 2026-06-01  
**代码基准**: catalog `v2026060102` / 工作区 HEAD

---

## 第十三轮（Terminal 开关未生效 · 2026-06-01）

| ID | 项 | 状态 |
|----|-----|------|
| F-041 | Terminal 未检查 `DiagnosticsTerminalEnabled` | **ACCEPTED** — 产品要求保持 root shell，不限制开关 |
| F-042 | OpenAPI 缺 `/api/v1/network/warp/license/apply` | **FIXED** — `api/openapi.yaml` |
| F-043 | Terminal CIDR 默认放行 | **ACCEPTED** — 保持不限制客户端 IP |
| F-044 | health 硬编码 terminal enabled | **ACCEPTED** — 随 F-041 不修复 |
| F-045 | warpnetns resolv 测试环境敏感 | **FIXED** — `ensureNetnsResolvFileAt` + 临时目录测试 |
| F-046 | Load 未自动从 `state.json.bak` 恢复 | **FIXED** — `store.loadLocked` / `store_load_test.go` |

---

## 第十二轮（Terminal 导航 + 常规保存 · v2026053110）

| 项 | 状态 |
|----|------|
| Terminal 移至系统→高级设置下 | **CHANGED** — 左侧栏固定入口 |
| 常规保存导致 qosnatd 退出 | **FIXED** |

---

## 汇总

| 状态 | 数量（本轮相关） |
|------|------------------|
| **OPEN** | 0 |
| **ACCEPTED（按产品决策）** | 3（F-041/043/044） |
| **FIXED（本轮）** | 3（F-042/045/046） |
| **FIXED / ACCEPTED（历史）** | 30+ |

---

## 第十三轮修复说明（2026-06-01）

- Terminal 相关（F-041/043/044）：按产品决策 **不修改**，保持 root shell 与无 IP 白名单默认。
- 已落地：OpenAPI、warpnetns 测试、state `.bak` 启动恢复。

---

## 验证

```bash
go test ./internal/api/... ./internal/store/... ./internal/nft/...
bash scripts/check-openapi-routes.sh
# 修复 F-041 后：默认 state 下 Terminal WS 应 403
```
