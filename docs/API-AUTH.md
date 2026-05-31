# API 鉴权与冒烟测试

## 安全策略

- **初始账号**：`deploy-qos-nat.sh start` 生成随机 `ADMIN_PASS`（20 位），写入 `/etc/qosnat2/env` 与 `initial-admin.txt`；无默认弱口令
- **引导前**：须先 `POST /api/v1/login`，再访问需鉴权的 `/api/v1/setup/*` 与 Web 引导页
- **引导完成后**：`env` 中明文 `ADMIN_PASS` 已清除；仅 `state.json` 中 bcrypt 口令有效
- **API Key**：仅存 `key_hash`；创建时明文仅返回一次
- **监听**：默认 `0.0.0.0:ADMIN_PORT`（HTTP）；启用 TLS 后为 HTTPS

## Session Cookie（浏览器）

1. `POST /api/v1/login` body `{"user":"admin","pass":"password"}`（或已设置的管理员）
2. 响应 `Set-Cookie: qosnat_sess=...`
3. 后续请求带 `credentials: include`（Web UI 已默认）

## API Key（脚本 / CI）

1. 登录 Web → **系统 → API 密钥** → 创建密钥（仅显示一次）
2. 或已有 `state.json` 中 `api_keys` 条目

```bash
export QOSNAT_API_KEY='qk_xxxxxxxx'
curl -s -H "X-API-Key: $QOSNAT_API_KEY" http://<host>:8080/api/v1/shaper/profiles
```

与 Cookie **二选一**；未鉴权返回 `401`。

## API Key 角色

| role | 写权限 |
|------|--------|
| `admin`（默认） | 全部 API |
| `firewall` | 仅写 `/api/v1/firewall/*`；可读全部 GET |
| `readonly` | 禁止写操作（403 `AUTH_READ_ONLY`） |

Prometheus 采集与告警示例见 [PROMETHEUS-METRICS.md](./PROMETHEUS-METRICS.md)。

## 环境变量（自动验收 / 冒烟）

| 变量 | 用途 |
|------|------|
| `ADMIN_USER` / `ADMIN_PASS` | 初始登录（默认 `admin` / `password`）；引导后写入 state 的账号 |
| `QOSNAT_API_KEY` | 跳过登录，所有请求带 `X-API-Key` |
| `QOSNAT_NFT_INCREMENTAL` | `1` 时防火墙单条增删尝试 nft 增量 apply（失败回退全表 reload） |

```bash
set -a; source /etc/qosnat2/env; set +a
export QOSNAT_PASS="${ADMIN_PASS:-password}"
/opt/qosnat2/scripts/test-ui-api.sh
```

## HTTPS

启用后 API 基址改为 `https://<host>:8080`。自签证书：

```bash
curl -sk -H "X-API-Key: $QOSNAT_API_KEY" https://<host>:8080/api/v1/health
```
