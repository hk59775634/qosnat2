# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

出站 IPv4 NAT 去掉 WAN catch-all masquerade，仅对策略网段等显式目标做 SNAT，避免三层公网源被改写出口 IP。

## 新增

- （无）

## 优化

- 出站 NAT：postrouting 不再对全部 WAN 出站流量 masquerade；仅策略网段、1:1/网段映射、出站策略与 hairpin 生效
- Web 文案：明确空策略网段时未列入流量走三层直通

## 修复

- 修复三层路由场景下公网源 IP 被 catch-all SNAT 导致出口 IP 不一致的问题

## 删除

- WAN 口 catch-all `oifname … masquerade` 规则

## 其他

- VPN/内网池若需 NAT，请加入「出站 NAT → 策略路由网段」；未列入则保持源地址转发
