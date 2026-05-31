# qosnat2 前端 UI 审计报告

**首轮日期**: 2026-05-30  
**第二轮复验**: 2026-05-31 · `3f67d44` / `v2026053101`

**复验摘要**: Terminal 红色警示 + 密码 grant 弹窗 **已做**；版本切换弹窗 **已做**；Apply 全局 sticky 告警 **已做**；防火墙 user-rule 搜索 **已做**。仍缺：风险 checkbox、state 备份向导 UI、全表搜索、nft diff。

**技术栈**: Vue 3 + Vite + Tailwind  
**参考产品**: pfSense / OPNsense / RouterOS / FortiGate

---

## 一、审计视角

| 视角 | 权重 | 摘要 |
|------|------|------|
| **用户体验** | 高 | 防火墙/NAT 表单较完整；深链与 nft 预览改善可运维性 |
| **运维体验** | 高 | Dashboard、诊断、版本切换可用；Terminal grant + 警示已加 |
| **VPN 运营商** | 中 | 多租户 shaper、ocserv/WG 管理具备；缺批量用户/计费联动 |

---

## 二、页面与流程评估

### 2.1 防火墙 (`FirewallRules.vue`)

| 项 | 评估 |
|----|------|
| 链/接口 Tab | ✓ 支持 `?chain=` `?iface=` `?rule=` 深链与 URL 同步 |
| 规则顺序 | ✓ iface 重排有警告文案 |
| wan-block 预设 | ✓ 有误导风险提示 |
| nft 预览 | ✓ 详情面板显示 `nft_lines` |
| 禁用规则 | ✓ 有确认对话框 |
| 对比 pfSense | 缺「快速复制规则」「拖拽排序持久化反馈」 |

### 2.2 别名 (`Aliases.vue`)

| 项 | 评估 |
|----|------|
| ASN 类型 | ✓ 已移除/拒绝 |
| 删除保护 | ✓ 409 errInUse 提示 |
| 对比 OPNsense | 缺 bulk import/export |

### 2.3 端口转发 (`PortForwards.vue`)

| 项 | 评估 |
|----|------|
| 与防火墙联动 | ✓ auto-fwd 列 + 跳转防火墙 |
| 说明文案 | ✓ nat/security locale 有 hint |
| 对比 FortiGate | 缺 hit count / 会话统计列 |

### 2.4 NAT / IPv6 / Outbound

- 功能页存在且与 API 对齐
- NAT64 多组件状态（jool/unbound）依赖 API status 字段，UI 信息密度高，新手学习曲线陡

### 2.5 QoS / Shaper

- 租户、档位、绑定 UI 较完整
- 缺「应用失败时 state 与 tc 不一致」的显式 banner（后端 BUG-008）

### 2.6 VPN (OCServ / WireGuard)

- 连接列表、组、证书选择具备
- 运营商场景：缺 RADIUS 用户 bulk、流量配额 UI（可能在 ocserv 外部系统）

### 2.7 系统 / 通用 (`General.vue`)

- 语言、时区、sysctl 等
- 版本切换需 grant token — 安全设计合理

### 2.8 诊断

- Terminal：**无醒目危险警告**（应默认折叠 + 二次确认）
- 抓包、日志：运维友好

---

## 三、表单与校验

| 项 | 状态 |
|----|------|
| 客户端 CIDR/端口校验 | ✓ `firewallRuleForm.js` |
| 与后端一致性 | 大体一致；极端 IPv6 格式依赖后端兜底 |
| 加载状态 | 多数页面有 loading；部分 PATCH 无按钮 disabled |
| 错误提示 | 统一 `error` toast；409/400 部分页面可更具体 |

---

## 四、权限管理（UI 层）

- **现状**: 登录门控 + 路由守卫；无角色 UI
- **问题**: 所有登录用户见全部菜单（单管理员假设）
- **建议**: 若引入 RBAC，按 nav 模块拆 `v-if="can('firewall')"`

---

## 五、功能缺失（相对竞品）

| 优先级 | 功能 | 说明 |
|--------|------|------|
| P1 | Terminal 危险操作门禁 | **FIXED** — Alert + grant + 风险 checkbox |
| P1 | Apply 失败全局提示 | **FIXED** — `useApplyAlert` + layout banner |
| P2 | 防火墙规则搜索/过滤 | **FIXED** — user/auto/builtin 均可搜 |
| P2 | 变更 diff 预览 | 保存前展示 nft diff |
| P2 | 配置备份/还原 UI | state.json 导出导入向导 |
| P3 | 暗色主题一致性 | 部分组件硬编码色 |
| P3 | 移动端适配 | 网关管理多为桌面场景，低优先 |
| P3 | 多语言覆盖度 | en/zh 较全；技术字符串偶发未翻译 |

---

## 六、布局问题

| 问题 | 位置 | 建议 |
|------|------|------|
| 安全/NAT 菜单分散 | nav | 增加「策略向导」聚合 WAN 放通流程 |
| 详情面板占宽 | FirewallRules | 可折叠 nft 预览 |
| Dashboard 信息密度 | Dashboard.vue | 关键告警（nft apply fail、WARP down）置顶 |

---

## 七、优化方案

### 7.1 短期（低成本）

1. Terminal 页增加 **高风险警告** 与「我了解风险」checkbox
2. API 写操作失败时 **统一 EventBus 告警**（含 revert 提示）
3. 防火墙列表 **按描述/端口搜索**

### 7.2 中期

1. **规则向导**：WAN 口只开放 443 + VPN 端口（对标 pfSense wizard）
2. **链接化文档**：表单项旁 ? 链到 `docs/API-ZH.md` 锚点
3. **乐观锁**：编辑规则时 If-Match / version 冲突提示

### 7.3 长期（运营商）

1. 只读 NOC 视图
2. 租户级 QoS 报表
3. 变更审批工作流（submit → approve → apply）

---

## 八、近期改进（已具备）

- 防火墙 chain/tab 深链与 URL 同步
- wan-block / orderHint 本地化
- 端口转发 ↔ 防火墙 auto 规则联动
- 别名删除占用提示
- disable rule 确认

---

## 九、结论

UI **达到可运维网关管理台水平**，中英文与模块覆盖优于多数自研脚本方案。与 pfSense/OPNsense 差距主要在 **策略向导、hit 统计、配置修订工作流**。优先补齐 **Terminal 警示** 与 **apply 失败可见性**，再 investment 搜索/diff 类效率功能。
