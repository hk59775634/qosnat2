# qosnat2 Bug 专项报告

**首轮日期**: 2026-05-30  
**第二轮复验**: 2026-05-31 · `v2026053101`  
**第四轮复验**: 2026-05-31 · 迭代完成后 **`v2026053103`**  
**第十三轮复验**: 2026-06-01 · 工作区 HEAD / catalog **`v2026060102`**

| Bug | 首轮 | 第四轮 |
|-----|------|--------|
| BUG-001 flush | P0 OPEN | **FIXED** |
| BUG-002 Terminal | P0 OPEN | **ACCEPTED**（应急 root + grant） |
| BUG-003 NAT revert | P1 OPEN | **FIXED** |
| BUG-004 NatStack | P1 OPEN | **FIXED**（rollback + 基线） |
| BUG-005 nft race | P1 OPEN | **FIXED** |
| BUG-006 atomic write | P2 OPEN | **FIXED** |
| BUG-008 import | — | **FIXED**（`commitStateImport`） |
| BUG-009 Save silent | P2 OPEN | **FIXED**（含 tuning PUT） |
| BUG-010 Shaper wizard | — | **FIXED**（F-030） |

**方法**: 静态代码审计 + 边界/并发/失败路径推演  
**说明**: Redis/MySQL 非运行时组件，N/A

---

## 场景模拟说明

| 场景 | 结论 |
|------|------|
| 1万 / 5万 / 10万 VPN 用户 | 瓶颈在单节点 conntrack、ocserv/WG 进程、tc 分类数，非 API QPS；见 PRODUCTION_READINESS |
| Redis 断开 | **N/A** — 核心路径无 Redis |
| MySQL 断开 | **N/A** — 核心路径无 MySQL |
| 网卡重启 | `applyWanLinkDataPlane` / WARP reconcile 可能部分失败，需人工 reload |
| 系统重启 | 依赖 systemd + `deploy-qos-nat.sh` 恢复；state.json 损坏则 P2-1 |
| nftables 重载 | scoped `delete table inet qosnat` 后重建本表（BUG-001 已修复）；仍有全表 O(n) 延迟 |
| API 高并发 | nftApplyMu 串行化（BUG-005 已修复） |

---

## Bug 列表

### BUG-001 — **状态: FIXED**

| 字段 | 内容 |
|------|------|
| **严重级别** | P0 |
| **位置** | `internal/nft/nft.go:77-78` |
| **复验** | Render 输出 `delete table inet qosnat`；`nft_test.go` 断言无 `flush ruleset` |
| **修复方案** | ✅ 已实施 |

---

### BUG-002

| 字段 | 内容 |
|------|------|
| **严重级别** | P0 |
| **位置** | `internal/api/terminal_handlers.go:87-89` |
| **复现条件** | 已认证用户打开「诊断 → Terminal」WebSocket |
| **影响** | 完整 shell 访问；可改系统配置、读密钥、横向移动 |
| **修复方案** | 默认关闭；白名单命令；降权用户；操作审计与告警 |

---

### BUG-003

| 字段 | 内容 |
|------|------|
| **严重级别** | P1 |
| **位置** | `internal/api/nat_handlers.go:46-47` 等（Save 后 reloadNft，无 check/revert） |
| **复现条件** | 提交非法或冲突的 NAT 映射（如错误 CIDR），或 nft 执行失败（语法/权限） |
| **影响** | `state.json` 已更新但内核 ruleset 为旧版或部分应用；UI 与真实流量不一致 |
| **修复方案** | 复用 `checkNftForState` + 备份 state 字段 + `reloadNftWith*Revert` |

---

### BUG-004

| 字段 | 内容 |
|------|------|
| **严重级别** | P1 |
| **位置** | `internal/api/nat_translation.go:52-66` |
| **复现条件** | 启用 NAT64，nft Apply 成功，随后 jool/unbound/dnsmasq 任一失败 |
| **影响** | 半激活 NAT64：客户端 DNS/翻译行为异常，难以从 UI 一眼看出 |
| **修复方案** | 分阶段 status 字段；失败 rollback nft+jool；或全部 check 后再 apply |

---

### BUG-005

| 字段 | 内容 |
|------|------|
| **严重级别** | P1 |
| **位置** | `internal/api/server.go:354-361`（无 mutex） |
| **复现条件** | 两个 API 客户端同时 PATCH 防火墙规则与 NAT 映射 |
| **影响** | 两次 `flush ruleset` + apply 交错，最终 ruleset 对应最后一次完成者，中间修改可能丢失 |
| **修复方案** | 全局 `nftApplyMu` 或 apply 队列 |

---

### BUG-006

| 字段 | 内容 |
|------|------|
| **严重级别** | P2 |
| **位置** | `internal/store/store.go:244` |
| **复现条件** | Save 过程中 kill -9 qosnatd 或磁盘满 |
| **影响** | `state.json` 截断，重启后配置丢失或 JSON 解析失败 |
| **修复方案** | atomic rename；启动时校验 + 自动从 `.bak` 恢复 |

---

### BUG-007

| 字段 | 内容 |
|------|------|
| **严重级别** | P2 |
| **位置** | `internal/api/egress_handlers.go:170-175` |
| **复现条件** | 修改 egress 策略后 Save 成功，`reloadNft` 失败 |
| **影响** | 策略路由与 filter 规则不同步 |
| **修复方案** | 与 firewall handler 相同 revert 模式 |

---

### BUG-008 — **状态: OPEN（新 · 2026-05-31）**

