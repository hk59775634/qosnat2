# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

dnsmasq 源码打 chnroutes 补丁，支持国内外 DNS 分流；Web UI 配置并从 hk59775634/chnroutes 自动更新路由表。

## 新增

- 从源码编译带 [dnsmasq-chnroute-path](https://github.com/hk59775634/dnsmasq-chnroute-path) 补丁的 dnsmasq（`scripts/build-dnsmasq-chnroutes.sh`，deploy 时自动执行）。
- DHCP/DNS 页启用 chnroutes 分流：国内 DNS（`server=,1`）/ 国外 DNS（`server=,0`）+ `chnroutes-file`。
- `POST /api/v1/dhcp/chnroutes/update` 下载/更新 `/etc/qosnat2/chnroutes.txt`（默认源 [hk59775634/chnroutes](https://github.com/hk59775634/chnroutes)，jsDelivr + GitHub Raw 双镜像）。

## 优化

- patched dnsmasq 编译选项对齐 Ubuntu（DBus/DNSSEC 等），兼容 systemd-helper 启动。
- chnroutes 路径限制在 `/etc/qosnat2/` 下；启用分流时要求 `dns_enabled` 且至少配置一组 trusted/untrusted DNS。

## 修复

- 修复 patched dnsmasq 仅含 HAVE_LOOP 时 systemd `checkconfig` 失败、服务无法启动的问题。

## 删除

- （无）

## 其他

- （无）
