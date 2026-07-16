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

## p12 第二批（`p13-security-hardening`）

- API Key：**bcrypt** 存储；遗留 SHA-256 首次校验通过后自动升级
- WireGuard GET：**不返回**服务端/Peer 私钥；`server_private_key_set` / `private_key_set` 标志；PUT 留空合并原密钥
- 防火墙规则：网卡名、CIDR、协议、别名名校验
- 前缀 NAT mapping：CIDR 校验
- HTTP 响应：**安全头**（nosniff、DENY frame、HSTS 在 TLS 下）
- `health`：未完成引导时不返回 `dev_lan`/`dev_wan`/`admin_port`
- systemd：`PrivateTmp` 等轻量加固（仍 root 运行）

## 升级步骤

```bash
git pull && git checkout p12-security-hardening   # 或 main
./deploy-qos-nat.sh -BuildWeb start               # 或仅替换 qosnatd 并 restart
systemctl restart qosnatd
```

## 生产网关：限制 unattended-upgrades

默认部署（`deploy-qos-nat.sh start`）会执行 **lockdown** 模式：

- 禁止 `apt-daily` / `apt-daily-upgrade` 定时器
- 禁止自动 `update` / `upgrade` / `unattended-upgrade`
- 写入包黑名单（systemd、netplan、frr、内核、iproute2、nftables 等），防止误开自动升级时动到数据面

维护窗口内手动升级：

```bash
apt update
apt upgrade   # 或按需指定包
```

单独配置（已安装机器）：

```bash
sudo ./scripts/configure-gateway-apt.sh lockdown        # 推荐
sudo ./scripts/configure-gateway-apt.sh security-only   # 仅安全更新 + 黑名单（仍有风险）
sudo ./scripts/configure-gateway-apt.sh off           # 恢复 Ubuntu 默认 apt 定时器
```

环境变量：`QOSNAT_GATEWAY_APT=off ./deploy-qos-nat.sh start` 可跳过。

配置文件：

- `/etc/apt/apt.conf.d/20qosnat2-gateway.conf`
- `/etc/apt/apt.conf.d/51qosnat2-gateway-unattended.conf`

已安装机器：若 `env` 中无 `ADMIN_PASS` 且未完成引导，需重新运行安装脚本或手动设置 `ADMIN_PASS` 后再登录。
