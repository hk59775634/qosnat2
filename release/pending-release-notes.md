# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复公网 IP 环回（NAT Reflection）：内网可经公网 IP 访问网关本机服务及端口转发目标。

## 新增

- 自动生成 hairpin input 规则（`auto-input-hairpin-*`）：内网访问公网 IP 上的管理口、VPN 与端口转发端口
- 端口转发回流时自动生成 LAN→LAN forward 放行（`auto-fwd-hairpin-*`）

## 优化

- 端口转发目标为本机地址时跳过无意义的回流 DNAT/SNAT，改由 input 链直接放行

## 修复

- 内网经公网 IP 访问端口转发内网主机时被 forward 默认丢弃导致不通

## 删除

- （无）

## 其他

- API 文档与端口转发页说明补充 hairpin 行为
