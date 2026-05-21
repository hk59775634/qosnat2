# 网卡与 VLAN：netplan 方案

qosnat2 **不再**对托管网卡/VLAN 使用 `ip addr` / `ip link add vlan`，统一写入：

| 路径 | 说明 |
|------|------|
| `/etc/netplan/99-qosnat2.yaml` | 由 `qosnatd` 根据 `state.json` → `network.ifaces` / `network.vlans` 生成 |
| `state.json` | 持久化期望配置 |

应用流程：`netplan generate` → `netplan apply`；若条目 `up: false`，apply 后对对应接口执行 `ip link set down`。

## 与 cloud-init 的关系

- 常见还有 `/etc/netplan/50-cloud-init.yaml`（安装时生成）。
- netplan 会**合并**多个文件；`99-qosnat2.yaml` 中同名 `ethernets.<dev>` 会**覆盖** cloud-init 中该口的地址定义。
- 建议：LAN/WAN 基线可留在 cloud-init；**仅**在 Web/API 修改过的口与 VLAN 写入 `99-qosnat2.yaml`。

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| PUT | `/api/v1/interfaces` | 更新物理口 → `network.ifaces` + netplan apply |
| GET/POST/DELETE | `/api/v1/network/vlans` | VLAN CRUD + netplan apply |
| GET | `/api/v1/network/netplan` | 预览将写入的 YAML |
| POST | `/api/v1/network/netplan/apply` | 按 state 重新 apply |

## 不托管的接口

`lo`、`ifb*`、`veth*`、`docker*`、`br-*` 不可通过接口 API 配置。

## 依赖

```bash
apt install netplan.io
```