| 字段 | 内容 |
|------|------|
| **严重级别** | P1 |
| **位置** | `system_state_handlers.go` import 路径 |
| **复现条件** | 导入 state 后 `reloadNft` 或 `applyNatStack` 失败 |
| **影响** | 磁盘 state 已替换，内核仍为旧配置或半配置 |
| **修复方案** | import 前 backup；apply 失败 revert + 再 apply |

---

### BUG-009 — **状态: OPEN（新 · 2026-05-31）**

| 字段 | 内容 |
|------|------|
| **严重级别** | P2 |
| **位置** | ocserv/WG/routes 等 ~14 handler 文件 |
| **复现条件** | 磁盘满或权限导致 `Save()` 失败 |
| **影响** | API 返回 200，内存 state 与磁盘不一致 |
| **修复方案** | 统一 `persistState(w)` 返回 500 |

---

### BUG-010（首轮）— Shaper Save 先于 dataplane

| 字段 | 内容 |
|------|------|
| **严重级别** | P2 |
| **位置** | `internal/api/shaper_handlers.go` |
| **复验** | **PARTIAL** — wizard 路径 reload 失败仍仅 log（F-030） |
| **修复方案** | apply 成功后再 persist；revert 对齐 firewall 模式 |

---

### BUG-011（首轮 Save 静默）— **大部分 FIXED**

| 字段 | 内容 |
|------|------|
| **严重级别** | P2 |
| **位置** | 原 `nat_handlers.go` 等 `_ = srv.store.Save()` |
| **复验** | 显式 discard **0 处**；~62 处 log-only 仍 OPEN（见 BUG-009） |

---

### BUG-010

| 字段 | 内容 |
|------|------|
| **严重级别** | P2 |
| **位置** | `internal/api/auth.go:41-46`（sessions.json 非原子写） |
| **复现条件** | 并发 login/logout + 进程崩溃 |
| **影响** | 会话文件损坏，全体用户需重新登录或无法登录 |
| **修复方案** | 同 BUG-006 atomic write |

---

### BUG-011

| 字段 | 内容 |
|------|------|
| **严重级别** | P2 |
| **位置** | `deploy-qos-nat.sh:365`；`scripts/uninstall.sh:189-191` |
| **复现条件** | 执行 stop 或 uninstall |
| **影响** | 停止服务时清空主机全部 nft 规则（与 BUG-001 同源） |
| **修复方案** | 仅删除 `table inet qosnat` |

---

### BUG-012

| 字段 | 内容 |
|------|------|
| **严重级别** | P3 |
| **位置** | `internal/api/terminal_handlers.go:35-38`（Origin 空则允许） |
| **复现条件** | 非浏览器客户端无 Origin 头连接 WebSocket |
| **影响** | 略放宽 CSRF/跨站 WebSocket 防护 |
| **修复方案** | 要求 Origin 或 CSRF token；SameSite=Strict cookie |

---

### BUG-013

| 字段 | 内容 |
|------|------|
| **严重级别** | P3 |
| **位置** | `internal/store/store.go:220-224`（legacy unmarshal 忽略错误） |
| **复现条件** | 从旧版 state 升级，字段类型不匹配 |
| **影响** | 部分 legacy NAT 字段静默未迁移 |
| **修复方案** | Unmarshal 错误日志 + 启动自检报告 |

---

### BUG-014

| 字段 | 内容 |
|------|------|
| **严重级别** | P3 |
| **位置** | `internal/nft/nft_test.go` — 部分测试需 root |
| **复现条件** | CI 无 CAP_NET_ADMIN 运行 `CheckRuleset` |
| **影响** | 非特权 CI 跳过语法校验，回归风险 |
| **修复方案** | 区分 render 单元测试与集成测试；CI 可选 privileged job |

---

### BUG-015

| 字段 | 内容 |
|------|------|
| **严重级别** | P3 |
| **位置** | WireGuard 异常重连 — `internal/wg/` + `wireguard_instances_handlers.go` |
| **复现条件** | 接口 down/up 频繁切换 |
| **影响** | 需依赖手动 restart 或下一轮 reconcile；未见自动 exponential backoff 文档 |
| **修复方案** | 监听 netlink 事件触发 reconcile；UI 显示 last error |

---

## 已修复 / 非 Bug（审计确认）

| 项 | 说明 |
|----|------|
| Input 链 admin 被 wan-drop 覆盖 | 已通过 `SyncAutoFilterRules` 顺序修复 |
| applyNatStack 后 WARP NAT 丢失 | 已加 `reconcileWarpAfterNft` |
| 防火墙规则无 revert | firewall/forward/aliases 已加 pre-check |
| 别名 ASN 类型 | 已在 store/nft 拒绝 |
| 删除被引用别名 | 已返回 409 |

---

## 第十三轮新增

### BUG-011 — **状态: ACCEPTED**（F-041，产品决策）

保持 Web Terminal 为已认证管理员 root shell，不实施 `diagnostics_terminal_enabled` 门禁。

### BUG-012 — **状态: FIXED**（F-042）

已补全 `api/openapi.yaml` → `POST /api/v1/network/warp/license/apply`。

### BUG-013 — **状态: ACCEPTED**（F-043，产品决策）

`QOSNAT_TERMINAL_ALLOW_CIDRS` 未设置时继续允许任意客户端 IP。

### BUG-014 — **状态: FIXED**（F-045）

`ensureNetnsResolvFileAt` + 测试使用 `t.TempDir()`。

### BUG-015 — **状态: FIXED**（F-046）

`Store.Load` 在主文件缺失/损坏时从 `state.json.bak` 恢复并写回主文件。

---

## 优先级修复顺序建议（更新 2026-06-01）

第十三轮计划项已处理：BUG-012/014/015 **FIXED**；BUG-011/013 **ACCEPTED**（Terminal 产品策略）。
