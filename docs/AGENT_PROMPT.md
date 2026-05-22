# qosnat2 Agent 提示（维护版）

> **适用场景**：在**已存在的** `/opt/qosnat2` 仓库上修 bug、加功能、写验收脚本或改文档。  
> **不适用**：从零新建项目（P0–P6 已完成，见 [`待开发清单.md`](待开发清单.md)）。

## 必读（按顺序）

1. [`待开发清单.md`](待开发清单.md) — **当前任务与验收**（§14 未勾项优先）
2. [`单机双网卡-QoS-NAT-开发说明.md`](单机双网卡-QoS-NAT-开发说明.md) — 架构准则（勿改回 netns/policer SHOT）
3. [`README.md`](../README.md) — 构建、部署、菜单
4. [`API-AUTH.md`](API-AUTH.md) — 登录、API Key、冒烟环境变量

参考对照：`reference/`（旧 qosnat，**禁止**照搬部署）

## 架构红线（不得破坏）

| 项 | 要求 |
|----|------|
| 拓扑 | 宿主机 `DEV_LAN` + `DEV_WAN`；**禁止** netns、flowtable、WAN 移入 netns |
| QoS 执行面 | HTB + fq_codel/cake + IFB(mirred)；**禁止** `TC_ACT_SHOT` policer |
| 控制面 | Go `qosnatd` + cilium/ebpf 管理 Map；API 改速率必须写 Map **并** 同步 HTB |
| mark | nft 与 QoS 隔离；IFB 用 mirred，不用 mark 分流 |
| 网卡名 | 来自 env / 引导，禁止写死 `ens18`/`ens19` |

## 仓库结构（已存在，勿重复造轮子）

```
cmd/qosnatd/
internal/{api,ebpf,shaper,nft,store,netif,route,dnsmasq,...}
bpf/classify.bpf.c
api/openapi.yaml
web/src/          # Vue3，改 API 时同步 client.js + openapi
scripts/acceptance-*.sh
deploy-qos-nat.sh
```

## 开发阶段（历史，均已完成）

P0–P6 见开发说明 §13。产品路线图 **P1–P4** 已关闭；未排期项见待开发清单「pfSense 类扩展」。

## 编码约束

- Go 1.22+；小步 diff，匹配现有包风格
- 改 REST 时同步 `api/openapi.yaml` 与 `web/src/api/client.js`
- 前端勿在 HTML 属性里用未转义 JSON 拼 `onclick`
- **仅在我明确要求时** `git commit` / 打 tag

## 验收（改数据面或 API 后建议跑）

```bash
go test ./...
go build -o /usr/local/bin/qosnatd ./cmd/qosnatd
cd web && npm run build

set -a; source /etc/qosnat2/env; set +a
export QOSNAT_PASS="${ADMIN_PASS:-password}"
/opt/qosnat2/scripts/test-ui-api.sh
/opt/qosnat2/scripts/acceptance-auto.sh
# 有测试机时：
/opt/qosnat2/scripts/acceptance-p2-iperf.sh
```

## 当前已知待办（文档级，非架构缺失）

- §14：`10.0.0.0/8` iperf、多 WAN failover、24h 长稳（见 `P3-stability.md`）
- 远期：Unbound / HAProxy / Captive Portal（清单已标）
- 产品范围外：IPsec、OpenVPN、BGP/SD-WAN（勿擅自扩展除非用户明确要求）

## 回复用户时

- 用中文说明「改了什么 / 为什么 / 如何验证」
- 引用代码用 `startLine:endLine:filepath` 格式
- 不要声称已完成 §14 未勾项，除非已跑验收脚本并更新文档
