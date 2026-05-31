# qosnat2 API 设计审计报告

**首轮日期**: 2026-05-30  
**第二轮复验**: 2026-05-31 · `3f67d44` / `v2026053101`

**评分**: **8/10**（首轮 7/10）— grant/scope/ETag/CI 已补；error envelope 与 OpenAPI 细节仍待统一。

---

## 二、发现的问题

### P1 — 高危

| ID | 问题 | 位置 | 建议 |
|----|------|------|------|
| API-1 | **写操作缺乏统一事务语义** | NAT/egress/shaper | **FIXED**（firewall/NAT/egress）；import **OPEN** |
| API-2 | **破坏性操作无二次确认** | version/terminal | **FIXED**（grant 模式） |

### P2 — 中危

| ID | 问题 | 位置 | 建议 |
|----|------|------|------|
| API-3 | **HTTP 状态码不一致** | 部分校验失败用 400，部分用 500；别名占用 409（良好） | 文档化状态码矩阵并统一 handler |
| API-4 | **错误体格式不统一** | 多数 handler | **PARTIAL** — `writeAPIError` ~10 处 |
| API-9 | OpenAPI 与实现漂移 | openapi.yaml | **PARTIAL** — CI 路由 OK；spec 细节漂移 |
| API-10 | 缺少 `ETag` | state export | **FIXED** — `If-None-Match` → 304 |

---

## 三、REST 设计审查

### URL 结构（示例）

```
/api/v1/auth/login          POST
/api/v1/session             GET
/api/v1/firewall/rules      GET/POST/PATCH/DELETE
/api/v1/firewall/aliases    CRUD
/api/v1/nat/forwards        CRUD
/api/v1/nat/ipv4/...        NAT 映射
/api/v1/shaper/...          QoS
/api/v1/vpn/ocserv/...      VPN
/api/v1/diagnostics/terminal WS
```

**优点**: 资源分组清晰（firewall/nat/shaper/vpn/system）。  
**问题**: NAT IPv4/IPv6 路径不对称；部分 legacy 路径保留在 store 迁移层。

### 返回格式

**现状**:
```json
{ "error": "unauthorized" }
{ "rules": [...], "nft_lines": { "id": "..." } }
```

**问题**: 无 `code` 字段；前端需解析字符串；无 `request_id` 便于排障。

### 状态码使用

| 场景 | 当前 | 推荐 |
|------|------|------|
| 未登录 | 401 | 401 + WWW-Authenticate 说明 |
| 校验失败 | 400 | 400 + 字段级 errors |
| 资源冲突（别名占用） | 409 | 保持 |
| nft 语法错误 | 400 | 422 Unprocessable Entity |
| 服务器错误 | 500 | 500 + 不泄露内部路径 |

### 参数校验

**已具备**: 防火墙端口 1–65535、IPv6 CIDR、别名成员、filter rule alias 引用检查。  
**缺口**: NAT 映射 bulk 导入无 schema；部分 handler 仍 trust JSON 缺省值。

### 分页

**不适用**于当前单文件 state 模型；若引入审计日志或 flow 统计，需 `cursor` 分页。

### 版本管理

- 已有 `/api/v1` 前缀 ✓
- 缺少 changelog 与 breaking change 政策
- 建议：破坏性变更升 v2，v1 保留至少一个 major 周期

### RBAC

- **现状**: 单一管理员 + API Key（全权限）
- **缺口**: 无只读运维角色、无 per-module 权限
- **VPN 运营商需求**: 多租户 shaper 在数据面有模型，API 层仍单管理员

---

## 四、认证与安全

| 项 | 评估 |
|----|------|
| Session | Cookie `qosnat_sess`，30 天 TTL，文件持久化 |
| API Key | bcrypt 哈希，支持 legacy 哈希升级 ✓ |
| CSRF | 依赖 SameSite（需部署 TLS 与 cookie 属性确认） |
| Rate limit | login 有限流（`loginLim`） |
| Terminal | 同权认证 — 见架构 P0-2 |

---

## 五、推荐 API 设计方案

### 5.1 统一响应 Envelope

```json
{
  "ok": true,
  "data": { },
  "error": null,
  "meta": {
    "request_id": "uuid",
    "version": "v1"
  }
}
```

错误时：
```json
{
  "ok": false,
  "error": {
    "code": "FIREWALL_NFT_INVALID",
    "message": "nft ruleset invalid: ...",
    "details": [{ "field": "rules[3].dst_port", "reason": "out of range" }]
  }
}
```

### 5.2 推荐错误码体系

| 前缀 | 域 |
|------|-----|
| `AUTH_*` | 认证会话 |
| `VALID_*` | 输入校验 |
| `FIREWALL_*` | 防火墙/nft |
| `NAT_*` | NAT/NAT64 |
| `QOS_*` | shaper/tc |
| `VPN_*` | ocserv/wg |
| `SYSTEM_*` | 升级/证书/网络 |

HTTP 状态与 code 分离：客户端逻辑用 `error.code`，展示用 `error.message`。

### 5.3 推荐认证体系

1. **短期**: 保持 session + API Key；生产强制 HTTPS；禁用 terminal 或独立 role
2. **中期**: API Key  scoped（read-only / firewall / nat / admin）
3. **长期**: OIDC 集成（运营商 SSO）；可选 MFA for destructive ops

### 5.4 写操作标准流程（Apply Pipeline）

```
POST /api/v1/firewall/rules?dry_run=1   → CheckRuleset only
POST /api/v1/firewall/rules             → validate → save → apply → on fail revert
```

响应含：
```json
{
  "applied": true,
  "nft_reloaded": true,
  "warnings": ["warp reconciled"]
}
```

---

## 六、OpenAPI 与文档

| 项 | 状态 |
|----|------|
| `api/openapi.yaml` | 已扩展 FilterRule、FirewallRulesListResponse（含 nft_lines） |
| `docs/API-ZH.md` | 已含防火墙与端口转发章节 |
| `docs/UI-API-ALIGNMENT.md` | 有 UI-API 对齐说明 |

**建议**: 每个 handler 注册时注释 operationId，CI 生成 diff。

---

## 七、结论

API 设计**满足单管理员网关场景**，REST 分组合理，近期 OpenAPI 与校验增强方向正确。优先统一 **写路径事务语义**、**错误 envelope** 与 **Terminal/高危操作保护**；分页与 RBAC 可按运营商产品化路线图迭代。
