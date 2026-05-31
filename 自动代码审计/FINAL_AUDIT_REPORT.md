# qosnat2 最终代码审计报告

**审计日期**: 2026-05-30  
**审计方式**: 全项目静态审计（架构 / Bug / API / UI / 生产就绪）  
**代码变更**: 无（只读审计）  
**报告目录**: `/opt/qosnat2/自动代码审计/`

---

## 执行摘要

qosnat2 是功能完整的 **单节点 Linux 网关控制面**（NAT、nft 防火墙、QoS、OpenConnect、WireGuard、WARP、NAT64 等）。架构上 `store` + `nft` 分离合理，近期在防火墙 apply 管道、WARP 回补、UI 联动方面已有明显改进。

**审计共发现问题 28 项**（含已修复确认 5 项），其中：

| 等级 | 数量 | 必须行动 |
|------|------|----------|
| **P0 致命** | 2 | 上线前必须处理或书面接受风险 |
| **P1 高危** | 6 | 下一迭代优先 |
| **P2 中危** | 12 | 计划内修复 |
| **P3 优化** | 8 | 按需 |

**总体结论**: 可在 **受控生产环境（独占网关、禁用 Terminal、定期备份 state）** 部署；**不适合**在未修复 P0 的情况下与其他 nft/iptables 栈共存；**单实例不支持 10 万 VPN 并发**，需水平扩展 POP。

---

## 分维度摘要

| 维度 | 评级 | 关键发现 |
|------|------|----------|
| **代码** | B | apply 路径不一致；原子写缺失 |
| **架构** | B+ | 模块清晰；Server 上帝对象；全局 nft flush |
| **API** | B | 功能全；缺统一 error/transaction |
| **UI** | B+ | 防火墙/NAT 运维体验好；Terminal 无警示 |
| **数据库** | N/A | 不使用 DB |
| **缓存** | N/A | 不使用 Redis |
| **部署** | B- | stop/uninstall 全局 flush；无 HA |

---

## 全部问题清单

### P0 — 致命

#### F-001 全局 nft flush

| 项 | 内容 |
|----|------|
| **等级** | P0 |
| **影响** | 流量中断；第三方规则丢失；与容器/CNI 冲突 |
| **位置** | `internal/nft/nft.go:78-79` |
| **修复** | `delete table inet qosnat` 替代 `flush ruleset` |
| **关联** | `deploy-qos-nat.sh:365`，`scripts/uninstall.sh:189-191` |

#### F-002 Web Terminal 满权限 Shell

| 项 | 内容 |
|----|------|
| **等级** | P0 |
| **影响** | 认证突破即整机沦陷 |
| **位置** | `internal/api/terminal_handlers.go`，`server.go:194` |
| **修复** | 默认禁用 / 降权 / 白名单 / MFA |

---

### P1 — 高危

#### F-003 NAT handler 无 nft 预检与回滚

| 项 | 内容 |
|----|------|
| **等级** | P1 |
| **影响** | 配置丢失、NAT 失效、UI 与内核不一致 |
| **位置** | `internal/api/nat_handlers.go` |
| **修复** | 采用 `nft_apply_helpers.go` 同款 check + revert |

#### F-004 applyNatStack 部分失败

| 项 | 内容 |
|----|------|
| **等级** | P1 |
| **影响** | NAT64/DNS64 半配置 |
| **位置** | `internal/api/nat_translation.go:36-67` |
| **修复** | 分步 rollback 或全量 dry-run 后 commit |

#### F-005 并发 nft.Apply 竞态

| 项 | 内容 |
|----|------|
| **等级** | P1 |
| **影响** | 规则集不确定；自动化脚本冲突 |
| **位置** | `internal/api/server.go:354-361` |
| **修复** | `nftApplyMu sync.Mutex` |

#### F-006 egress handler 无 revert

| 项 | 内容 |
|----|------|
| **等级** | P1 |
| **影响** | 出口策略与 nft 不同步 |
| **位置** | `internal/api/egress_handlers.go` |
| **修复** | checkNftForState + revert |

#### F-007 nat_v6 handler 同类问题

| 项 | 内容 |
|----|------|
| **等级** | P1 |
| **影响** | IPv6 NAT/NPT 配置漂移 |
| **位置** | `internal/api/nat_v6_handlers.go` |
| **修复** | 统一 apply pipeline |

#### F-008 API 高危操作无二次确认

| 项 | 内容 |
|----|------|
| **等级** | P1 |
| **影响** | 误操作版本切换/Terminal |
| **位置** | `system_version_handlers.go`，terminal |
| **修复** | re-auth token / Confirm header |

---

### P2 — 中危

#### F-009 state.json 非原子写

| **位置** | `internal/store/store.go:233-244` |
| **修复** | temp + fsync + rename |

#### F-010 sessions.json 非原子写

| **位置** | `internal/api/auth.go:41-46` |
| **修复** | 同上 |

#### F-011 忽略 Save 错误

| **位置** | `nat_handlers.go` 等 `_ = srv.store.Save()` |
| **修复** | 检查返回值，失败 abort |

#### F-012 shaper Save 先于 BPF/tc

