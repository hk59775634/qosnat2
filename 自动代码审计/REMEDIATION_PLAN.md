# qosnat2 修复与优化建议

**依据**: [FINAL_AUDIT_REPORT.md](./FINAL_AUDIT_REPORT.md) 及分阶段审计报告  
**日期**: 2026-05-30  
**原则**: 保守修复 — 优先复用已有模式（如 `nft_apply_helpers.go`），每项变更附带回归测试；**避免「修一个 bug 引入两个 bug」**

---

## 一、修复策略总览

| 阶段 | 周期 | 目标 | 审计项 |
|------|------|------|--------|
| **0 — 运维缓解** | 立即 | 不改代码降低生产风险 | F-001/F-002/F-009 运维侧 |
| **1 — 生产安全** | 1–2 周 | 消除致命面 | F-001, F-002, F-005, F-009, F-010 |
| **2 — 配置一致性** | 2–4 周 | state 与内核对齐 | F-003~F-007, F-011, F-012 |
| **3 — 体验与可观测** | 按需 | API/UI/文档 | F-015~F-020 |
| **4 — 规模化** | 路线图 | 容量与架构 | P2/P3 优化项 |

---

## 二、阶段 0 — 无需改代码的运维缓解（立即可做）

在代码修复落地前，生产环境可先执行：

| 措施 | 针对问题 | 操作 |
|------|----------|------|
| **独占 nft 主机** | F-001 | 不在同一台机器运行 Docker CNI/firewalld/手工 nft；文档告知用户 |
| **禁用 Terminal** | F-002 | 反向代理层 block `/api/v1/diagnostics/terminal`；或防火墙仅允许管理 IP 访问 Admin 端口 |
| **state 定时备份** | F-009 | cron：`cp -a /var/lib/qosnat2/state.json /backup/state-$(date +%F).json` |
| **变更窗口** | F-001/F-005 | 避免脚本并发改防火墙+NAT；人工变更串行 |
| **TLS + 强密码** | F-002/F-017 | 强制 HTTPS；Session 30 天偏长，运维定期轮换密码/API Key |
| **NAT64 变更验证** | F-004 | 改 NAT64 后手动检查：`jool instance display`、`unbound-control status`、客户端 DNS |

---

## 三、阶段 1 — P0/P1 生产安全（优先开发）

### 3.1 F-001：缩小 nft flush 范围 【P0 · 高价值 · 需谨慎】

**问题**: `flush ruleset` 清空整机 nft 表。  
**位置**: `internal/nft/nft.go:78-79`，`deploy-qos-nat.sh`，`scripts/uninstall.sh`

**推荐改法（保守）**:

```nft
# 替换 flush ruleset，改为：
delete table inet qosnat
# 若 delete 失败（表不存在）则忽略，继续加载新表
```

**实现步骤**:

1. `Render()` 中将 `flush ruleset\n\n` 改为 `delete table inet qosnat\n\n`（表名用 `TableName` 常量）。
2. `Apply()` 加载前可选执行 `nft list table inet qosnat >/dev/null 2>&1 || true`，不依赖全局 flush。
3. 同步修改 `deploy-qos-nat.sh` stop、`scripts/uninstall.sh`：优先 `delete table`，仅在确认无其他 nft 依赖时再考虑 flush。
4. **保留** `reconcileWarpAfterNft()` — 作为防御性回补，不因 F-001 修复而删除。
5. 更新 `docs/SECURITY-HARDENING.md`：说明「qosnat2 管理 `table inet qosnat`」。

**风险与验证**:

| 风险 | 缓解 |
|------|------|
| 旧规则残留于同表内未覆盖链 | 当前 Render 为全表重写，delete table 后整表加载，行为与现逻辑一致 |
| iptables-nft 规则不在 inet qosnat | delete table **不会**再误删 iptables-nft（相对 flush ruleset 是巨大改进） |
| 首次安装表不存在 | delete 失败需 nft 脚本 tolerant（`-f` 或 ignore error） |

**验收**:

