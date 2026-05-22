# P4：租户 / VXLAN / ASN

## 租户 QoS

- API：`GET/POST/PUT/DELETE /api/v1/shaper/tenants`
- 每个租户包含多个 CIDR、统一 `down`/`up`，保存后展开为带 `tenant_id` 的 profile 并刷新 mirred/HTB/BPF
- Web：**Traffic → 租户 QoS**

## VXLAN

- `state.json` → `network.vxlan_tunnels[]`
- netplan 写入 `tunnels:`（`mode: vxlan`, `id`, `local`, `remote`, `port`）
- API：`/api/v1/network/vxlan`
- Web：**Network → VXLAN**

## ASN Alias

- 防火墙别名 `type: asn`，字段 `asn` + `members[]`（前缀列表，需自行维护或外部导入）
- nft 仍生成 `ipv4_addr` 类型 set，与 `ipv4_addr` 别名用法相同
- Web：**Security → Aliases** 选择类型 `asn`

## 范围外

- 无 live BGP/Whois 拉取 ASN 前缀
- 无 EVPN/多 POP 控制器（单宿主机 overlay）