| **位置** | `internal/api/shaper_handlers.go` |
| **修复** | apply 后 save 或 revert |

#### F-013 WARP 依赖 flush 后回补

| **位置** | `internal/warpnetns/warpnetns.go`，`reconcileWarpAfterNft` |
| **修复** | 根因修复 F-001 + 保留 reconcile |

#### F-014 legacy 迁移静默失败

| **位置** | `internal/store/store.go:220-224` |
| **修复** | 记录 Unmarshal 错误 |

#### F-015 API 错误格式不统一

| **位置** | 各 handler `writeJSON` |
| **修复** | 统一 error envelope |

#### F-016 HTTP 状态码不一致

| **位置** | 多个 handler |
| **修复** | 状态码矩阵文档 + 重构 |

#### F-017 无 RBAC

| **位置** | `auth.go` 单管理员模型 |
| **修复** | scoped API Key / 角色 |

#### F-018 UI Terminal 无危险警示

| **位置** | 诊断 Terminal 页 |
| **修复** | 警告 + 默认关闭 |

#### F-019 apply 失败 UI 不可见

| **位置** | 全局 |
| **修复** | sticky alert + revert 提示 |

#### F-020 无配置备份 UI

| **位置** | system 模块 |
| **修复** | 导出/导入 state.json |

---

### P3 — 优化

| ID | 问题 | 位置/建议 |
|----|------|-----------|
| F-021 | Terminal Origin 空允许 | `terminal_handlers.go:35-38` |
| F-022 | OpenAPI 与路由漂移 | CI diff |
| F-023 | 无 ETag 乐观锁 | 防火墙并发编辑 |
| F-024 | Server 上帝对象 | 拆 domain service |
| F-025 | 防火墙无搜索 | UI 效率 |
| F-026 | nft 测试需 root | CI privileged job |
| F-027 | WG 链路 reconcile 文档不足 | 运维文档 |
| F-028 | URL API 命名混用 | 新路由 kebab-case |

---

## 已修复项（审计确认，不计入待办）

| 项 | 位置 | 说明 |
|----|------|------|
| ✅ | `internal/store/firewall_auto.go` | input 链顺序：auto accept 在 user input 前 |
| ✅ | `internal/api/server.go` | `reconcileWarpAfterNft` 挂接 reload |
| ✅ | `internal/api/nft_apply_helpers.go` | firewall/forward/alias pre-check + revert |
| ✅ | `internal/store/aliases.go` | 别名校验、删除引用 409 |
| ✅ | UI | 深链、wan-block 警告、端口转发联动、nft_lines |

---

## 六维度交叉矩阵

| 问题 | 代码 | 架构 | API | UI | DB | 缓存 | 部署 |
|------|:----:|:----:|:---:|:--:|:--:|:----:|:----:|
| F-001 flush | ✓ | ✓ | | | | | ✓ |
| F-002 Terminal | ✓ | ✓ | ✓ | ✓ | | | ✓ |
| F-003 NAT revert | ✓ | ✓ | ✓ | ✓ | | | |
| F-004 NatStack | ✓ | ✓ | ✓ | ✓ | | | |
| F-005 竞态 | ✓ | ✓ | ✓ | | | | |
| F-009 原子写 | ✓ | ✓ | | | ✓ | | ✓ |

---

## 修复路线图

### 阶段 1 — 生产安全（1–2 周）

1. F-001 + deploy/uninstall 脚本
2. F-002 Terminal 策略
3. F-009 / F-010 原子写
4. F-005 apply 互斥

### 阶段 2 — 一致性（2–4 周）

5. F-003 / F-006 / F-007 统一 apply pipeline
6. F-004 applyNatStack 事务
7. F-011 Save 错误处理
8. F-012 shaper revert

### 阶段 3 — 体验与规模化（按需）

9. F-015–F-020 API/UI 改进
10. F-021–F-028 优化项
11. 压测与容量文档（见 PRODUCTION_READINESS.md）

---

## 子报告索引

| 文件 | 内容 |
|------|------|
| [ARCHITECTURE_AUDIT.md](./ARCHITECTURE_AUDIT.md) | 架构 / 逻辑 / 稳定性 / 安全 |
| [BUG_REPORT.md](./BUG_REPORT.md) | Bug 编号、复现、修复 |
| [API_REVIEW.md](./API_REVIEW.md) | REST 设计、错误码、认证建议 |
| [UI_REVIEW.md](./UI_REVIEW.md) | UX / 运维 / 竞品差距 |
| [PRODUCTION_READINESS.md](./PRODUCTION_READINESS.md) | 容量估算、瓶颈、HA |

---

## 审计声明

- 本次为 **静态代码审计**，未在生产流量下进行 penetration test 或满载压测。
- Redis/MySQL 仅出现在外部 RADIUS 文档，**不计入** qosnat2 运行时风险。
- 审计 **未修改** 任何源代码；修复需单独变更与测试。
- 模板文件（`第一阶段`…`终极提示词.md`）保留不变；本报告系列为审计输出。

**审计人**: 自动化代码审计（Cursor Agent）  
**版本基准**: qosnat2 工作区当前 HEAD（含未提交 firewall/UI 改进）
