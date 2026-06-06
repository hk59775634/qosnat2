# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

DHCP 静态租约支持按 MAC 单独指定网关与 DNS。

## 新增

- 静态租约可为单个 MAC 覆盖默认网关（DHCP option 3）与 DNS（option 6）；留空时仍使用全局 DHCP 配置

## 优化

- （无）

## 修复

- （无）

## 删除

- （无）

## 其他

- OpenAPI 补充静态租约 router、dns_servers 字段；dnsmasq 通过 tag 下发 per-host 选项
