# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

新增每内网源 IP 的 NAT 出站 conntrack 会话上限，自动覆盖 LAN、QoS 网段、ocserv/WG 隧道地址。

## 新增

- 防火墙页可配置每 IP 最大出站会话数（nft ct count，0=关闭）
- 自动聚合监控网段：QoS 策略、NAT 路由、DHCP LAN、ocserv 池、WireGuard peer 等

## 优化

- （无）

## 修复

- （无）

## 删除

- （无）

## 其他

- API：`PUT /api/v1/firewall/session-limit`
