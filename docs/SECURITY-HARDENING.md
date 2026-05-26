# 安全加固说明（tag: `p12-security-hardening`）

相对备份 tag **`p11-pre-security-fix`**（`3e95cc1`）的变更摘要。

## 引导与鉴权

- `POST /api/v1/setup/*`（除 health/login 外）需 Session 或 API Key
- 移除默认弱口令 `admin`/`password`；安装脚本生成随机 20 位口令并显示/写入 `initial-admin.txt`
- Web：须先登录，再进入 `/setup` 引导；可选在最后一步修改管理员密码

## 可靠性

- **WAN/LAN 角色**：`WriteDevRoles` 后从 env 文件同步进程环境（修复保存无效）
- **路由 PUT**：先删旧内核路由再 apply，失败回滚
- **ocserv vhost PUT**：不存在域名不再内存 append
- **ocserv 删组**：同步失败返回 500
- **usertraffic**：查询时持锁或拷贝桶，避免数据竞争

## 输入校验

- 策略路由 / 引导 policy_routes：`store.ValidateCIDR`
- 静态 NAT inner/outer：`store.ValidateIPv4OrCIDR`
- sysctl PUT：仅允许 catalog 中的键

## 其他

- ocserv 安装状态文件权限 `0600`
- 详见 `docs/PRE-SECURITY-FIX.md` 中未纳入本版的项（API Key KDF、WireGuard 脱敏等）

## 升级步骤

```bash
git pull && git checkout p12-security-hardening   # 或 main
./deploy-qos-nat.sh -BuildWeb start               # 或仅替换 qosnatd 并 restart
systemctl restart qosnatd
```

已安装机器：若 `env` 中无 `ADMIN_PASS` 且未完成引导，需重新运行安装脚本或手动设置 `ADMIN_PASS` 后再登录。
