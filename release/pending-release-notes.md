# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

补全 ocserv IPv4/IPv6 双栈：会话级限速共用一条管道，Apply 自动转发与 IPv6 masquerade。

## 新增

- ocserv 全局 / 组 / 虚拟主机支持 `ipv6_network` 与 `ipv6_subnet_prefix`（默认每客户端 /128）
- Apply 启用 IPv6 转发；nft 为 ocserv IPv6 池生成 WAN masquerade（NPTv6 已覆盖同前缀则跳过）
- 推送 `default` 且已配 IPv6 时自动补 `route = ::/0`
- Web：服务器/组/vhost IPv6 配置、会话列表 VPN IPv6 列；文档与 OpenAPI 同步

## 优化

- （无）

## 修复

- （无）

## 删除

- （无）

## 其他

- 双栈统一限速使用 ocserv 会话 `rx/tx-data-per-sec`，不依赖 ASAv 或 EDT per-IP