- [ ] 主机上预先 `nft add table inet foreign_test`，Apply 后该表仍存在
- [ ] 现有 `internal/nft/*_test.go` render 测试通过
- [ ] 有 root 的环境跑 `CheckRuleset` + 完整 Apply 冒烟
- [ ] WARP 连接状态下改防火墙，NAT 仍正常

**工作量**: 小（约 0.5–1 天）  
**回归优先级**: ⭐⭐⭐ 最高

---

### 3.2 F-002：Web Terminal 硬化 【P0】

**问题**: 认证通过即 root shell。  
**位置**: `terminal_handlers.go`，`server.go:194`

**推荐方案（分档，可叠加）**:

| 档位 | 改动 | 复杂度 |
|------|------|--------|
| **A — 默认关闭** | `state.System.DiagnosticsTerminalEnabled bool`，默认 `false`；handler 内检查 | 低 |
| **B — 二次确认** | 打开 Terminal 前 POST `/diagnostics/terminal/grant` 需重输密码或一次性 token（可复用 version switch grant 模式） | 中 |
| **C — 降权** | `cmd.SysProcAttr` 以 `nobody` 或专用用户启动；`rbash` + 只读 PATH | 中 |
| **D — 网络限制** | 环境变量 `QOSNAT_TERMINAL_ALLOW_CIDRS=10.0.0.0/8` | 低 |

**UI 配套（F-018）**:

- Terminal 页顶部红色 Alert：「等同于服务器 root 访问」
- 设置关闭时隐藏菜单入口

**验收**:

- [ ] 默认安装 Terminal API 返回 403
- [ ] 开启 grant 流程后可用；审计日志有 open/close
- [ ] 生产文档说明如何禁用

**工作量**: A+D 约 1 天；B+C 再加 1–2 天

---

### 3.3 F-005：nft Apply 全局互斥 【P1 · 低风险高收益】

**问题**: 并发 HTTP 请求交错 `nft.Apply`。

**推荐改法**:

```go
// server.go Server 结构体
nftApplyMu sync.Mutex

func (srv *Server) reloadNft() error {
    srv.nftApplyMu.Lock()
    defer srv.nftApplyMu.Unlock()
    // 现有逻辑不变
}
```

**扩展**: 所有直接调用 `nft.Apply` 的路径（`applyNatStack`、`applyWanLinkDataPlane`）应走 `reloadNft()` 或统一的 `withNftApply(fn)`，避免绕过锁。

**验收**:

- [ ] 并发测试：两 goroutine 同时 PATCH 不同资源，最终 ruleset 与最后一次成功 Save 一致
- [ ] 无死锁（Terminal/长耗时 apply 不嵌套持锁调用 API）

**工作量**: 小（约 0.5 天）

---

### 3.4 F-009 / F-010：原子写 【P2 提升为阶段 1】

**问题**: `state.json` / `sessions.json` 崩溃可截断。

**推荐改法** — 抽取 `internal/store/atomic_write.go`:

```go
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, ".tmp-*")
    // Write, Sync, Close, Rename
}
```

- `store.Save()` 与 `sessionStore.saveLocked()` 共用
- 可选：每次 Save 前复制 `state.json.bak`（仅保留 1 份）

**验收**:

- [ ] 单元测试：模拟 rename 前崩溃，原文件完整
- [ ] 启动 Load 失败时日志明确

**工作量**: 小（约 0.5 天）

---

## 四、阶段 2 — 配置一致性（对齐 firewall 已验证模式）

### 4.1 统一 Dataplane Apply Pipeline 【F-003/F-006/F-007/F-011】

**现状**: `firewall_handlers.go` / `forward_handlers.go` / `aliases_handlers.go` 已实现：

```
proposed state → checkNftForState → Save → reloadNft → 失败 revert → Save → reloadNft
```

**待对齐 handler**:

| 文件 | 备份字段 | 建议 helper |
|------|----------|-------------|
| `nat_handlers.go` | `Nat.IPv4.*`, `Nat.Nat64*` 等 | `reloadNftWithNatRevert(backupNat)` |
| `egress_handlers.go` | `Network.EgressPolicies` | `reloadNftWithEgressRevert` |
| `nat_v6_handlers.go` | `Nat.Nptv6*` | 同上或并入 Nat revert |

