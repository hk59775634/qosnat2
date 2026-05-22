# API 鉴权与冒烟测试

## Session Cookie（浏览器）

1. `POST /api/v1/login` body `{"user":"admin","pass":"..."}`
2. 响应 `Set-Cookie: qosnat_sess=...`
3. 后续请求带 `credentials: include`（Web UI 已默认）

## API Key（脚本 / CI）

1. 登录 Web → **系统 → API 密钥** → 创建密钥（仅显示一次）
2. 或已有 `state.json` 中 `api_keys` 条目

```bash
export QOSNAT_API_KEY='qosnat_xxxxxxxx'
curl -s -H "X-API-Key: $QOSNAT_API_KEY" http://127.0.0.1:8080/api/v1/shaper/profiles
```

与 Cookie **二选一**；未鉴权返回 `401`。

## 环境变量（自动验收 / 冒烟）

| 变量 | 用途 |
|------|------|
| `ADMIN_PASS` 或 `QOSNAT_PASS` | `acceptance-auto.sh` / `test-ui-api.sh` 登录 |
| `QOSNAT_API_KEY` | 跳过登录，所有请求带 `X-API-Key` |

```bash
# 读取部署 env 后跑完整验收
sudo bash -c 'set -a; source /etc/qosnat2/env; set +a; export QOSNAT_PASS="$ADMIN_PASS"; /opt/qosnat2/scripts/acceptance-auto.sh'

# 仅 API 冒烟（需已 setup_complete）
set -a; source /etc/qosnat2/env; set +a
export QOSNAT_PASS="$ADMIN_PASS"   # 或 export QOSNAT_API_KEY=...
/opt/qosnat2/scripts/test-ui-api.sh
```

## HTTPS

启用后 API 基址改为 `https://<host>:8080`（默认端口不变）。自签证书：

```bash
curl -sk -H "X-API-Key: $QOSNAT_API_KEY" https://127.0.0.1:8080/api/v1/health
```

验收脚本：`scripts/acceptance-https.sh`（需 root、已 setup_complete、已知管理员密码）。
