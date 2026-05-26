# 安全加固前备份说明（tag: `p11-pre-security-fix`）

**Git 提交**：`3e95cc1`（与 tag `p10-ocserv-rate-ui` 同代码）  
**日期**：2026-05-26  
**用途**：在 2026-05 安全/可靠性加固与引导流程改造**之前**的快照，便于回滚或对比。

## 该版本已知问题（加固项摘要）

| 类别 | 说明 |
|------|------|
| 引导 | `POST /api/v1/setup/complete` 无需登录；默认 `admin`/`password` |
| WAN/LAN | 保存角色后 `reloadEnv` 可能仍读进程内旧 `DEV_LAN`/`DEV_WAN`（已在后续提交修复） |
| nft | 策略路由/防火墙等字符串未严格校验即写入规则 |
| 路由 | `PUT /api/v1/routes/{id}` 先删旧路由再 apply，失败时 state/内核不一致 |
| 流量 | ocserv `usertraffic` 查询与采样存在并发读写 |
| vhost | `PUT` 不存在的 domain 会在内存中 append 后返回 404 |
| 密钥 | API Key 仅 SHA-256；WireGuard 私钥 GET 明文返回 |

## 已具备能力（此 tag 仍保留）

- ocserv vhost 继承全局、独立密码用户、限速 UI 方向修正（`p10-ocserv-rate-ui`）
- OpenAPI 中文说明草案（工作区未并入本 tag）

## 回滚到此版本

```bash
git checkout p11-pre-security-fix
# 或
git checkout 3e95cc1
```

重新编译部署后需 `systemctl restart qosnatd`。

## 后续 tag

加固与「先登录再引导、安装随机口令」见 `p12-security-hardening`（main 在加固合并后）。