**实现模板**（与 firewall 相同，降低回归风险）:

1. `GET` 当前 state 片段 → `backup := clone...`
2. `Update` proposed → `proposed := srv.store.Get()`（或内存 proposed）
3. `checkNftForState(proposed)` 失败 → **400，不写盘**
4. `Save()` 失败 → **500，不 reload**
5. `reloadNft()` 失败 → revert + Save + reloadNft → 返回错误

**Save 错误**: 全局搜索 `_ = srv.store.Save()`，改为：

```go
if err := srv.store.Save(); err != nil {
    writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save failed"})
    return
}
```

**验收**:

- [ ] 故意提交非法 NAT CIDR → state 不变
- [ ] 模拟 nft load 失败（mock 或坏规则）→ state 回滚
- [ ] `go test ./internal/api/...` 增补 handler 测试

**工作量**: 中（约 2–3 天，逐 handler 合并）  
**建议顺序**: egress → nat_handlers → nat_v6 → shaper

---

### 4.2 F-004：`applyNatStack` 事务化 【P1 · 复杂度高】

**问题**: nft 成功、jool/unbound/dnsmasq 失败 → 半配置。

**推荐改法（保守两阶段）**:

**阶段 A — 可观测（先做，低风险）**:

- API 响应 / status 增加字段：`nat_stack: { nft: ok, jool: ok, unbound: ok, dnsmasq: ok, last_error: "" }`
- UI 展示「部分应用失败」banner（F-019）

**阶段 B — 回滚（后做）**:

```go
type natStackBackup struct {
    sysctl map[string]string
    // jool/unbound 可选 snapshot 或「禁用 NAT64」安全态
}
```

失败时：

1. 若 jool 失败 → 尝试 `jool.Apply(previousNat)` 或 disable
2. 若仍失败 → `reloadNft()` 用 backup state
3. 返回聚合 error

**不建议**: 第一版就做完整分布式事务；先 A 再 B。

**工作量**: A 约 1 天；B 约 2–3 天

---

### 4.3 F-012：Shaper apply 顺序 【P2】

**问题**: Save 先于 BPF/tc 成功。

**改法**:

1. 内存 `Update` proposed profile
2. 调用 shaper/BPF apply
3. 成功后再 `Save()`；失败则 `Update` revert 内存（不 Save）

**验收**: BPF 加载失败时 reload 页面仍为旧档位。

**工作量**: 中（约 1–2 天，需熟悉 shaper 路径）

---

## 五、阶段 3 — API / UI 优化

### 5.1 API 【F-015/F-016/F-008】

| 项 | 建议 | 优先级 |
|----|------|--------|
| 错误 envelope | 新增 `writeAPIError(w, code, httpStatus, errCode, msg)`，**旧格式并行**一版，避免一次破坏前端 | P2 |
| dry_run | `POST /firewall/rules?dry_run=1` 仅 `checkNftForState` | P2 |
| 高危确认 | 版本切换已有 grant；Terminal 复用同一机制 | P1 |
| 状态码矩阵 | 写入 `docs/API-ZH.md`：校验 400、nft 422、冲突 409 | P3 |

### 5.2 UI 【F-018/F-019/F-020/F-025】

| 项 | 建议 | 工作量 |
|----|------|--------|
| Terminal 警示 | 红色 Alert + 设置开关 | 小 |
| Apply 失败全局 toast | axios/fetch 拦截器检测 `nft ruleset invalid` / 500 after write | 小 |
| 配置导出/导入 | System 页：下载 state.json、上传需 confirm + API | 中 |
| 防火墙搜索 | 客户端 filter by 描述/端口/链 | 小 |
| 规则 diff 预览 | 依赖 dry_run API，中期 | 大 |

### 5.3 文档 【F-027/F-022】

- 运维手册：WG 断线、`applyWanLinkDataPlane` 手动触发
- CI：`openapi.yaml` 与 `server.go` 路由 diff（脚本级即可）

---

## 六、阶段 4 — 规模化与架构（路线图）

| 项 | 建议 | 何时做 |
|----|------|--------|
| nft 增量更新 | 单条规则 `nft insert` 而非全表 reload | 规则 >500 且 reload >1s 时 |
| Prometheus metrics | nft_reload_seconds、conntrack_count | 有监控栈时 |
| RBAC scoped API Key | read-only / firewall / admin | 运营商多运维时 |
| 拆 Server 上帝对象 | FirewallService + ApplyCoordinator | 新功能开发受阻时 |
| 10 万用户 | **多 POP 水平扩展**，非单实例优化 | 产品定位明确后 |

---

## 七、修复项对照表

| 审计 ID | 标题 | 阶段 | 工作量 | 风险 |
|---------|------|------|--------|------|
| F-001 | nft flush 范围 | 1 | S | 中（需充分测试） |
| F-002 | Terminal | 1 | S–M | 低 |
| F-005 | nftApplyMu | 1 | S | 低 |
| F-009/010 | 原子写 | 1 | S | 低 |
| F-003/006/007 | NAT/egress revert | 2 | M | 低（有模板） |
| F-011 | Save 错误 | 2 | S | 低 |
| F-004 | applyNatStack | 2 | M–L | 中 |
| F-012 | shaper revert | 2 | M | 中 |
| F-013 | WARP | 随 F-001 | — | 低（保留 reconcile） |
| F-014 | legacy 迁移 | 3 | S | 低 |
| F-015~020 | API/UI | 3 | M | 低 |
| F-021~028 | 优化 | 4 | 各异 | 低 |

S=小 M=中 L=大

---

## 八、建议开发顺序（单线程保守路线）

```
Week 1:  F-005 nftApplyMu → F-009 原子写 → F-002-A Terminal 默认关
Week 2:  F-001 delete table（充分测试）→ deploy/uninstall 脚本
Week 3:  F-003 egress revert → nat_handlers revert
Week 4:  F-007 nat_v6 → F-011 Save 检查 → F-004-A status 字段
Week 5+: F-012 shaper → F-004-B 回滚 → UI banner / 导出配置
```

每周末跑：

```bash
go test ./internal/store/... ./internal/nft/... ./internal/api/...
# 有 root 时
go test ./internal/nft/ -run CheckRuleset
```

---

## 九、不建议的做法

| 做法 | 原因 |
|------|------|
| 一次性重写所有 handler | 回归面过大 |
| 去掉 WARP reconcile | F-001 修复后仍需防御 |
| 全面改 API 响应格式（无兼容层） | 前端全量 break |
| 为 10 万用户优化单实例 nft | 架构上不可行 |
| 引入 Redis/MySQL 做 state | 与产品设计背离，复杂度高 |

---

## 十、已修复项 — 维护时注意

以下已在当前分支完成，**后续 refactor 勿回退**:

- Input 链顺序（`SyncAutoFilterRules`）
- `reconcileWarpAfterNft` 挂接
- firewall / forward / aliases 的 check + revert
- 别名校验与 409 删除保护
- UI 深链、wan-block 警告、端口转发联动

修改 `nft_apply_helpers.go` 或 `firewall_auto.go` 时，务必跑：

```bash
go test ./internal/store/ -run 'Auto|Filter|Alias'
go test ./internal/api/ -run Firewall
```

---

## 十一、相关文档

| 文档 | 用途 |
|------|------|
| [FINAL_AUDIT_REPORT.md](./FINAL_AUDIT_REPORT.md) | 问题全集 |
| [BUG_REPORT.md](./BUG_REPORT.md) | 复现条件 |
| [PRODUCTION_READINESS.md](./PRODUCTION_READINESS.md) | 容量与部署清单 |
| [API_REVIEW.md](./API_REVIEW.md) | 错误码与 envelope 设计 |
| [UI_REVIEW.md](./UI_REVIEW.md) | 前端优化清单 |

---

**下一步建议**: 若开始写代码，优先 **F-005 → F-009 → F-002-A → F-001** 四件；其中 F-001 单独 PR、充分冒烟后再合并。
